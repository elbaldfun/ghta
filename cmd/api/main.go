// Command api is the GHTA backend entrypoint: it loads config, connects to
// MongoDB, provisions schema/indexes, registers source adapters, schedules the
// fetch/categorize jobs, and serves the REST API with graceful shutdown.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"

	"github.com/elbaldfun/ghta/internal/config"
	"github.com/elbaldfun/ghta/internal/domain"
	"github.com/elbaldfun/ghta/internal/handler"
	"github.com/elbaldfun/ghta/internal/job"
	"github.com/elbaldfun/ghta/internal/provider"
	"github.com/elbaldfun/ghta/internal/repository"
	"github.com/elbaldfun/ghta/internal/service"
	"github.com/elbaldfun/ghta/internal/source"
	"github.com/elbaldfun/ghta/internal/source/github"
	"github.com/elbaldfun/ghta/internal/taxonomy"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("configuration error", "err", err)
		os.Exit(1)
	}

	logger := newLogger(cfg.LogLevel)
	slog.SetDefault(logger)

	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	store, err := repository.Connect(rootCtx, cfg)
	if err != nil {
		slog.Error("mongodb connection failed", "err", err)
		os.Exit(1)
	}
	if err := store.EnsureSchema(rootCtx); err != nil {
		slog.Error("schema provisioning failed", "err", err)
		os.Exit(1)
	}
	slog.Info("connected to mongodb", "db", cfg.MongoDB)

	// Source registry + adapters.
	registry := source.NewRegistry()
	registry.Register(github.NewAdapter(cfg.GitHubToken, cfg.RateLimitBuffer, 50, logger))

	fetcher := job.NewFetcher(store, registry, logger)

	// Trend metrics (growth over the snapshot history).
	metrics := service.NewMetricsService(store, logger)

	// Taxonomy: the git-controlled category tree is the single source of truth.
	taxNodes, err := taxonomy.Load("taxonomy/taxonomy.yaml")
	if err != nil {
		slog.Error("taxonomy load failed", "err", err)
		os.Exit(1)
	}
	if err := taxonomy.Sync(rootCtx, store, taxNodes); err != nil {
		slog.Error("taxonomy sync failed", "err", err)
		os.Exit(1)
	}
	rules, err := taxonomy.LoadRules("taxonomy/topic-map.yaml")
	if err != nil {
		slog.Error("topic map load failed", "err", err)
		os.Exit(1)
	}
	facets, err := taxonomy.LoadFacets("taxonomy/facets.yaml")
	if err != nil {
		slog.Error("facets load failed", "err", err)
		os.Exit(1)
	}
	slog.Info("taxonomy synced", "categories", len(taxNodes), "topicRules", len(rules.Topics), "typeFacets", len(facets.Type))

	// AI categorization pipeline: type facet + domain (rules -> LLM).
	aiService := service.NewAIService(store, provider.New(cfg, logger))
	categorizer := job.NewCategorizer(store, rules, facets, aiService, cfg.CategorizeBatchSize, cfg.DomainMaxLabels, logger)

	// Scheduled jobs. Metrics run right after each fetch pass.
	scheduler := cron.New(cron.WithSeconds())
	if _, err := scheduler.AddFunc(cfg.FetchCron, func() {
		fetcher.Run(rootCtx)
		if err := metrics.Run(rootCtx); err != nil {
			slog.Error("metrics computation failed", "err", err)
		}
	}); err != nil {
		slog.Error("invalid FETCH_CRON", "err", err)
		os.Exit(1)
	}
	if _, err := scheduler.AddFunc(cfg.CategorizeCron, func() { categorizer.Run(rootCtx) }); err != nil {
		slog.Error("invalid CATEGORIZE_CRON", "err", err)
		os.Exit(1)
	}
	scheduler.Start()
	defer scheduler.Stop()

	starHistory := service.NewStarHistoryService(store, logger)

	facetOrder := make([]service.TypeFacet, len(facets.Type))
	for i, f := range facets.Type {
		facetOrder[i] = service.TypeFacet{Key: f.Key, Name: f.Name}
	}
	facetOrder = append(facetOrder, service.TypeFacet{Key: facets.Fallback, Name: facets.FallbackName})

	router := newRouter(store, fetcher, categorizer, metrics, starHistory, rootCtx, cfg.AdminToken, facetOrder)

	srv := &http.Server{
		Addr:              ":" + strconv.Itoa(cfg.Port),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		slog.Info("api listening", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http server error", "err", err)
			stop()
		}
	}()

	<-rootCtx.Done()
	slog.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "err", err)
	}
	if err := store.Close(shutdownCtx); err != nil {
		slog.Error("mongo disconnect failed", "err", err)
	}
	slog.Info("stopped")
}

