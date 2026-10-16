# GHTA — Trend Aggregation Platform (Go backend)

The backend of [starrank.dev](https://starrank.dev): ranks open-source projects
(GitHub repositories, Hugging Face models) by stars and growth, labels them with
an LLM, and serves the boards over a read-only REST API. The frontend is in
[`web/`](web/).

## Architecture

- **Go** (Gin, mongo-driver, robfig/cron, slog). Two processes from one image:
  `cmd/api` only reads; `cmd/worker` runs every scheduled job.
- **MongoDB** — `items` (one document per project), `item_blobs` (README and
  releases), `boards` and `boards_history` (precomputed boards), `apps`,
  `metric_snapshots` (daily points), `star_history`, `developers`, `job_runs`.
- Design and history: [`docs/REBUILD.md`](docs/REBUILD.md); operations:
  [`docs/DEPLOY.md`](docs/DEPLOY.md).

```
cmd/api            read-only HTTP server (/v1, /health, /internal/jobs)
cmd/worker         every scheduled job (fetch, judge, rank, …)
cmd/import         one-off import of the old collections into the new ones
internal/
  api              HTTP handlers of /v1
  store            reads and writes of the new collections
  pipeline         the worker's jobs
  judge            LLM classification
  boards           ranking boards
  source           GitHub / Hugging Face adapters
  taxonomy         the category tree (taxonomy/*.yaml)
pkg/query          range-expression parsing
```

## Run locally

```bash
cp .env.example .env      # then fill GITHUB_API_TOKEN etc.
docker run -d -p 27017:27017 --name mongo mongo:7
go run ./cmd/api          # read-only API on :3000
go run ./cmd/worker -run fetch,rank,boards   # fill it
```

The api needs `MONGODB_URI`; the worker also `GITHUB_API_TOKEN` for fetching.
See [`.env.example`](.env.example).

A GitHub token: `gh auth token` (if you use the GitHub CLI) or create a classic
PAT at https://github.com/settings/tokens with the `public_repo` scope.

## Endpoints

- `GET /health` — `?strict=1` answers 503 when the last fetch is stale
- `GET /v1/*` — the read-only interface the website uses (`internal/api`)
- `GET /internal/jobs` — the worker's latest runs (bearer `ADMIN_API_TOKEN`)

## Jobs

All scheduled work runs in `cmd/worker` on `PIPELINE_CRON` (fetch → … → rank);
`go run ./cmd/worker -list` prints the pipeline, `-run fetch,rank` runs jobs
once. The api only reads.

## Test

```bash
go test ./...                              # unit tests
MONGODB_URI=mongodb://localhost:27017 go test ./...   # + mongo integration tests
```
