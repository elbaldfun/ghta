// Command api serves the read-only /v1 interface of the rebuilt store
// (docs/REBUILD.md) with graceful shutdown. Every scheduled job runs in
// cmd/worker; this process only reads, apart from filling star_history for a
// detail page that has none yet.
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

	"github.com/elbaldfun/ghta/internal/api"
	"github.com/elbaldfun/ghta/internal/config"
	"github.com/elbaldfun/ghta/internal/domain"
	"github.com/elbaldfun/ghta/internal/repository"
	"github.com/elbaldfun/ghta/internal/service"
	"github.com/elbaldfun/ghta/internal/store"
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

	// The connection only; the worker provisions the indexes.
	conn, err := repository.Connect(rootCtx, cfg)
	if err != nil {
		slog.Error("mongodb connection failed", "err", err)
		os.Exit(1)
	}
	slog.Info("connected to mongodb", "db", cfg.MongoDB)

	// A detail page's long star curve: backfilled from GitHub's stargazer
	// history on the first view of a repository that has none stored.
	starHistory := service.NewStarHistoryService(conn, logger)
	v1 := api.New(store.New(conn.DB), logger).WithBackfill(func(ctx context.Context, id string) []api.Point {
		points, _ := starHistory.Ensure(ctx, domain.SourceGitHub, id)
		out := make([]api.Point, len(points))
		for i, p := range points {
			out[i] = api.Point{T: p.T.UnixMilli(), V: int(p.V)}
		}
		return out
	})

	srv := &http.Server{
		Addr:              ":" + strconv.Itoa(cfg.Port),
		Handler:           newRouter(v1, cfg.AdminToken),
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
	if err := conn.Close(shutdownCtx); err != nil {
		slog.Error("mongo disconnect failed", "err", err)
	}
	slog.Info("stopped")
}

func newRouter(v1 *api.API, adminToken string) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	// Public: /health and the read-only /v1 interface the website consumes.
	v1.RegisterHealth(r)
	v1.Register(r)

	// Admin, bearer-token guarded: GET /internal/jobs, the worker's latest runs.
	v1.RegisterInternal(r.Group("", api.RequireAdminToken(adminToken)))

	return r
}

func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
}
