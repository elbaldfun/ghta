# perf-verify — functional verification for the API performance work

Performance changes must be **faster and identical**. This harness proves the
"identical" half; benchmarks and Mongo `explain()` prove the "faster" half.

## Three-part verification

| What | How | Proves |
|------|-----|--------|
| Behavior unchanged | `verify.sh record` before, `verify.sh diff` after | results (order, membership, counts) identical |
| Index actually used | `explain("executionStats")` in mongosh | `IXSCAN`, no in-memory `SORT`, `docsExamined ≈ nReturned` |
| Latency dropped | `verify.sh bench` (needs `hey`) | p50/p95 down |

Plus Go regression tests that lock current behavior at the service layer.

## 1. Differential replay (`verify.sh`)

```bash
# BEFORE the change — snapshot the current responses
./verify.sh record https://api.starrank.dev            # or a staging/local URL

# make the change, redeploy, then:
./verify.sh diff   https://staging-api.starrank.dev    # non-empty diff = behavior changed
```

Edit `paths.txt` to cover every filter/sort/pagination shape a change touches.
Volatile timestamps are stripped; **array order is compared verbatim** because
order is exactly what a sort/index change can break. The sweep is sequential —
against a cold/slow origin it can take minutes; prefer a warmed staging box, and
narrow `paths.txt` when iterating.

`bench` sub-command reports p50/p95 per path when `hey` is installed.

## 2. Go regression tests

Follow the existing `MONGODB_URI` integration pattern:

```bash
# ephemeral mongo
docker run -d --name m -p 47017:27017 mongo:7
MONGODB_URI="mongodb://localhost:47017" go test ./internal/service/ ./internal/repository/ -v
docker rm -f m
```

Without `MONGODB_URI` the Mongo-backed tests skip and the pure cache tests still run.

Current guards (`internal/service/trend_regression_test.go`):
- `TestListStarsDescOrderAndTotal` — default board order + total.
- `TestListPaginationNoDupesOrGapsWithTies` — **tiebreaker guard**: paging over
  tied sort keys must cover every item once. Fails if a compound index makes tie
  order non-deterministic → fix is a `_id` tiebreaker in the sort, not deleting
  the test.
- `TestListTypeFilterAndTotal` — facet filter + scoped total.
- `TestListSearchSubstringSemantics` — locks the current case-insensitive
  substring search (`auth` matches `oauth`). A `$text` index will NOT reproduce
  this; compare the delta before migrating.

`internal/service/rankcache_test.go`:
- `TestTTLCacheBlocksOnStaleRecompute` — documents that a stale hit currently
  blocks the caller for the full recompute. Update it when stale-while-revalidate
  lands (stale value returned immediately, refresh observed asynchronously).

## 3. Index verification (per index added)

```javascript
db.tracked_items.find({type:"cli"}).sort({"metrics.stars":-1}).limit(24)
  .explain("executionStats").executionStats
// want: totalKeysExamined ~ nReturned, no SORT stage, executionTimeMillis low
```
