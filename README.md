# GHTA — Trend Aggregation Platform (Go backend)

Multi-source developer/software trend aggregator. Fetches ranking/trend data from
multiple sources (GitHub today; App Store, Chrome Web Store, Microsoft Store next),
accumulates metric history, categorizes items with AI, and serves trend insights
over a REST API.

Spec-driven: see [`openspec/`](openspec/) for the project context, capability specs,
and change proposals. This backend implements change `1-rewrite-backend-golang`.

## Architecture

- **Go** (Gin, mongo-driver, robfig/cron, slog) — replaces the retired NestJS backend in `src/`.
- **MongoDB** — `tracked_items` (source-agnostic main docs), `metric_snapshots`
  (time-series), `categories`, `users`, `fetch_runs`.
- **Source adapters** implement one `Fetcher` contract and register in a registry;
  the fetch job drives them, so new sources plug in without touching the core.

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
make run                  # or: go run ./cmd/api
```

Required env: `MONGODB_URI`, `GITHUB_API_TOKEN`. `AI_PROVIDER` is `openai` or
`deepseek` (LM Studio). Missing/invalid required config fails startup and names
the offending field. See [`.env.example`](.env.example).

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