func newRouter(store *repository.Store, fetcher *job.Fetcher, categorizer *job.Categorizer, metrics *service.MetricsService, starHistory *service.StarHistoryService, jobCtx context.Context, adminToken string, facetOrder []service.TypeFacet) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	// ---- Public: health plus the read-only API the website consumes ----

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	trending := handler.NewTrendingHandler(service.NewTrendService(store, starHistory))
	r.GET("/trending", trending.List)
	r.GET("/trending/rising", trending.Rising)
	r.GET("/trending/item", trending.Item)

	handler.NewStatsHandler(store).Register(r)

	categoryHandler := handler.NewCategoryHandler(service.NewCategoryService(store), facetOrder)
	categoryHandler.RegisterPublic(r) // read-only tree + facets for navigation

	// ---- Admin: bearer-token guarded ----
	// These mutate data, expose user records, or start quota-burning jobs, so
	// they must not be anonymously reachable on the public internet.
	admin := r.Group("", handler.RequireAdminToken(adminToken))

	categoryHandler.RegisterAdmin(admin)
	handler.NewUserHandler(service.NewUserService(store)).Register(admin)

	// Internal: manually trigger a fetch. With ?source=&shard= it runs one shard
	// synchronously (handy for smoke tests); otherwise a full pass in the background.
	admin.POST("/internal/fetch", func(c *gin.Context) {
		src := c.Query("source")
		shard := c.Query("shard")
		if src != "" && shard != "" {
			if err := fetcher.RunShard(c.Request.Context(), domain.Source(src), shard); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"status": "shard fetched", "source": src, "shard": shard})
			return
		}
		go fetcher.Run(jobCtx)
		c.JSON(http.StatusAccepted, gin.H{"status": "fetch started"})
	})

	// Internal: manually trigger a categorization pass (background).
	admin.POST("/internal/categorize", func(c *gin.Context) {
		go categorizer.Run(jobCtx)
		c.JSON(http.StatusAccepted, gin.H{"status": "categorize started"})
	})

	// Internal (change 12 migration): reset done/failed items back to pending so
	// the categorizer re-classifies them on the new tree. ?limit=N stages the
	// rollout (default 500; 0 = all). Operator loops reset -> categorize until
	// drained. Reset only touches analysisStatus/failCount; categoryPath/type
	// stay until re-categorized, so the site keeps serving during migration.
	admin.POST("/internal/reset-analysis", func(c *gin.Context) {
		limit := 500
		if v := c.Query("limit"); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be >= 0"})
				return
			}
			limit = n
		}
		n, err := service.ResetAnalysis(c.Request.Context(), store, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "reset", "count": n})
	})

	// Internal: warm up the star-history cache for the top-N repos by stars,
	// in the background. Already-backfilled repos are skipped; remote queries
	// are paced to stay polite to the public datasets.
	admin.POST("/internal/backfill-history", func(c *gin.Context) {
		top := 1000
		if v := c.Query("top"); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 || n > 10000 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "top must be 1..10000"})
				return
			}
			top = n
		}
		go starHistory.Warmup(jobCtx, top)
		c.JSON(http.StatusAccepted, gin.H{"status": "backfill started", "top": top})
	})

	// Internal: recompute growth metrics (synchronous, so callers can read fresh data).
	admin.POST("/internal/metrics", func(c *gin.Context) {
		if err := metrics.Run(c.Request.Context()); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "metrics computed"})
	})

	return r
}

func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
}
