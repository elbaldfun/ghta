package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/elbaldfun/ghta/internal/domain"
	"github.com/elbaldfun/ghta/internal/item"
)

// CollDevelopers is shared with the old system unchanged (docs/REBUILD.md §4.3).
const CollDevelopers = "developers"

func (s *Store) Developers() *mongo.Collection { return s.DB.Collection(CollDevelopers) }

// AppCorpusFilter matches the user-facing app corpus: an app or cli by type,
// or anything shipping platform builds; never a library, and never an item
// the judge excluded from the store. Items not judged yet pass.
func AppCorpusFilter() bson.M {
	return bson.M{
		"labels.type": bson.M{"$ne": "library"},
		"$or": bson.A{
			bson.M{"labels.type": bson.M{"$in": bson.A{"app", "cli"}}},
			bson.M{"gh.platforms.0": bson.M{"$exists": true}},
		},
		"labels.shelf": bson.M{"$ne": item.ShelfExcluded},
	}
}

// AssetCandidate is an app waiting for an asset job.
type AssetCandidate struct {
	ID         primitive.ObjectID `bson:"_id"`
	Source     string             `bson:"source"`
	ExternalID string             `bson:"externalId"`
	Homepage   string             `bson:"homepage"`
}

// AssetQueue returns apps a job ("icon" or "shot") has not processed yet,
// most popular first. needHomepage leaves out apps without one.
func (s *Store) AssetQueue(ctx context.Context, job string, needHomepage bool, limit int) ([]AssetCandidate, error) {
	filter := AppCorpusFilter()
	filter["jobs."+job+".status"] = bson.M{"$exists": false}
	if needHomepage {
		filter["homepage"] = bson.M{"$gt": ""}
	}
	cur, err := s.Items().Find(ctx, filter, options.Find().
		SetSort(bson.D{{Key: "metrics.stars", Value: -1}}).SetLimit(int64(limit)).
		SetProjection(bson.M{"source": 1, "externalId": 1, "homepage": 1}))
	if err != nil {
		return nil, fmt.Errorf("%s queue: %w", job, err)
	}
	out := []AssetCandidate{}
	if err := cur.All(ctx, &out); err != nil {
		return nil, fmt.Errorf("%s queue: %w", job, err)
	}
	return out, nil
}

// SetIcon records the icon job's verdict on one item; url "" means none found.
func (s *Store) SetIcon(ctx context.Context, id primitive.ObjectID, url string, at time.Time) error {
	set := bson.M{"jobs.icon": item.JobState{Status: item.StatusDone, At: at.UTC()}}
	if url != "" {
		set["assets.iconUrl"] = url
	}
	if _, err := s.Items().UpdateByID(ctx, id, bson.M{"$set": set}); err != nil {
		return fmt.Errorf("set icon: %w", err)
	}
	return nil
}

// SetShot records the screenshot job's verdict; url "" means none found.
// source is "readme" or "og".
func (s *Store) SetShot(ctx context.Context, id primitive.ObjectID, url, source string, at time.Time) error {
	set := bson.M{"jobs.shot": item.JobState{Status: item.StatusDone, At: at.UTC()}}
	if url != "" {
		set["assets.screenshotUrl"], set["assets.shotSource"] = url, source
	}
	if _, err := s.Items().UpdateByID(ctx, id, bson.M{"$set": set}); err != nil {
		return fmt.Errorf("set screenshot: %w", err)
	}
	return nil
}

// Readme returns an item's README, or "" when it has none.
func (s *Store) Readme(ctx context.Context, source, externalID string) (string, error) {
	var b struct {
		Readme string `bson:"readme"`
	}
	err := s.Blobs().FindOne(ctx, bson.M{"source": source, "externalId": externalID},
		options.FindOne().SetProjection(bson.M{"readme": 1})).Decode(&b)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("readme %s: %w", externalID, err)
	}
	return b.Readme, nil
}

// ---- reconcile ---------------------------------------------------------------

