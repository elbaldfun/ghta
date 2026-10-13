package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	"github.com/elbaldfun/ghta/internal/item"
	"github.com/elbaldfun/ghta/internal/judge"
	"github.com/elbaldfun/ghta/internal/store"
	"github.com/elbaldfun/ghta/internal/worker"
)

const (
	judgeBatch = 10
	// A chunk is voted on and written as a unit, so an interrupted run loses
	// at most one. It is also the pool the second pass draws batch-mates from.
	judgeChunk       = 100
	judgeReadmeRunes = 300
	judgeMaxFail     = 3
	// This many calls failing in a row means the model is gone, not that the
	// items are hard.
	judgeMaxCallFailures = 3
)

// Asker is the model behind the judge job; *judge.Judge implements it.
type Asker interface {
	Ask(ctx context.Context, batch []judge.Input, full bool) (map[string]judge.Verdict, error)
}

// JudgeConfig bounds one run of the judge job.
type JudgeConfig struct {
	TreeVersion int
	MaxPerRun   int
	MinStars    float64
	HotWeekly   float64
}

// Judge labels items with the model: domain, form, shelf, taglines,
// alternatives and tags in one call (docs/REBUILD.md M3).
//
// Every item is asked twice, with different batch-mates, and a third time
// when the two answers differ. Only a decided item is written; anything cut
// short by a failed call stays pending for the next run.
type Judge struct {
	st    *store.Store
	log   *slog.Logger
	model Asker
	cfg   JudgeConfig
	rng   *rand.Rand
	now   func() time.Time
}

func NewJudge(st *store.Store, model Asker, cfg JudgeConfig, log *slog.Logger) *Judge {
	if log == nil {
		log = slog.Default()
	}
	return &Judge{st: st, log: log, model: model, cfg: cfg, rng: rand.New(rand.NewSource(time.Now().UnixNano())), now: time.Now}
}

func (*Judge) Name() string { return "judge" }

// judgeTally counts what a run did, for the job report.
type judgeTally struct {
	calls, callFailures, streak int
	agreed, majority, split     int
}

func (j *Judge) Run(ctx context.Context) (rep worker.Report, err error) {
	queue, err := j.st.JudgeQueue(ctx, store.JudgeScope{
		TreeVersion: j.cfg.TreeVersion, PromptVersion: judge.PromptVersion,
		MinStars: j.cfg.MinStars, HotWeekly: j.cfg.HotWeekly,
	}, j.cfg.MaxPerRun)
	if err != nil {
		return rep, err
	}
	var t judgeTally
	defer func() {
		rep.Note("queued", len(queue))
		rep.Note("calls", t.calls)
		rep.Note("callFailures", t.callFailures)
		rep.Note("agreed", t.agreed)
		rep.Note("majority", t.majority)
		rep.Note("split", t.split)
	}()
	for start := 0; start < len(queue); start += judgeChunk {
		chunk := queue[start:min(start+judgeChunk, len(queue))]
		if err := j.chunk(ctx, chunk, &t, &rep); err != nil {
			return rep, err
		}
		j.log.Info("judge: chunk done", "items", min(start+judgeChunk, len(queue)), "of", len(queue),
			"agreed", t.agreed, "majority", t.majority, "split", t.split, "failed", rep.Failed)
	}
	return rep, nil
}

// chunk takes one slice of the queue through the votes and writes the result.
func (j *Judge) chunk(ctx context.Context, chunk []store.JudgeCandidate, t *judgeTally, rep *worker.Report) error {
	ids := make([]string, len(chunk))
	byID := make(map[string]store.JudgeCandidate, len(chunk))
	for i, c := range chunk {
		ids[i] = c.ExternalID
		byID[c.ExternalID] = c
	}
	readmes, err := j.st.Readmes(ctx, item.SourceGitHub, ids)
	if err != nil {
		return err
	}
	inputs := make(map[string]judge.Input, len(chunk))
	for _, c := range chunk {
		inputs[c.ExternalID] = judge.Input{
			ID: c.ExternalID, Name: c.Name, Description: c.Description, Language: c.Language,
			Topics: c.Topics, Platforms: c.Gh.Platforms, Readme: judge.Excerpt(readmes[c.ExternalID], judgeReadmeRunes),
		}
	}

	votes := map[string][]judge.Verdict{}
	unfit := map[string]bool{} // asked, and the answer was unusable
	// pass asks about ids in batches of judgeBatch and adds each answer to the votes.
	pass := func(ids []string, full bool) error {
		for s := 0; s < len(ids); s += judgeBatch {
			if err := ctx.Err(); err != nil {
				return err
			}
			part := ids[s:min(s+judgeBatch, len(ids))]
			batch := make([]judge.Input, len(part))
			for i, id := range part {
				batch[i] = inputs[id]
			}
			got, err := j.model.Ask(ctx, batch, full)
			t.calls++
			if err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				t.callFailures++
				if t.streak++; t.streak >= judgeMaxCallFailures {
					return fmt.Errorf("judge: %d model calls failed in a row, last: %w", t.streak, err)
				}
				j.log.Warn("judge: model call failed, batch left pending", "err", err)
				continue
			}
			t.streak = 0
			for _, id := range part {
				if v, ok := got[id]; ok {
					votes[id] = append(votes[id], v)
				} else {
					unfit[id] = true
				}
			}
		}
		return nil
	}
	// again lists the items that have a first answer and still need one more.
	again := func() []string {
		var out []string
		for _, id := range ids {
			if len(votes[id]) == 0 || unfit[id] {
				continue
			}
			if _, more := judge.Decide(votes[id]); more {
				out = append(out, id)
			}
		}
		j.rng.Shuffle(len(out), func(a, b int) { out[a], out[b] = out[b], out[a] })
		return out
	}

	if err := pass(ids, true); err != nil {
		return err
	}
	// The second answer for everyone, then a third where the two differ.
	for round := 1; round < judge.MaxVotes; round++ {
		if err := pass(again(), false); err != nil {
			return err
		}
	}

	at := j.now()
	for _, id := range ids {
		c := byID[id]
		if unfit[id] {
			if err := j.st.JudgeFailed(ctx, c.ID, judgeMaxFail, at); err != nil {
				return err
			}
			rep.Failed++
			continue
		}
		if len(votes[id]) == 0 {
			continue
		}
		d, more := judge.Decide(votes[id])
		if more {
			continue // a pass failed for this item: pending, asked afresh next run
		}
		v := d.Verdict
		if c.Labels.ShelfOverride != "" {
			v.Shelf = c.Labels.ShelfOverride // a person's shelf outranks the model's
		}
		hasGui := v.HasGui
		labels := item.Labels{
			Domain: v.Domain(), Type: v.Type, Shelf: v.Shelf, HasGui: &hasGui,
			TaglineZh: v.TaglineZh, TaglineEn: v.TaglineEn, Alternatives: v.Alternatives, Tags: v.Tags,
			Confidence: d.Confidence, Version: j.cfg.TreeVersion,
		}
		if err := j.st.SetVerdict(ctx, c.ID, labels, judge.PromptVersion, at); err != nil {
			return err
		}
		rep.Processed++
		switch d.Confidence {
		case judge.Agreed:
			t.agreed++
		case judge.Majority:
			t.majority++
		default:
			t.split++
		}
	}
	return nil
}
