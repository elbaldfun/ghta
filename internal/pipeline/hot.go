package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/elbaldfun/ghta/internal/boards"
	"github.com/elbaldfun/ghta/internal/item"
	"github.com/elbaldfun/ghta/internal/source/github"
	"github.com/elbaldfun/ghta/internal/store"
	"github.com/elbaldfun/ghta/internal/worker"
)

// Trending scrapes one of GitHub's trending lists.
type Trending func(ctx context.Context, window string) ([]github.TrendingRepo, error)

// RepoSearcher fetches repositories by search query, in the fetch's shape.
type RepoSearcher interface {
	SearchItems(ctx context.Context, query string, limit int) ([]item.Fetched, error)
}

// Hot stores GitHub's trending lists as boards, each repo joined to its
// labels. A listed repo we do not track yet is fetched and written like any
// fetched repo, so the judge and the boards take it from there; trending
// is where the fetch's star cutoff would otherwise miss a breakout.
//
// A window that fails to scrape keeps the board it had.
type Hot struct {
	st     *store.Store
	scrape Trending
	gh     RepoSearcher
	log    *slog.Logger
	now    func() time.Time
}

func NewHot(st *store.Store, scrape Trending, gh RepoSearcher, log *slog.Logger) *Hot {
	if log == nil {
		log = slog.Default()
	}
	return &Hot{st: st, scrape: scrape, gh: gh, log: log, now: time.Now}
}

func (*Hot) Name() string { return "hot" }

func (j *Hot) Run(ctx context.Context) (worker.Report, error) {
	var rep worker.Report
	lists := map[string][]github.TrendingRepo{}
	var ids []string
	for _, w := range boards.HotWindows {
		repos, err := j.scrape(ctx, w)
		if err != nil {
			if ctx.Err() != nil {
				return rep, ctx.Err()
			}
			j.log.Warn("hot: scrape failed", "window", w, "err", err)
			rep.Failed++
			continue
		}
		lists[w] = repos
		for _, r := range repos {
			ids = append(ids, r.ExternalID)
		}
	}
	if len(lists) == 0 {
		return rep, fmt.Errorf("hot: every window failed to scrape")
	}

	known, err := j.labels(ctx, ids)
	if err != nil {
		return rep, err
	}
	added := 0
	for _, id := range ids {
		if _, ok := known[id]; ok {
			continue
		}
		found, err := j.gh.SearchItems(ctx, "repo:"+id, 1)
		if err != nil || len(found) == 0 || !strings.EqualFold(found[0].Item.ExternalID, id) {
			if ctx.Err() != nil {
				return rep, ctx.Err()
			}
			continue // renamed, private or gone since GitHub listed it
		}
		if _, err := j.st.WriteFetched(ctx, found, j.now()); err != nil {
			return rep, err
		}
		known[id] = hotLabels{}
		added++
	}

	now := j.now().UTC()
	for w, repos := range lists {
		rows := make([]boards.HotRepo, len(repos))
		for i, r := range repos {
			l, tracked := known[r.ExternalID]
			rows[i] = boards.HotRepo{
				FullName: r.ExternalID, Owner: r.Owner, Name: r.Name, Description: r.Description, Language: r.Language,
				Stars: r.Stars, Forks: r.Forks, StarsGained: r.StarsThisPeriod, Tracked: tracked,
				Domain: l.Labels.Domain, Type: l.Labels.Type, TaglineZh: l.Labels.TaglineZh, TaglineEn: l.Labels.TaglineEn,
				IconURL: l.Assets.IconURL,
			}
		}
		if err := j.st.PutBoard(ctx, boards.HotKey(w), rows, now); err != nil {
			return rep, err
		}
		rep.Processed += len(rows)
	}
	rep.Note("added", added)
	return rep, nil
}

type hotLabels struct {
	Labels struct {
		Domain    []string `bson:"domain"`
		Type      string   `bson:"type"`
		TaglineZh string   `bson:"taglineZh"`
		TaglineEn string   `bson:"taglineEn"`
	} `bson:"labels"`
	Assets struct {
		IconURL string `bson:"iconUrl"`
	} `bson:"assets"`
}

// labels returns what the boards show of each tracked repo among ids.
func (j *Hot) labels(ctx context.Context, ids []string) (map[string]hotLabels, error) {
	cur, err := j.st.Items().Find(ctx, bson.M{"source": item.SourceGitHub, "externalId": bson.M{"$in": ids}},
		options.Find().SetProjection(bson.M{"externalId": 1, "labels.domain": 1, "labels.type": 1,
			"labels.taglineZh": 1, "labels.taglineEn": 1, "assets.iconUrl": 1}))
	if err != nil {
		return nil, fmt.Errorf("hot labels: %w", err)
	}
	var rows []struct {
		ExternalID string    `bson:"externalId"`
		Labels     hotLabels `bson:",inline"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, fmt.Errorf("hot labels: %w", err)
	}
	out := make(map[string]hotLabels, len(rows))
	for _, r := range rows {
		out[r.ExternalID] = r.Labels
	}
	return out, nil
}
