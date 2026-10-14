# 部署手册：starrank.dev

架构：单台 **DigitalOcean droplet** 上的 Docker Compose（Next.js 前端 + Go API + MongoDB），
公网流量经同机的**共享 Caddy 入口**进来。

> ⚠️ **这台机器不止跑 ghta。** opticdrop.app 也在上面，两个站共用一个 Caddy。
> Caddy 因此是独立的 compose 项目（仓库 [`droplet-edge`](https://github.com/elbaldfun/droplet-edge)，
> 服务器 `/opt/edge`），**不在本仓库的 compose 里**。ghta 在入口上只拥有一个文件：
> `deploy/starrank.caddy`。详见下文「入口（Caddy）」。

> 2026-08 前端从 Vercel 迁回 droplet：Hobby 免费额度被爬虫流量打爆，
> 团队被 fair-use 封锁（部署被拒、整站 402）。

前端所有数据获取都在服务端完成，浏览器从不直连后端，因此**后端无需 CORS**，
后端地址也不会出现在浏览器 bundle 里（`API_URL` 刻意不带 `NEXT_PUBLIC_` 前缀）。

```
用户 ──HTTPS──> starrank.dev / api.starrank.dev ──> Caddy（自动 TLS）
                              ├─> ghta-web:3000（Next.js standalone）
                              │        │ 服务端 fetch（API_URL=http://ghta-api:3000）
                              └────────┴─> ghta-api:3000 ──> mongo:27017
                                            （均不对公网发布端口）
```

## 生产环境

| 项 | 值 |
|---|---|
| 服务器 | `24.199.88.38`（主机名 `starrank`，Debian 13） |
| 配置 | 1 vCPU / 1.9 GB 内存 / 50 GB 磁盘 + 2 GB swap |
| 部署目录 | `/opt/ghta`（compose 文件源在仓库 `deploy/docker-compose.yml`） |
| 入口 | `/opt/edge`（共享 Caddy，源在 `droplet-edge` 仓库） |
| 防火墙 | ufw 仅放行 22 / 80 / 443 |

---

## 端点权限模型

| 端点 | 权限 |
|---|---|
| `GET /health` | 公开。进程在就返回 200；正文 `fetch` 是新 worker 上次成功抓取的时间，超过 36 小时 `status` 为 `degraded`。加 `?strict=1` 时过期返回 503，给外部监控用 |
| `GET /v1/*` | 公开（重构后的只读接口，含 `/v1/badge/{owner}/{name}.svg`） |
| `GET /trending`、`/trending/rising`、`/trending/item` | 公开（前端唯一使用的接口） |
| `/category` 全部 CRUD | **需 token**（含删除） |
| `POST /internal/*` | **需 token**（触发抓取/归类等昂贵任务） |
| `GET /internal/jobs` | **需 token**：新 worker 每个任务的最近一次运行，和最近的运行记录（`?job=&limit=`） |

鉴权：`Authorization: Bearer <ADMIN_API_TOKEN>`。
**未配置 `ADMIN_API_TOKEN` 时管理端点一律 503**（fail-closed），漏配不会变成对公网敞开。

---

## 后端：日常操作

```bash
ssh 24.199.88.38
cd /opt/ghta

docker compose ps                 # 状态
docker compose logs -f api        # 日志
docker compose restart api        # 重启
```

### 发布新版本

所有发布都走 `scripts/release.sh`（Claude Code 里说“发布版本”会按
`.claude/skills/release` 自动执行同一套流程）：

```bash
scripts/release.sh status          # 线上 api / web 各是哪个 commit + 最近发布记录
scripts/release.sh api             # 发后端
scripts/release.sh web             # 发前端
scripts/release.sh all             # 两个都发，后端先行
scripts/release.sh compose         # 同步 deploy/docker-compose.yml 并重建有变化的服务
scripts/release.sh site            # 发布 deploy/starrank.caddy 到共享 Caddy
scripts/release.sh rollback api    # 回滚到上一个镜像（web 同理）
scripts/release.sh worker          # 发任务进程（重构切换之后才用，见下）
```

`worker` 是重构后跑全部定时任务的进程（`docs/REBUILD.md`）。它和 `api` 是同一个镜像的
两个入口，所以：

- compose 里 `worker` 服务挂在 `worker` profile 后面，`docker compose up -d` 不会启动它。
  **切换之前不要启动**：旧 `api` 还在跑旧任务，两边会把全量抓取各做一遍。
- 第一次启动和之后的更新都用 `scripts/release.sh worker`。`all` 在 worker 已经上线后会
  顺带重启它（顺序 api → worker → web）。
- 回滚 `api` 会把 worker 一起带回上一个镜像；没有单独的 `rollback worker`。
- worker 不对外提供服务，发布后的检查是"启动 10 秒后仍在运行且没有被 docker 重启过"。
  看日志：`docker logs --tail 50 ghta-worker-1`。每次任务运行在 `job_runs` 集合里有一条记录。
- 手动跑一次某个任务：`docker compose run --rm worker -run rank`（任务名见 `-list`）。
- `boards` 任务接在 `rank` 后面，把开发者、话题、生态、热力图、分类计数、形态分面、语言和
  停更统计算好整块写进 `boards`，接口只读结果；排名类榜单的前 100 名每天另存一份到
  `boards_history`（保留 400 天）。改了榜单规则想马上生效：`-run boards`，全库两三秒。

脚本做的事：

1. 要求工作区干净、HEAD 已并入 `origin/main`（只发已合并的代码；紧急情况用
   `RELEASE_ALLOW_UNMERGED=1` 跳过）。
2. `go vet` + `go test`（仅 api）。
3. 交叉编译 `linux/amd64` 镜像——本机是 arm64、服务器是 amd64，**必须交叉编译**。
   镜像带 `org.opencontainers.image.revision` 标签，记录对应的 commit。
4. 把线上正在用的镜像另存为 `:prev`，再 `docker save | ssh docker load` 传过去
   （没有镜像仓库）。
5. 确认服务器上的 compose 与仓库一致，然后 `docker compose up -d <服务>`。
6. 等健康检查返回 200，并确认容器跑的确实是新镜像。
7. 往服务器 `/opt/ghta/releases.log` 追加一行发布记录。

**顺序：后端先行。** 前端可能发送新参数（如新的 `sort` 值），老 API 会返回 400。

### 分支与发布

| 分支 | 用途 | 怎么进去 |
|---|---|---|
| `develop` | 集成分支，所有 PR 的目标 | PR，CI 通过后自动合并 |
| `main` | 生产分支，`release.sh` 只发它包含的提交 | `sync-main.yml` 从 `develop` 同步；热修 PR |

- 自动合并：CI 的 `build-test` 与 `web` 都通过后，工作流用合并提交合入 PR。草稿、带 `hold` 标签、来自 fork、
  或改动了 `.github/workflows` 的 PR 不会被自动合并。
- 同步：GitHub → Actions → "Sync develop to main" → Run workflow。仓库变量 `AUTO_SYNC_MAIN` 设为 `true` 后，
  每次 `develop` 有推送都会自动同步。
- 重构期间（`docs/REBUILD.md`）不设这个变量：`develop` 带着切换前不能上生产的代码。切换当天手动同步一次，
  之后再打开自动同步。
- 生产热修：从 `main` 拉分支，PR 进 `main`，发版，然后开一个 `main` → `develop` 的 PR 把修复合回去。
- 同步内容里包含 `.github/workflows` 的改动时，GitHub 会拒绝机器人的推送，那一次需要手动把 `develop` 合进 `main`。

### 判定任务（judge）

`judge` 用大模型给条目定领域、形态、货架、文案、替代品和标签，一次调用全部给出
（`internal/judge`）。模型是局域网里的 LM Studio，droplet 连不到，所以这个任务不在 droplet
的 worker 里跑：只有设置了 `JUDGE_BASE_URL` 的 worker 流水线里才有 `judge`。

- 在能连到模型的机器上跑：`JUDGE_BASE_URL=http://<模型机>:1234/v1 JUDGE_MODEL=<模型名>
  MONGODB_URI=… MONGODB_DB=ghta WORKER_NAME=judge ./worker -run judge`。
  `WORKER_NAME` 要和 droplet 上的 `pipeline` 不同：worker 启动时只收尾自己名下没跑完的记录。
- 每条问两遍，第二遍换一批同伴，只问领域、形态、货架。两遍一致记 `labels.confidence: agreed`；
  不一致问第三遍取多数，记 `majority`；三遍各不相同记 `split`：领域和形态保留第一遍的答案，货架没有多数就先不上架，等人工复核。
  复核时按 `labels.confidence: "split"` 查出来分批处理。
- 一次运行最多处理 `JUDGE_MAX_PER_RUN` 条（默认 3000）。星数低于 `JUDGE_MIN_STARS`（默认 1000）
  的条目只有周增长达到 `JUDGE_HOT_WEEKLY`（默认 50）才判定。
- 模型调用连续失败 3 次任务就报错退出，没写完的条目保持 pending，下次接着跑。
- 换提示词后要全量重判：把 `internal/judge` 的 `PromptVersion` 加一。
- 换分类树不用手工操作：`migrate` 任务每晚排在 `judge` 之前，按 `taxonomy/migrations.yaml`
  把能机械改写的标签改掉（改名、合并、货架改名），把要重判的条目（拆分的叶子、要重判的货架）
  标回待判定。重判完成前条目保留旧标签。改写前的领域存在 `labels.prev`，只存第一次。
- 评测：`JUDGE_BASE_URL=… JUDGE_MODEL=… go run ./cmd/eval -judge`，在 golden 上按同样的
  两遍流程打分。

### 手动触发任务

```bash
TOKEN=$(ssh 24.199.88.38 'grep ^ADMIN_API_TOKEN= /opt/ghta/.env | cut -d= -f2')
curl -X POST -H "Authorization: Bearer $TOKEN" https://api.starrank.dev/internal/metrics
```

### 备份

数据库不对外暴露，备份需在服务器上做：

```bash
ssh 24.199.88.38 'cd /opt/ghta && set -a && . ./.env && set +a && \
  docker compose exec -T mongo mongodump \
    --uri="mongodb://$MONGO_ROOT_USER:$MONGO_ROOT_PASSWORD@localhost:27017/ghta?authSource=admin" \
    --gzip --archive' > backup-$(date +%F).archive.gz
```

> ⚠️ `metric_snapshots` 是 **timeseries** 集合。用受限权限的账号 `mongodump` 会因无权读
> `system.buckets.*` 而失败；上面的命令用的是 root 账号所以没问题。若换用受限账号，
> 需改走 `mongoexport` / `mongoimport`（目标端须先由 API 的 `EnsureSchema` 建好集合）。

---

## 前端：droplet 上的 Next.js（standalone）

镜像 `ghta-web`，源在 `web/Dockerfile`；compose 服务名 `web`，由 Caddy 反代
`starrank.dev`。发布用 `scripts/release.sh web`，它等价于：

```bash
docker buildx build --platform linux/amd64 -t ghta-web:latest --load \
  --build-arg API_URL=https://api.starrank.dev \
  --build-arg NEXT_PUBLIC_SITE_URL=https://starrank.dev \
  --build-arg NEXT_PUBLIC_GA_ID=<取自 deploy/release.env> \
  -f web/Dockerfile web/
```

要点：

- **build-arg 的 `API_URL` 用公网地址**：`next build` 期间 SSG 页面会真的拉数据，
  此时容器内网的 `http://ghta-api:3000` 还不存在；运行时环境变量会覆盖为内网地址。
- `NEXT_PUBLIC_*` 在构建时内联进浏览器 bundle，改值必须重新构建镜像。
- 博客 Markdown（`web/content/blog/`）是运行时从磁盘读的，Dockerfile 已显式
  COPY 进镜像；新增文章要重新构建镜像。
- GA 衡量 ID 写在 `deploy/release.env`（公开值，可入库）；留空则构建出的前端不带统计。
- compose 有改动时：`scripts/release.sh compose`。

## 入口（Caddy）

Caddy 是这台机器上所有站点共用的入口，由 `droplet-edge` 仓库管理。ghta 与它的
接口只有两处：

1. **站点文件** `deploy/starrank.caddy`（starrank.dev / www / api 三个域名）。改动后发布：

   ```bash
   scripts/release.sh site
   ```

   脚本会连同其他站点一起校验，通过才热重载；失败自动回滚，不影响别的站。

2. **`edge` 网络**：`web`、`api` 在 compose 里接入外部网络 `edge`，别名
   `ghta-web`、`ghta-api`，Caddy 按这两个名字反代。

> ⛔ **不要往 `/opt/ghta/` 放 `Caddyfile`，也不要在本仓库的 compose 里加回 caddy 服务。**
> 2026-08 两个项目各自整份覆盖对方的 Caddyfile / compose，互相打挂过多次
> （opticdrop.app 525）。拆成独立入口就是为了让这件事不可能再发生。

## DNS（Cloudflare）

域名注册在 Cloudflare，NS 也在 Cloudflare（`melody.ns` / `yisroel.ns`）。

| 类型 | 名称 | 值 | 代理状态 |
|---|---|---|---|
| A | `api` | `24.199.88.38` | **DNS only（灰云）** |
| A | `@` | `24.199.88.38` | **DNS only（灰云）** |
| A | `www` | `24.199.88.38` | **DNS only（灰云）** |

> ⚠️ **云朵必须是灰色。** 橙云（Proxied）会让 Caddy 的 ACME HTTP 验证拿不到证书。
> 若以后要用 Cloudflare 的缓存/防爬（橙云），需给 Caddy 换 DNS-01 验证
> （cloudflare 插件 + API token）后再开。

---

## 已知问题

- **AI 归类在生产不可用**：`AI_PROVIDER=deepseek` 指向局域网的 LM Studio
  (`192.168.50.74:1234`)，droplet 访问不到。服务器上暂设为 `openai` + 空 key，
  归类定时任务会每天失败一次，不影响抓取与展示。需要归类的话改用云端 LLM 的 key。
- **`githubtrends` 集合未迁移**：643 MB 的 NestJS 遗留数据，Go 代码完全不引用。

## 上线检查清单

- [ ] `https://starrank.dev` 打开正常且有真实数据
- [ ] `/zh` 与 `/en` 均可访问，语言与明暗主题切换正常
- [ ] 详情页统计、增长图、README 正常
- [ ] `curl https://api.starrank.dev/health` → 200
- [ ] `curl https://api.starrank.dev/category -X POST` → 401
- [ ] DevTools → Sources 全局搜 `24.199.88.38`，**搜不到**（后端地址未泄漏）
- [ ] `nmap -p 27017 24.199.88.38` 显示 filtered/closed（数据库未暴露）
