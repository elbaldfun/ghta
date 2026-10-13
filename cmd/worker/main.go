// Command worker runs the scheduled jobs of the rebuilt system. It is the only
// process that writes the rebuilt collections; the api only reads them, so the
// worker can be restarted at any time without affecting the site.
//
//	worker                  run the nightly pipeline on PIPELINE_CRON
//	worker -run fetch,rank  run the named jobs once, in pipeline order, and exit
//	worker -list            print the pipeline and exit
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
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/robfig/cron/v3"

	"github.com/elbaldfun/ghta/internal/pipeline"
	"github.com/elbaldfun/ghta/internal/source/github"
	"github.com/elbaldfun/ghta/internal/source/huggingface"
	"github.com/elbaldfun/ghta/internal/store"
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
		fmt.Println(strings.Join(names(nightly(nil, log)), "\n"))
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

	jobs := nightly(st, log)
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
	log.Info("worker started", "db", db, "pipelineCron", spec, "jobs", names(jobs))

	<-ctx.Done()
	log.Info("worker stopping: waiting for the running job")
	<-sched.Stop().Done()
	return nil
}

// nightly is the pipeline, in the order the jobs run. Later jobs read what
// earlier ones wrote, so the order is part of the design.
func nightly(st *store.Store, log *slog.Logger) []worker.Job {
	buffer, err := strconv.Atoi(getEnv("RATE_LIMIT_BUFFER", "200"))
	if err != nil {
		buffer = 200
	}
	gh := github.NewAdapter(os.Getenv("GITHUB_API_TOKEN"), buffer, 50, log)
	return []worker.Job{
		pipeline.NewFetch(st, log, gh, huggingface.NewAdapter(os.Getenv("HF_TOKEN"), log)),
		// Before rank: a repository found gone or renamed must leave the rankings.
		pipeline.NewReconcile(st, gh, buffer, log),
		pipeline.NewRank(st),
		pipeline.NewIcon(st, log),
		pipeline.NewShot(st, log),
		// Last: a full sweep can sit out GitHub's hourly window more than once.
		pipeline.NewDevSync(st, gh, buffer, log),
	}
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
