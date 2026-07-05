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
cmd/api            entrypoint (config, mongo, jobs, HTTP)
internal/
  config           env loading + startup validation
  domain           TrackedItem, Category, User, FetchRun, MetricSnapshot
  repository       mongo store, indexes, bulk upsert, snapshots
  source           Fetcher contract + registry
    github         GitHub GraphQL adapter (rate-limit aware, resumable shards)
  service          trend query, category/user CRUD, AI categorization
  handler          HTTP handlers
  provider         AI provider (OpenAI / LM Studio) abstraction
  job              fetcher + categorizer cron jobs
pkg/query          range-expression parsing
api/openapi.yaml   frozen REST contract (for the frontend)
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

- `GET /health`
- `GET /trending` — filter by `source`, `stars`/`issues` ranges (`a..b`, `>n`, `<n`, `n`), `language`, `category`; `sort=field:order`; `limit≤50`
- `GET/POST/PATCH/DELETE /category`, `/category/:id` — CRUD + tree
- `GET/POST/PATCH/DELETE /user`, `/user/:id`
- `POST /internal/fetch[?source=&shard=]`, `POST /internal/categorize` — manual job triggers

## Jobs

- **Fetcher** (`FETCH_CRON`) — shards each source, resumable via `fetch_runs`, bulk-upserts items and appends daily snapshots.
- **Categorizer** (`CATEGORIZE_CRON`) — AI-categorizes pending items, creating categories as needed; escalates repeated failures to `failed`.

## Test

```bash
go test ./...                              # unit tests
MONGODB_URI=mongodb://localhost:27017 go test ./...   # + mongo integration tests
```
