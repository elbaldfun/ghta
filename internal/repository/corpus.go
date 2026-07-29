package repository

import "go.mongodb.org/mongo-driver/bson"

// AppCandidateFilter matches the raw app-candidate corpus: an app/cli by form
// facet OR anything shipping platform builds, never a library. This is the
// input set for the store judgement (change 15) — before the excluded verdict.
func AppCandidateFilter() bson.M {
	return bson.M{
		"type": bson.M{"$ne": "library"},
		"$or": bson.A{
			bson.M{"type": bson.M{"$in": bson.A{"app", "cli"}}},
			bson.M{"sourceData.platforms.0": bson.M{"$exists": true}},
		},
	}
}

// AppCorpusFilter matches the user-facing app corpus: candidates minus items
// the store judgement excluded (frameworks/lists/services that slipped in via
// the topic fallback). The single $ne clause covers both stages — unprocessed
// items have no store.category and pass until judged.
func AppCorpusFilter() bson.M {
	f := AppCandidateFilter()
	f["store.category"] = bson.M{"$ne": "excluded"}
	return f
}
