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
cmd/api            read-only HTTP server (/v1, /mcp, /health, /internal/jobs)
cmd/worker         every scheduled job (fetch, judge, rank, …)
cmd/import         one-off import of the old collections into the new ones
internal/
  api              HTTP handlers of /v1 and the /mcp tools
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
- `POST /mcp` — the MCP server for coding agents (below)
- `GET /internal/jobs` — the worker's latest runs (bearer `ADMIN_API_TOKEN`)

## MCP server

StarRank answers MCP tool calls at `https://api.starrank.dev/mcp` (Streamable
HTTP, read-only, no key). Add it to Claude Code with:

```bash
claude mcp add --transport http starrank https://api.starrank.dev/mcp
```

Other clients (Cursor, VS Code, Windsurf, …) take the same URL as a remote
HTTP MCP server, e.g. `{"mcpServers": {"starrank": {"url": "https://api.starrank.dev/mcp"}}}`.
Then ask things like "the fastest-growing Rust projects this week" or "how does
ollama/ollama rank?".

| Tool | What it returns |
| --- | --- |
| `search_repos` | repositories matching keywords, most relevant first |
| `top_repos` | a ranking, optionally by domain and/or language, by total stars or stars gained per day / week / month |
| `trending_today` | the most stars gained in the last day |
| `get_repo` | one repository: stars, growth, and its overall / language / domain ranks with the change since the previous day |
| `list_domains` | the domain taxonomy with counts (the paths `top_repos` takes) |

Every result links to its starrank.dev page. Up to 25 rows a call; calls are
rate-limited per client (about one a second, bursts of 20). Code:
[`internal/api/mcp.go`](internal/api/mcp.go).

**中文：** StarRank 提供只读的 MCP 服务器，地址 `https://api.starrank.dev/mcp`，
无需密钥。在 Claude Code 里执行上面那行 `claude mcp add` 即可；Cursor、VS Code
等客户端把同一个地址配置为远程 HTTP MCP 服务器。之后可以直接问"本周 Rust
涨星最快的项目""ollama/ollama 排第几"。工具有搜索仓库、领域或语言榜单、
今日在涨、单个仓库详情与名次（含名次变化）、领域列表；每条结果带
starrank.dev 链接，单次最多 25 条，按客户端限流。

## Jobs

All scheduled work runs in `cmd/worker` on `PIPELINE_CRON` (fetch → … → rank);
`go run ./cmd/worker -list` prints the pipeline, `-run fetch,rank` runs jobs
once. The api only reads.

## Test

```bash
go test ./...                              # unit tests
MONGODB_URI=mongodb://localhost:27017 go test ./...   # + mongo integration tests
```