// StaleCandidates returns the external ids of live items of one source that no
// fetch has touched since cutoff, longest-untouched first.
func (s *Store) StaleCandidates(ctx context.Context, source string, cutoff time.Time, limit int) ([]string, error) {
	cur, err := s.Items().Find(ctx,
		bson.M{"source": source, "fetchedAt": bson.M{"$lt": cutoff}, "stale": bson.M{"$ne": true}},
		options.Find().SetSort(bson.D{{Key: "fetchedAt", Value: 1}}).SetLimit(int64(limit)).
			SetProjection(bson.M{"externalId": 1}))
	if err != nil {
		return nil, fmt.Errorf("stale candidates: %w", err)
	}
	var rows []struct {
		ExternalID string `bson:"externalId"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, fmt.Errorf("stale candidates: %w", err)
	}
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.ExternalID
	}
	return out, nil
}

// MarkStale tombstones an item that is gone from its source. It stays in the
// collection for its history and leaves every ranking.
func (s *Store) MarkStale(ctx context.Context, source, externalID, reason string, at time.Time) error {
	at = at.UTC()
	_, err := s.Items().UpdateOne(ctx, bson.M{"source": source, "externalId": externalID},
		bson.M{"$set": bson.M{"stale": true, "staleReason": reason, "staleAt": at, "updatedAt": at}})
	if err != nil {
		return fmt.Errorf("mark stale %s: %w", externalID, err)
	}
	return nil
}

// Exists reports whether an item is tracked.
func (s *Store) Exists(ctx context.Context, source, externalID string) (bool, error) {
	n, err := s.Items().CountDocuments(ctx, bson.M{"source": source, "externalId": externalID}, options.Count().SetLimit(1))
	if err != nil {
		return false, fmt.Errorf("exists %s: %w", externalID, err)
	}
	return n > 0, nil
}

// ErrTaken is returned by Rename when the new identity is already tracked.
var ErrTaken = errors.New("external id already tracked")

// Rename moves a GitHub item, with its history and its blob, to the identity
// its repository now has upstream.
func (s *Store) Rename(ctx context.Context, source, oldID, newID, owner, name string, at time.Time) error {
	_, err := s.Items().UpdateOne(ctx, bson.M{"source": source, "externalId": oldID},
		bson.M{"$set": bson.M{"externalId": newID, "name": name, "gh.owner": owner, "updatedAt": at.UTC()}})
	if mongo.IsDuplicateKeyError(err) {
		return ErrTaken
	}
	if err != nil {
		return fmt.Errorf("rename %s to %s: %w", oldID, newID, err)
	}
	// A blob already under the new id (left by an earlier life of that name)
	// is the stale one: the moved item's blob replaces it.
	if _, err := s.Blobs().DeleteOne(ctx, bson.M{"source": source, "externalId": newID}); err != nil {
		return fmt.Errorf("rename blob %s: %w", oldID, err)
	}
	if _, err := s.Blobs().UpdateOne(ctx, bson.M{"source": source, "externalId": oldID},
		bson.M{"$set": bson.M{"externalId": newID}}); err != nil {
		return fmt.Errorf("rename blob %s: %w", oldID, err)
	}
	return nil
}

// RefreshMetrics stores a fresh reading of an item's metrics taken outside the
// fetch shards. It counts as an observation like any fetch: the history ring
// and growth advance, and the day's first reading appends a metric snapshot.
func (s *Store) RefreshMetrics(ctx context.Context, source, externalID string, metrics map[string]float64, at time.Time) error {
	at = at.UTC()
	key := bson.M{"source": source, "externalId": externalID}
	var cur struct {
		PrimaryMetric string       `bson:"primaryMetric"`
		History       []item.Point `bson:"history"`
	}
	err := s.Items().FindOne(ctx, key, options.FindOne().SetProjection(bson.M{"primaryMetric": 1, "history": 1})).Decode(&cur)
	if err != nil {
		return fmt.Errorf("refresh metrics %s: %w", externalID, err)
	}
	set := bson.M{"metrics": metrics, "fetchedAt": at, "updatedAt": at}
	if value, ok := metrics[cur.PrimaryMetric]; ok {
		next, growth, firstToday := observe(cur.History, value, item.DayOf(at))
		set["history"], set["growth"] = next, growth
		if firstToday {
			if _, err := s.Snapshots().InsertOne(ctx, newSnapshot(source, externalID, metrics, at)); err != nil {
				return fmt.Errorf("refresh metrics %s: snapshot: %w", externalID, err)
			}
		}
	}
	if _, err := s.Items().UpdateOne(ctx, key, bson.M{"$set": set}); err != nil {
		return fmt.Errorf("refresh metrics %s: %w", externalID, err)
	}
	return nil
}

// ---- developers --------------------------------------------------------------

// Owners returns every distinct GitHub account that owns a tracked repository.
func (s *Store) Owners(ctx context.Context) ([]string, error) {
	raw, err := s.Items().Distinct(ctx, "gh.owner", bson.M{"source": item.SourceGitHub})
	if err != nil {
		return nil, fmt.Errorf("distinct owners: %w", err)
	}
	owners := make([]string, 0, len(raw))
	for _, v := range raw {
		if login, ok := v.(string); ok && login != "" {
			owners = append(owners, login)
		}
	}
	return owners, nil
}

// FreshLogins returns the developers whose profile was fetched at or after
// since, so a sweep can skip them.
func (s *Store) FreshLogins(ctx context.Context, since time.Time) (map[string]bool, error) {
	cur, err := s.Developers().Find(ctx, bson.M{"fetchedAt": bson.M{"$gte": since}},
		options.Find().SetProjection(bson.M{"login": 1}))
	if err != nil {
		return nil, fmt.Errorf("fresh logins: %w", err)
	}
	var rows []struct {
		Login string `bson:"login"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, fmt.Errorf("fresh logins: %w", err)
	}
	set := make(map[string]bool, len(rows))
	for _, r := range rows {
		set[r.Login] = true
	}
	return set, nil
}

// UpsertDevelopers writes profiles keyed by login. A profile is authoritative,
// so each write replaces every profile field; fields other jobs keep on the
// document are left alone.
func (s *Store) UpsertDevelopers(ctx context.Context, devs []domain.Developer) error {
	if len(devs) == 0 {
		return nil
	}
	models := make([]mongo.WriteModel, 0, len(devs))
	for _, d := range devs {
		models = append(models, mongo.NewUpdateOneModel().
			SetFilter(bson.M{"login": d.Login}).SetUpsert(true).
			SetUpdate(bson.M{"$set": bson.M{
				"login": d.Login, "type": d.Type, "name": d.Name, "company": d.Company, "blog": d.Blog,
				"location": d.Location, "bio": d.Bio, "twitterUsername": d.TwitterUsername,
				"followers": d.Followers, "following": d.Following, "publicRepos": d.PublicRepos,
				"avatarUrl": d.AvatarURL, "ghCreatedAt": d.GHCreatedAt, "fetchedAt": d.FetchedAt,
			}}))
	}
	if _, err := s.Developers().BulkWrite(ctx, models, options.BulkWrite().SetOrdered(false)); err != nil {
		return fmt.Errorf("upsert developers: %w", err)
	}
	return nil
}
