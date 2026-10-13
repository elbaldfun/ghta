// Command import rebuilds the new collections (items, item_blobs) from the old
// store and verifies the result. It only reads the old collections, drops and
// refills the new ones, and can be run as often as needed.
//
//	go run ./cmd/import                 import, then verify a 2,000-item sample
//	go run ./cmd/import -sample 0       verify every item (the cutover gate)
//	go run ./cmd/import -verify-only    verify an existing import
//
// Exit status is 1 when a check fails. See docs/REBUILD.md §4.4 and §6.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/elbaldfun/ghta/internal/config"
	"github.com/elbaldfun/ghta/internal/importer"
	"github.com/elbaldfun/ghta/internal/repository"
	"github.com/elbaldfun/ghta/internal/service"
	"github.com/elbaldfun/ghta/internal/taxonomy"
)

func main() {
	var (
		verifyOnly = flag.Bool("verify-only", false, "skip the import, verify what is there")
		sample     = flag.Int("sample", 2000, "items to compare field by field; 0 = all")
		batch      = flag.Int("batch", 500, "items per write batch")
		treePath   = flag.String("taxonomy", "taxonomy/taxonomy.yaml", "taxonomy tree")
	)
	flag.Parse()
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	if err := run(log, *verifyOnly, *sample, *batch, *treePath); err != nil {
		log.Error("import failed", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger, verifyOnly bool, sample, batch int, treePath string) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Not config.Load: that insists on the fetcher's and the LLM's settings,
	// none of which an import needs.
	cfg := &config.Config{MongoURI: os.Getenv("MONGODB_URI"), MongoDB: os.Getenv("MONGODB_DB")}
	if cfg.MongoURI == "" || cfg.MongoDB == "" {
		return fmt.Errorf("MONGODB_URI and MONGODB_DB are required")
	}
	tree, err := taxonomy.LoadTree(treePath)
	if err != nil {
		return err
	}
	leaves := map[string]bool{}
	for _, n := range tree.Leaves() {
		leaves[n.Path] = true
	}
	rules := importer.Rules{
		ValidPath:   func(p string) bool { return leaves[p] },
		ShelfPath:   service.ShelfDomainPath,
		BaseVersion: tree.Version - 1,
	}

	old, err := repository.Connect(ctx, cfg)
	if err != nil {
		return err
	}
	defer old.Close(context.Background())

	out := map[string]any{"database": cfg.MongoDB, "taxonomyVersion": tree.Version}
	if !verifyOnly {
		rep, err := importer.Run(ctx, old.DB, rules, importer.Options{Batch: batch, Log: log})
		out["import"] = rep
		if err != nil {
			emit(out)
			return err
		}
	}
	checks, err := importer.Verify(ctx, old.DB, rules, sample)
	if err != nil {
		emit(out)
		return err
	}
	out["checks"], out["passed"] = checks, importer.Passed(checks)
	emit(out)
	if !importer.Passed(checks) {
		return fmt.Errorf("verification failed")
	}
	return nil
}

func emit(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}
