package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"slices"
	"time"

	"github.com/elbaldfun/ghta/internal/judge"
	"github.com/elbaldfun/ghta/internal/taxonomy"
)

// judgeResult is one golden item's outcome under the merged judge.
type judgeResult struct {
	ID         string          `json:"id"`
	Golden     []string        `json:"golden"`
	GoldenType string          `json:"goldenType"`
	Votes      []judge.Verdict `json:"votes"`
	Final      judge.Verdict   `json:"final"`
	Confidence string          `json:"confidence"`
}

// runJudge scores the merged judge (internal/judge) the way the worker runs
// it: every item asked twice in different batches, a third time when the two
// answers disagree. It needs JUDGE_BASE_URL and JUDGE_MODEL.
func runJudge(ctx context.Context, tree taxonomy.Tree, golden []goldenItem, metas map[string]itemMeta, batchSize int, outPath string, quick bool) {
	base, model := os.Getenv("JUDGE_BASE_URL"), os.Getenv("JUDGE_MODEL")
	if base == "" || model == "" {
		must(fmt.Errorf("JUDGE_BASE_URL and JUDGE_MODEL are required"), "judge")
	}
	j := judge.New(judge.NewClient(base, model, os.Getenv("JUDGE_API_KEY")), tree)

	inputs := map[string]judge.Input{}
	var ids []string
	// The golden file lists a few repositories twice; each is judged once.
	seen := map[string]bool{}
	golden = slices.DeleteFunc(slices.Clone(golden), func(g goldenItem) bool {
		dup := seen[g.ID]
		seen[g.ID] = true
		return dup
	})
	for _, g := range golden {
		if m, ok := metas[g.ID]; ok {
			inputs[g.ID] = judge.Input{ID: g.ID, Name: m.Name, Description: m.Description, Language: m.Language, Topics: m.Topics}
			ids = append(ids, g.ID)
		}
	}
	votes := map[string][]judge.Verdict{}
	rng := rand.New(rand.NewSource(1))
	start := time.Now()
	calls := 0
	// pass asks about ids in batches; shuffling changes which items share one.
	pass := func(name string, ids []string, full bool) {
		ids = slices.Clone(ids)
		if !full {
			rng.Shuffle(len(ids), func(a, b int) { ids[a], ids[b] = ids[b], ids[a] })
		}
		for s := 0; s < len(ids); s += batchSize {
			var batch []judge.Input
			for _, id := range ids[s:min(s+batchSize, len(ids))] {
				batch = append(batch, inputs[id])
			}
			got, err := j.Ask(ctx, batch, full)
			calls++
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s batch %d failed: %v\n", name, s, err)
				continue
			}
			for id, v := range got {
				votes[id] = append(votes[id], v)
			}
			fmt.Fprintf(os.Stderr, "%s %d/%d  %s\n", name, min(s+batchSize, len(ids)), len(ids), time.Since(start).Round(time.Second))
		}
	}
	if quick {
		// One short answer each: enough to compare prompts on domain, form and shelf.
		pass("quick", ids, false)
		ids = nil
	}
	pass("first", ids, true)
	// Only items with a full answer go on: the prose comes from it.
	var answered []string
	for _, id := range ids { // empty under -judge-quick
		if len(votes[id]) == 1 {
			answered = append(answered, id)
		}
	}
	pass("second", answered, false)
	var third []string
	for _, id := range answered {
		if _, more := judge.Decide(votes[id]); more {
			third = append(third, id)
		}
	}
	pass("third", third, false)

	var results []judgeResult
	for _, g := range golden {
		if len(votes[g.ID]) == 0 {
			continue
		}
		d, _ := judge.Decide(votes[g.ID])
		if d.Confidence == "" {
			d.Confidence = "single" // a later pass failed for this item
		}
		results = append(results, judgeResult{ID: g.ID, Golden: g.Domain, GoldenType: g.Type,
			Votes: votes[g.ID], Final: d.Verdict, Confidence: d.Confidence})
	}

	fmt.Printf("\n══ merged judge: %d items, %d calls, %s (%.1fs per item) ══\n",
		len(results), calls, time.Since(start).Round(time.Second), time.Since(start).Seconds()/float64(max(len(results), 1)))
	// A golden label under apps/ is a shelf label; the others are domain labels.
	// Domain accuracy and shelf accuracy are scored apart, each on the golden
	// items that carry that kind of label.
	score := func(label string, pick func(judgeResult) (judge.Verdict, bool)) {
		var n, withDomain, top1, hit, tp, pred, gold, withShelf, shelfOK, typeOK int
		for _, r := range results {
			v, ok := pick(r)
			if !ok {
				continue
			}
			n++
			if v.Type == r.GoldenType {
				typeOK++
			}
			var goldDomain, goldShelf []string
			for _, p := range r.Golden {
				if taxonomy.IsShelfPath(p) {
					goldShelf = append(goldShelf, taxonomy.ShelfSlug(p))
				} else {
					goldDomain = append(goldDomain, p)
				}
			}
			if len(goldShelf) > 0 {
				withShelf++
				if slices.Contains(goldShelf, v.Shelf) {
					shelfOK++
				}
			}
			if len(goldDomain) == 0 {
				continue
			}
			withDomain++
			if slices.Contains(goldDomain, v.Primary()) {
				top1++
			}
			common := 0
			for _, p := range v.Paths {
				if slices.Contains(goldDomain, p) {
					common++
				}
			}
			if common > 0 {
				hit++
			}
			tp, pred, gold = tp+common, pred+len(v.Paths), gold+len(goldDomain)
		}
		fmt.Printf("%-20s n=%-4d domain(%d): top1 %.3f hit-any %.3f P %.3f R %.3f  shelf(%d): %.3f  type %.3f\n",
			label, n, withDomain, safeDiv(top1, withDomain), safeDiv(hit, withDomain), safeDiv(tp, pred), safeDiv(tp, gold),
			withShelf, safeDiv(shelfOK, withShelf), safeDiv(typeOK, n))
	}
	score("first answer alone", func(r judgeResult) (judge.Verdict, bool) { return r.Votes[0], true })
	score("after voting", func(r judgeResult) (judge.Verdict, bool) { return r.Final, true })
	for _, c := range []string{judge.Agreed, judge.Majority, judge.Split, "single"} {
		score("  "+c, func(r judgeResult) (judge.Verdict, bool) { return r.Final, r.Confidence == c })
	}

	if outPath != "" {
		raw, _ := json.MarshalIndent(results, "", " ")
		must(os.WriteFile(outPath, raw, 0o644), "write results")
	}
}

// loadMetaCache reads item metadata saved by an earlier run (-meta).
func loadMetaCache(path string) map[string]itemMeta {
	metas := map[string]itemMeta{}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	must(json.Unmarshal(raw, &metas), "parse "+path)
	return metas
}
