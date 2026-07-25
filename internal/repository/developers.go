package repository

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/elbaldfun/ghta/internal/domain"
)

// DistinctOwners returns every distinct repo-owner login in the corpus. These
// are canonical GitHub account ids (the "owner" of owner/repo), the input set
// for the developer layer.
func (s *Store) DistinctOwners(ctx context.Context) ([]string, error) {
	raw, err := s.Items().Distinct(ctx, "sourceData.owner", bson.M{})
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

// FreshLogins returns the set of developer logins fetched at or after `since`,
// so a sweep can skip accounts that are already up to date (making the job
// incremental and resumable).
func (s *Store) FreshLogins(ctx context.Context, since time.Time) (map[string]struct{}, error) {
	cur, err := s.Developers().Find(ctx,
		bson.M{"fetchedAt": bson.M{"$gte": since}},
		options.Find().SetProjection(bson.M{"login": 1}),
	)
	if err != nil {
		return nil, fmt.Errorf("fresh logins: %w", err)
	}
	var rows []struct {
		Login string `bson:"login"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, fmt.Errorf("decode fresh logins: %w", err)
	}
	set := make(map[string]struct{}, len(rows))
	for _, r := range rows {
		set[r.Login] = struct{}{}
	}
	return set, nil
}

// UpsertDevelopers writes each developer profile keyed by login. The profile is
// authoritative, so every field is replaced on each write.
func (s *Store) UpsertDevelopers(ctx context.Context, devs []domain.Developer) (int, error) {
	if len(devs) == 0 {
		return 0, nil
	}
	models := make([]mongo.WriteModel, 0, len(devs))
	for _, d := range devs {
		set := bson.M{
			"login":           d.Login,
			"type":            d.Type,
			"name":            d.Name,
			"company":         d.Company,
			"blog":            d.Blog,
			"location":        d.Location,
			"bio":             d.Bio,
			"twitterUsername": d.TwitterUsername,
			"followers":       d.Followers,
			"following":       d.Following,
			"publicRepos":     d.PublicRepos,
			"avatarUrl":       d.AvatarURL,
			"ghCreatedAt":     d.GHCreatedAt,
			"fetchedAt":       d.FetchedAt,
		}
		models = append(models, mongo.NewUpdateOneModel().
			SetFilter(bson.M{"login": d.Login}).
			SetUpdate(bson.M{"$set": set}).
			SetUpsert(true))
	}
	res, err := s.Developers().BulkWrite(ctx, models, options.BulkWrite().SetOrdered(false))
	if err != nil {
		return 0, fmt.Errorf("bulk upsert developers: %w", err)
	}
	return int(res.UpsertedCount + res.ModifiedCount), nil
}
