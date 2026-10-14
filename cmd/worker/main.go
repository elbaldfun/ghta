// Command worker runs the scheduled jobs of the rebuilt system. It is the only
// process that writes the rebuilt collections; the api only reads them, so the
// worker can be restarted at any time without affecting the site.
//
//	worker                  run the nightly pipeline on PIPELINE_CRON
//	worker -run fetch,rank  run the named jobs once, in pipeline order, and exit
//	worker -list            print the pipeline and exit
//
// The judge job joins the pipeline when JUDGE_BASE_URL and JUDGE_MODEL name an
// OpenAI-compatible model server; see internal/judge.
//
// With -run the exit status is 1 when a job fails. See docs/REDESIGN.md §5.3.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/robfig/cron/v3"

	"github.com/elbaldfun/ghta/internal/boards"
	"github.com/elbaldfun/ghta/internal/judge"
	"github.com/elbaldfun/ghta/internal/pipeline"
	"github.com/elbaldfun/ghta/internal/source/github"
	"github.com/elbaldfun/ghta/internal/source/huggingface"
	"github.com/elbaldfun/ghta/internal/store"
	"github.com/elbaldfun/ghta/internal/taxonomy"
	"github.com/elbaldfun/ghta/internal/worker"
)

func main() {
	var (
		runList = flag.String("run", "", "comma-separated jobs to run once, then exit")
		list    = flag.Bool("list", false, "print the pipeline and exit")
	)
	flag.Parse()
	_ = godotenv.Load() // .env is optional; the real environment wins

	level := slog.LevelInfo
	_ = level.UnmarshalText([]byte(getEnv("LOG_LEVEL", "info")))
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(log)

	if err := run(log, *runList, *list); err != nil {
		log.Error("worker failed", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger, runList string, list bool) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if list {
		jobs, err := nightly(nil, log)
		if err != nil {
			return err
		}
		fmt.Println(strings.Join(names(jobs), "\n"))
		return nil
	}
	uri, db := os.Getenv("MONGODB_URI"), os.Getenv("MONGODB_DB")
	if uri == "" || db == "" {
		return fmt.Errorf("MONGODB_URI and MONGODB_DB are required")
	}
	// Only fetch needs the token; a one-off run of other jobs does not.
	if os.Getenv("GITHUB_API_TOKEN") == "" && (runList == "" || wants(runList, "fetch", "reconcile", "devsync")) {
		return fmt.Errorf("GITHUB_API_TOKEN is required by fetch, reconcile and devsync")
	}
	st, err := store.Connect(ctx, uri, db)
	if err != nil {
		return err
	}
	defer st.Close(context.Background())
	host, _ := os.Hostname()
	st.Worker = getEnv("WORKER_NAME", host)

	jobs, err := nightly(st, log)
	if err != nil {
		return err
	}
	if err := st.EnsureIndexes(ctx); err != nil {
		return err
	}
	if n, err := st.CloseInterruptedRuns(ctx, time.Now()); err != nil {
		return err
	} else if n > 0 {
		log.Warn("worker: closed runs left over by a previous process", "runs", n)
	}
	runner := worker.NewRunner(st, log)

	if runList != "" {
		chosen, err := pick(jobs, runList)
		if err != nil {
			return err
		}
		for _, r := range runner.Run(ctx, chosen...) {
			if r.Err != nil {
				return fmt.Errorf("job %s: %w", r.Job, r.Err)
			}
		}
		return ctx.Err()
	}

	spec := getEnv("PIPELINE_CRON", "0 30 3 * * *")
	// SkipIfStillRunning: a pipeline that overruns a day is not queued twice.
	sched := cron.New(cron.WithSeconds(), cron.WithLocation(time.UTC),
		cron.WithChain(cron.SkipIfStillRunning(cron.DiscardLogger)))
	if _, err := sched.AddFunc(spec, func() { runner.Run(ctx, jobs...) }); err != nil {
		return fmt.Errorf("PIPELINE_CRON %q: %w", spec, err)
	}
	sched.Start()
	log.Info("worker started", "worker", st.Worker, "db", db, "pipelineCron", spec, "jobs", names(jobs))

	<-ctx.Done()
	log.Info("worker stopping: waiting for the running job")
	<-sched.Stop().Done()
	return nil
}

// nightly is the pipeline, in the order the jobs run. Later jobs read what
// earlier ones wrote, so the order is part of the design.
//
// The judge is in it only where a model is configured (JUDGE_BASE_URL): the
// model lives on a private network the production host cannot reach, so the
// judge runs from a machine that can, against the same database.
func nightly(st *store.Store, log *slog.Logger) ([]worker.Job, error) {
	buffer := envInt("RATE_LIMIT_BUFFER", 200)
	gh := github.NewAdapter(os.Getenv("GITHUB_API_TOKEN"), buffer, 50, log)
	dir := getEnv("TAXONOMY_DIR", "taxonomy")
	tree, err := taxonomy.LoadTree(filepath.Join(dir, "taxonomy.yaml"))
	if err != nil {
		return nil, err
	}
	steps, err := taxonomy.LoadMigrations(filepath.Join(dir, "migrations.yaml"), tree)
	if err != nil {
		return nil, err
	}
	facets, err := taxonomy.LoadFacets(filepath.Join(dir, "facets.yaml"))
	if err != nil {
		return nil, err
	}
	forms := make([]boards.Facet, 0, len(facets.Type)+1)
	for _, f := range facets.Type {
		forms = append(forms, boards.Facet{Key: f.Key, Name: f.Name})
	}
	forms = append(forms, boards.Facet{Key: facets.Fallback, Name: facets.FallbackName})
	jobs := []worker.Job{
		pipeline.NewFetch(st, log, gh, huggingface.NewAdapter(os.Getenv("HF_TOKEN"), log)),
		// Before rank: a repository found gone or renamed must leave the rankings.
		pipeline.NewReconcile(st, gh, buffer, log),
		// Before the judge: it picks up what a tree change sends back to it.
		pipeline.NewMigrate(st, tree, steps, log),
	}
	if base := os.Getenv("JUDGE_BASE_URL"); base != "" {
		model := os.Getenv("JUDGE_MODEL")
		if model == "" {
			return nil, fmt.Errorf("JUDGE_MODEL is required when JUDGE_BASE_URL is set")
		}
		// Before rank: domain rankings follow the labels.
		jobs = append(jobs, pipeline.NewJudge(st, judge.New(judge.NewClient(base, model, os.Getenv("JUDGE_API_KEY")), tree),
			pipeline.JudgeConfig{
				TreeVersion: tree.Version,
				MaxPerRun:   envInt("JUDGE_MAX_PER_RUN", 3000),
				MinStars:    float64(envInt("JUDGE_MIN_STARS", 1000)),
				HotWeekly:   float64(envInt("JUDGE_HOT_WEEKLY", 50)),
			}, log))
	}
	return append(jobs,
		pipeline.NewRank(st),
		// After rank and the judge: boards read growth and labels as of tonight.
		pipeline.NewBoards(st, tree, forms),
		pipeline.NewIcon(st, log),
		pipeline.NewShot(st, log),
		// Last: a full sweep can sit out GitHub's hourly window more than once.
		pipeline.NewDevSync(st, gh, buffer, log),
	), nil
}

func envInt(key string, fallback int) int {
	if n, err := strconv.Atoi(os.Getenv(key)); err == nil {
		return n
	}
	return fallback
}

// pick returns the named jobs in pipeline order, whatever order they were
// asked for in.
func pick(jobs []worker.Job, csv string) ([]worker.Job, error) {
	want := map[string]bool{}
	for _, n := range strings.Split(csv, ",") {
		if n = strings.TrimSpace(n); n != "" {
			want[n] = true
		}
	}
	var out []worker.Job
	for _, j := range jobs {
		if want[j.Name()] {
			out = append(out, j)
			delete(want, j.Name())
		}
	}
	for n := range want {
		return nil, fmt.Errorf("unknown job %q; known: %s", n, strings.Join(names(jobs), ", "))
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("-run names no job")
	}
	return out, nil
}

// wants reports whether a -run list names any of jobs.
func wants(csv string, jobs ...string) bool {
	for _, n := range strings.Split(csv, ",") {
		if slices.Contains(jobs, strings.TrimSpace(n)) {
			return true
		}
	}
	return false
}

func names(jobs []worker.Job) []string {
	out := make([]string, len(jobs))
	for i, j := range jobs {
		out[i] = j.Name()
	}
	return out
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
