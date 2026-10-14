package pipeline

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/elbaldfun/ghta/internal/boards"
	"github.com/elbaldfun/ghta/internal/item"
	"github.com/elbaldfun/ghta/internal/store"
	"github.com/elbaldfun/ghta/internal/taxonomy"
	"github.com/elbaldfun/ghta/internal/worker"
)

// Boards materialises every ranking the api used to aggregate on request
// (internal/boards) and records the head of each ranked board for the day.
//
// Like rank, it reads the corpus once through a narrow projection and works
// in memory; a board is replaced whole, so a reader sees yesterday's or
// today's, never half of each.
type Boards struct {
	st     *store.Store
	tree   taxonomy.Tree
	facets []boards.Facet
	now    func() time.Time
}

// NewBoards takes the domain tree, for names and counts, and the forms in the
// order the filter chips show them.
func NewBoards(st *store.Store, tree taxonomy.Tree, facets []boards.Facet) *Boards {
	return &Boards{st: st, tree: tree, facets: facets, now: time.Now}
}

func (*Boards) Name() string { return "boards" }

func (j *Boards) Run(ctx context.Context) (worker.Report, error) {
	var rep worker.Report
	now := j.now().UTC().Truncate(time.Millisecond)
	day := item.DayOf(now)

	var corpus boards.Corpus
	cur, err := j.st.Items().Find(ctx, bson.M{"source": item.SourceGitHub, "stale": bson.M{"$ne": true}},
		options.Find().SetBatchSize(2000).SetProjection(boards.Projection()))
	if err != nil {
		return rep, fmt.Errorf("read items: %w", err)
	}
	for cur.Next(ctx) {
		var r boards.Row
		if err := cur.Decode(&r); err != nil {
			rep.Failed++
			continue
		}
		corpus.Add(r)
	}
	err = cur.Err()
	_ = cur.Close(ctx)
	if err != nil {
		return rep, fmt.Errorf("read items: %w", err)
	}
	profiles, err := j.profiles(ctx)
	if err != nil {
		return rep, err
	}

	written := 0
	put := func(key string, rows any) error {
		if err := j.st.PutBoard(ctx, key, rows, now); err != nil {
			return err
		}
		written++
		return nil
	}
	// head records the leading ids of a ranked board for the day.
	head := func(key string, n int, id func(int) string) error {
		ids := make([]string, min(n, boards.HistoryHead))
		for i := range ids {
			ids[i] = id(i)
		}
		return j.st.PutBoardHead(ctx, key, day, ids, now)
	}

	var tops []string
	for _, n := range j.tree.Nodes {
		tops = append(tops, n.Path)
	}
	for _, g := range boards.Groups(tops) {
		rising, merit := corpus.Developers(g, profiles)
		for board, rows := range map[string][]boards.Developer{boards.DevRising: rising, boards.DevMerit: merit} {
			key := boards.DevelopersKey(board, g.Key)
			if err := put(key, rows); err != nil {
				return rep, err
			}
			if err := head(key, len(rows), func(i int) string { return rows[i].Login }); err != nil {
				return rep, err
			}
		}
		hot, popular := corpus.Topics(g)
		for sort, rows := range map[string][]boards.Topic{boards.TopicHot: hot, boards.TopicPopular: popular} {
			if err := put(boards.TopicsKey(sort, g.Key), rows); err != nil {
				return rep, err
			}
		}
	}
	for _, p := range boards.Pillars {
		hot, popular := corpus.Ecosystem(p)
		for sort, rows := range map[string][]boards.EcoItem{boards.EcoHot: hot, boards.EcoPopular: popular} {
			key := boards.EcosystemKey(p, sort)
			if err := put(key, rows); err != nil {
				return rep, err
			}
			if err := head(key, len(rows), func(i int) string { return rows[i].ExternalID }); err != nil {
				return rep, err
			}
		}
	}
	for key, rows := range map[string]any{
		boards.HeatKey:       corpus.Heat(j.tree),
		boards.CategoriesKey: corpus.Categories(j.tree),
		boards.TypeFacetsKey: corpus.TypeFacets(j.facets),
		boards.LanguagesKey:  corpus.Languages(),
		boards.StalenessKey:  corpus.Staleness(now),
	} {
		if err := put(key, rows); err != nil {
			return rep, err
		}
	}

	rep.Processed = corpus.Len()
	rep.Note("boards", written)
	return rep, nil
}

// profiles loads the developer profiles the developer boards join on.
func (j *Boards) profiles(ctx context.Context) (map[string]boards.Profile, error) {
	cur, err := j.st.Developers().Find(ctx, bson.M{}, options.Find().SetProjection(bson.M{
		"login": 1, "type": 1, "name": 1, "twitterUsername": 1, "followers": 1, "company": 1, "avatarUrl": 1, "bio": 1,
	}))
	if err != nil {
		return nil, fmt.Errorf("read developers: %w", err)
	}
	var rows []boards.Profile
	if err := cur.All(ctx, &rows); err != nil {
		return nil, fmt.Errorf("read developers: %w", err)
	}
	out := make(map[string]boards.Profile, len(rows))
	for _, p := range rows {
		out[p.Login] = p
	}
	return out, nil
}
