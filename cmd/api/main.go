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
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/elbaldfun/ghta/internal/config"
	"github.com/elbaldfun/ghta/internal/domain"
	"github.com/elbaldfun/ghta/internal/handler"
	"github.com/elbaldfun/ghta/internal/job"
	"github.com/elbaldfun/ghta/internal/provider"
	"github.com/elbaldfun/ghta/internal/repository"
	"github.com/elbaldfun/ghta/internal/service"
	"github.com/elbaldfun/ghta/internal/source"
	"github.com/elbaldfun/ghta/internal/source/github"
	"github.com/elbaldfun/ghta/internal/source/huggingface"
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

	// Source registry + adapters. Keep the GitHub adapter handy for the live
	// "newly created" search the API exposes.
	registry := source.NewRegistry()
	ghAdapter := github.NewAdapter(cfg.GitHubToken, cfg.RateLimitBuffer, 50, logger)
	registry.Register(ghAdapter)
	// HuggingFace models (change 14): rides the same daily fetch pass; the
	// shared metrics job then derives likes velocity from its snapshots.
	registry.Register(huggingface.NewAdapter(cfg.HFToken, logger))

	fetcher := job.NewFetcher(store, registry, logger)

	// Trend metrics (growth over the snapshot history).
	metrics := service.NewMetricsService(store, logger)

	// Taxonomy: the git-controlled category tree is the single source of truth.
	tree, err := taxonomy.LoadTree("taxonomy/taxonomy.yaml")
	if err != nil {
		slog.Error("taxonomy load failed", "err", err)
		os.Exit(1)
	}
	taxNodes := tree.Nodes
	if err := taxonomy.Sync(rootCtx, store, taxNodes); err != nil {
		slog.Error("taxonomy sync failed", "err", err)
		os.Exit(1)
	}
	// Tree migrations (one step per version). Validated against the tree at
	// boot so a typo in migrations.yaml fails here, not mid-migration.
	migrations, err := taxonomy.LoadMigrations("taxonomy/migrations.yaml", tree)
	if err != nil {
		slog.Error("taxonomy migrations load failed", "err", err)
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
	slog.Info("taxonomy synced", "version", tree.Version, "categories", len(taxNodes), "topicRules", len(rules.Topics), "typeFacets", len(facets.Type), "migrations", len(migrations))

	// AI categorization pipeline: type facet + domain (rules -> LLM).
	aiProvider := provider.New(cfg, logger)
	aiService := service.NewAIService(store, aiProvider)
	categorizer := job.NewCategorizer(store, rules, facets, aiService, cfg.CategorizeBatchSize, cfg.DomainMaxLabels, cfg.LLMConcurrency,
		tree.Version, job.Gate{MinStars: cfg.CategorizeMinStars, HotWeekly: cfg.CategorizeHotWeekly}, logger)
	migrator := taxonomy.NewMigrator(store, tree, logger)
	taxAudit := job.NewTaxonomyAudit(store, tree, logger)
	devSync := job.NewDevSync(store, ghAdapter, 0, cfg.RateLimitBuffer, logger)
	altFinder := job.NewAltFinder(store, aiProvider, logger)
	storeFinder := job.NewStoreFinder(store, aiProvider, logger)
	shotFetcher := job.NewShotFetcher(store, logger)
	iconFetcher := job.NewIconFetcher(store, logger)
	reconciler := job.NewReconciler(store, ghAdapter, cfg.RateLimitBuffer, logger)

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
	if _, err := scheduler.AddFunc(cfg.TaxonomyAuditCron, func() { _, _ = taxAudit.Run(rootCtx) }); err != nil {
		slog.Error("invalid TAXONOMY_AUDIT_CRON", "err", err)
		os.Exit(1)
	}
	if _, err := scheduler.AddFunc(cfg.DevSyncCron, func() { devSync.Run(rootCtx) }); err != nil {
		slog.Error("invalid DEVSYNC_CRON", "err", err)
		os.Exit(1)
	}
	if _, err := scheduler.AddFunc(cfg.StoreCron, func() { storeFinder.Run(rootCtx) }); err != nil {
		slog.Error("invalid STORE_CRON", "err", err)
		os.Exit(1)
	}
	if _, err := scheduler.AddFunc(cfg.AltFindCron, func() { altFinder.Run(rootCtx) }); err != nil {
		slog.Error("invalid ALTFIND_CRON", "err", err)
		os.Exit(1)
	}
	if _, err := scheduler.AddFunc(cfg.ShotCron, func() { shotFetcher.Run(rootCtx) }); err != nil {
		slog.Error("invalid SHOT_CRON", "err", err)
		os.Exit(1)
	}
	if _, err := scheduler.AddFunc(cfg.IconCron, func() { iconFetcher.Run(rootCtx) }); err != nil {
		slog.Error("invalid ICON_CRON", "err", err)
		os.Exit(1)
	}
	if _, err := scheduler.AddFunc(cfg.ReconcileCron, func() { reconciler.Run(rootCtx) }); err != nil {
		slog.Error("invalid RECONCILE_CRON", "err", err)
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

	router := newRouter(store, fetcher, categorizer, altFinder, storeFinder, shotFetcher, iconFetcher, devSync, metrics, starHistory, ghAdapter, rootCtx, cfg.AdminToken, facetOrder,
		taxDeps{tree: tree, migrations: migrations, migrator: migrator, audit: taxAudit})

	srv := &http.Server{
		Addr:              ":" + strconv.Itoa(cfg.Port),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		// Bound the full request/response lifecycle and idle keep-alives so a
		// slow or stalled client can't pin goroutines/FDs (Slowloris) on the
		// small single-vCPU container. Read covers a dribbled body; Write caps
		// slow readers; Idle reaps parked keep-alive connections.
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  90 * time.Second,
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

// taxDeps bundles the taxonomy-versioning pieces the router exposes.
// migrateRun is the state of the one taxonomy migration this process may be
// running: a guard against a second start, and the result of the last one.
type migrateRun struct {
	mu       sync.Mutex
	running  bool
	from, to int
	started  time.Time
	finished time.Time
	stats    taxonomy.Stats
	err      string
}

func (r *migrateRun) start(from, to int) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.running {
		return false
	}
	// Field by field: assigning a whole migrateRun would overwrite the mutex
	// this method is holding.
	r.running, r.from, r.to = true, from, to
	r.started, r.finished = time.Now().UTC(), time.Time{}
	r.stats, r.err = taxonomy.Stats{}, ""
	return true
}

func (r *migrateRun) finish(stats taxonomy.Stats, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.running, r.finished, r.stats = false, time.Now().UTC(), stats
	if err != nil {
		r.err = err.Error()
	}
}

func (r *migrateRun) snapshot() gin.H {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.started.IsZero() {
		return gin.H{"status": "idle"}
	}
	h := gin.H{"status": "running", "from": r.from, "to": r.to, "started": r.started}
	if !r.running {
		h["status"], h["finished"], h["stats"] = "done", r.finished, r.stats
		if r.err != "" {
			h["status"], h["error"] = "failed", r.err
		}
	}
	return h
}

type taxDeps struct {
	tree       taxonomy.Tree
	migrations []taxonomy.Migration
	migrator   *taxonomy.Migrator
	audit      *job.TaxonomyAudit
}

func newRouter(store *repository.Store, fetcher *job.Fetcher, categorizer *job.Categorizer, altFinder *job.AltFinder, storeFinder *job.StoreFinder, shotFetcher *job.ShotFetcher, iconFetcher *job.IconFetcher, devSync *job.DevSync, metrics *service.MetricsService, starHistory *service.StarHistoryService, ghAdapter *github.Adapter, jobCtx context.Context, adminToken string, facetOrder []service.TypeFacet, tax taxDeps) *gin.Engine {
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
	r.GET("/search/suggest", trending.Suggest)

	handler.NewStatsHandler(store).Register(r)
	handler.NewNewReposHandler(ghAdapter).Register(r)
	handler.NewHotReposHandler(store, ghAdapter).Register(r)                    // GET /trending/hot — scraped weekly trending, ingests newcomers
	handler.NewDeveloperHandler(service.NewDeveloperService(store)).Register(r) // GET /developers — merit/rising ranking
	handler.NewTopicHandler(service.NewTopicService(store)).Register(r)         // GET /topics — hot topics per domain
	handler.NewEcosystemHandler(service.NewEcosystemService(store)).Register(r) // GET /ecosystem — AI stack (skills/mcp/agents)
	handler.NewAppHandler(service.NewAppService(store)).Register(r)             // GET /apps — open-source app directory (filter by OS)
	handler.NewModelHandler(service.NewModelService(store)).Register(r)         // GET /models — HuggingFace model boards

	categoryService := service.NewCategoryService(store)
	categoryHandler := handler.NewCategoryHandler(categoryService, facetOrder)
	categoryHandler.RegisterPublic(r) // read-only tree + facets for navigation
	// Old-path -> new-path for every tree migration so far (rename/merge, and a
	// split's largest successor): the website 301s stale category URLs with it.
	aliases := taxonomy.Aliases(tax.migrations)
	r.GET("/category/aliases", func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=3600")
		c.JSON(http.StatusOK, gin.H{"version": tax.tree.Version, "aliases": aliases})
	})
	handler.NewHeatmapHandler(service.NewHeatmapService(store, categoryService)).Register(r) // GET /category/heat
	handler.NewTrendHandler(store).Register(r)                                               // GET /trend — real per-repo star series

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

	// Internal: manually trigger a categorization pass (background). ?concurrency=N
	// retunes LLM parallelism live (persists for later runs too).
	admin.POST("/internal/categorize", func(c *gin.Context) {
		if v := c.Query("concurrency"); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "concurrency must be a positive number"})
				return
			}
			categorizer.SetConcurrency(n)
		}
		go categorizer.Run(jobCtx)
		c.JSON(http.StatusAccepted, gin.H{"status": "categorize started", "concurrency": categorizer.Concurrency()})
	})

	// Internal: build/refresh the developer layer — fetch each repo owner's
	// GitHub account profile into `developers`. Full sweep, self-pacing to the
	// API budget, resumable (skips accounts fetched within the refresh window).
	admin.POST("/internal/dev-sync", func(c *gin.Context) {
		go devSync.Run(jobCtx)
		c.JSON(http.StatusAccepted, gin.H{"status": "dev-sync started"})
	})

	// Internal: infer "open-source alternative to <paid product>" for the app
	// corpus (LLM), most-popular-first, resumable. Background.
	admin.POST("/internal/alt-find", func(c *gin.Context) {
		go altFinder.Run(jobCtx)
		c.JSON(http.StatusAccepted, gin.H{"status": "alt-find started"})
	})

	// Internal: extract app brand icons from homepages, most-popular-first,
	// resumable. Background.
	admin.POST("/internal/icon-fetch", func(c *gin.Context) {
		go iconFetcher.Run(jobCtx)
		c.JSON(http.StatusAccepted, gin.H{"status": "icon-fetch started"})
	})

	// Internal: app-store judgement (change 15). ?drain=1 keeps running until
	// the whole pending corpus is judged (the backfill); otherwise one bounded
	// pass. Background either way; watch the logs for progress.
	admin.POST("/internal/store-find", func(c *gin.Context) {
		if c.Query("drain") == "1" {
			go storeFinder.RunUntilDone(jobCtx)
			c.JSON(http.StatusAccepted, gin.H{"status": "store drain started"})
			return
		}
		limit := 0
		if v := c.Query("limit"); v != "" {
			limit, _ = strconv.Atoi(v)
		}
		go storeFinder.RunN(jobCtx, limit)
		c.JSON(http.StatusAccepted, gin.H{"status": "store pass started", "limit": limit})
	})

	// Internal: screenshot extraction pass (README first-image + og:image),
	// most-popular-first, resumable. Background; watch logs for the coverage
	// number that gates the UI decision.
	admin.POST("/internal/shot-fetch", func(c *gin.Context) {
		go shotFetcher.Run(jobCtx)
		c.JSON(http.StatusAccepted, gin.H{"status": "shot-fetch started"})
	})

	// Internal: manual shelf correction — the override always wins over the LLM
	// verdict. ?category= must be a known slug or "excluded"; empty clears the
	// override.
	admin.POST("/internal/store-override", func(c *gin.Context) {
		id := c.Query("id")
		category := c.Query("category")
		if id == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "id required (owner/repo)"})
			return
		}
		if category != "" && !service.ValidShelfSlug(category) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unknown category slug"})
			return
		}
		var update bson.M
		if category == "" {
			update = bson.M{"$unset": bson.M{"store.categoryOverride": ""}}
		} else {
			update = bson.M{"$set": bson.M{"store.categoryOverride": category}}
		}
		res, err := store.Items().UpdateOne(c.Request.Context(),
			bson.M{"source": domain.SourceGitHub, "externalId": id}, update)
		if err != nil {
			slog.Error("store-override failed", "id", id, "err", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		if res.MatchedCount == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "item not found"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "id": id, "category": category})
	})

	// Internal: apply the tree migration step that lands on the loaded tree
	// version (taxonomy/migrations.yaml). Mechanical ops run inline and
	// return their counts; anything re-queued drains through the nightly
	// categorizer / storefinder runs (or the /internal/categorize and
	// /internal/store-find triggers). Idempotent: safe to call again.
	migrate := &migrateRun{}
	admin.POST("/internal/taxonomy-migrate", func(c *gin.Context) {
		var step *taxonomy.Migration
		for i := range tax.migrations {
			if tax.migrations[i].To == tax.tree.Version {
				step = &tax.migrations[i]
			}
		}
		if step == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "no migration step targets tree version " + strconv.Itoa(tax.tree.Version)})
			return
		}
		// A migration scans the collection once per op and outlives the
		// server's write timeout, so it runs on the job context and reports
		// through GET /internal/taxonomy-migrate and the log.
		if !migrate.start(step.From, step.To) {
			c.JSON(http.StatusConflict, gin.H{"error": "a migration is already running"})
			return
		}
		m := *step
		go func() {
			stats, err := tax.migrator.Run(jobCtx, m, service.CurrentStoreVersion)
			migrate.finish(stats, err)
			if err != nil {
				slog.Error("taxonomy migrate failed", "err", err, "partial", stats)
				return
			}
			slog.Info("taxonomy migrate done", "from", m.From, "to", m.To, "stats", stats)
		}()
		c.JSON(http.StatusAccepted, gin.H{"status": "started", "from": step.From, "to": step.To})
	})

	// Internal: state and counts of the last migration started on this process.
	admin.GET("/internal/taxonomy-migrate", func(c *gin.Context) {
		c.JSON(http.StatusOK, migrate.snapshot())
	})

	// Internal: undo the last migration for every item that still has its
	// pre-migration snapshot (30-day window), marking them done on ?to=<v>.
	admin.POST("/internal/taxonomy-rollback", func(c *gin.Context) {
		to, err := strconv.Atoi(c.Query("to"))
		if err != nil || to < 1 || to >= tax.tree.Version {
			c.JSON(http.StatusBadRequest, gin.H{"error": "to must be a version below the loaded tree"})
			return
		}
		ids, err := taxonomy.LeafIDs(c.Request.Context(), store)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		n, err := tax.migrator.Rollback(c.Request.Context(), to, ids)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "rolled back", "to": to, "count": n})
	})

	// Internal: drop the pre-migration snapshots once the rollback window has
	// passed. ?days=N (default 30) keeps anything touched more recently.
	admin.POST("/internal/taxonomy-clear-snapshots", func(c *gin.Context) {
		days := 30
		if v := c.Query("days"); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "days must be >= 1"})
				return
			}
			days = n
		}
		n, err := tax.migrator.ClearRollbackSnapshots(c.Request.Context(), time.Now().UTC().AddDate(0, 0, -days))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "cleared", "olderThanDays": days, "count": n})
	})

	// Internal: per-leaf size/heat vs the split/merge thresholds. GET returns
	// the last nightly report; POST recomputes now.
	admin.GET("/internal/taxonomy-audit", func(c *gin.Context) {
		rep := tax.audit.Last()
		if rep == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "no audit yet; POST to compute"})
			return
		}
		c.JSON(http.StatusOK, rep)
	})
	admin.POST("/internal/taxonomy-audit", func(c *gin.Context) {
		rep, err := tax.audit.Run(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, rep)
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
