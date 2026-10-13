# StarRank 平台重新分析、设计与重构方案

> 写于 2026-10-13。基于当前已上线功能、`docs/STRATEGY.md` 的产品规划、`openspec/` 的历史决策，
> 以及对生产库的实测数据。这是**待评审的方案**，不是已完成清单；评审通过后再按第七节分期开工。
> 行文约定：标 📏 的数字是今天在生产库上跑查询量到的，标 📄 的来自已有文档，其余是判断。

---

## 一、结论先行

1. **产品方向没问题，承载它的结构已经到头了。** 规划里的护城河（README 排名徽章、刷 star 检测、
   开发者图谱、周报）每一项都需要"预计算好的榜单/名次"和"能长时间跑的采集作业"，而现在的
   架构是：一个 Go 进程同时当 API 和 8 个定时作业的宿主，所有榜单在公网请求到达时现场聚合，
   一个 1 GB 的集合里 78% 是 README 正文。这周的 mongod OOM 是这个结构的症状，不是孤立 bug。
2. **重构的主线只有三条**：① API 与作业拆成两个进程；② 主集合瘦身（大字段外置、作业状态归一）；
   ③ 榜单与名次改为夜间物化、API 只读结果。三条做完，徽章和周报是顺手的事，刷 star 检测有了
   能跑长作业的地方。
3. **不换数据库、不拆微服务、不重写前端。** 现有 Go 分层和 Next.js 前端质量都不差，问题在数据
   模型和进程边界，改这两处就够。
4. **按四个阶段推进，每阶段单独可上线、可回滚**：止血（1 周）→ 瘦身（1–2 周）→ 物化（1–2 周）→
   分类归一（2 周）。产品功能从第三阶段起并行开工。

---

## 二、现状盘点

### 2.1 产品与功能

线上 starrank.dev，8 种语言，导航 6 个入口（排行 / 爆款 / AI 生态 / 应用商店 / 开发者 / 博客），
共 18 个页面路由。对照 📄 `STRATEGY.md` 的四个方向：

| 方向 | 规划项 | 状态 |
|---|---|---|
| 增长 | README 排名徽章 `/badge/{owner}/{repo}.svg` | ❌ 未做。后端 `Ranks()` 已能算名次，但每次算是 3–4 次计数查询，不能直接暴露给外链流量 |
| 增长 | SEO 长尾（详情页 + 博客） | ⚠️ 博客只有 2 篇（中英各一）；GA 测量 ID 为空，线上没有统计 |
| 增长 | 本周/本月爆款 | ✅ `/breakout`，爬 GitHub 官方榜并自动入库 |
| 护城河 | 刷 star 检测 / star 质量分 | ❌ 未做，也没有采集 stargazer 的作业 |
| 护城河 | 归类导航 | ✅ 57 节点领域树 + 8 个形态 type，归类率 99%+ |
| 护城河 | 开发者层 → "值得关注的人" | ✅ 数据层已建（📏 46,563 账号，28,776 个人，11,513 自报 X），`/developers` 已上线 |
| 首页专区 | AI 生态 / 热门话题 / 热力图 | ✅ `/ecosystem` `/topics` `/map` |
| 超出规划 | 开源应用商店（change 13/15） | ✅ 货架 46 子类、中英 tagline、截图、替代品、最佳合集 |
| 超出规划 | HuggingFace 模型榜（change 14） | ✅ `/models`，📏 58,823 条，占语料 46% |
| 变现 | 数据/API 付费 | 按规划暂缓，正确 |

一句话：**流量层和展示层做得比规划多，护城河层一项没动。** 后面第六节会说明重构如何把这两项
变成低成本工作。

### 2.2 数据资产（📏 2026-10-13 生产实测）

| 集合 | 文档数 | 逻辑大小 | 磁盘 | 索引 | 说明 |
|---|---|---|---|---|---|
| `tracked_items` | 128,993（GitHub 70,500 / HF 58,823） | 1,054 MB | 976 MB | 32 个，84 MB | 平均 14 KB/文档，最大 504 KB |
| 其中 README 正文 | 70,500 条 | **817 MB** | — | — | 占集合 78%；平均 12 KB |
| `metric_snapshots` | 2026-07-18 至今，87 天 | 729 MB | 109 MB | 71 MB | timeseries，400 天 TTL |
| `star_history` | 36,030 | 66 MB | 25 MB | | GH Archive 回填的月度曲线 |
| `developers` | 46,563 | 16 MB | 8 MB | | |
| `fetch_runs` | **121,543** | 17 MB | 5 MB | | 每天每分片一条，从不清理 |
| `categories` | 222 | | | | 领域树 57 + 货架 + HF 任务 + 遗留 |
| `category_suggestions` | 404 | | | | AI 提议的新分类，无人审 |
| `users` | 2 | | | | 测试账号，无功能使用 |

富集覆盖：归类完成 128,458 / 待处理 613 / 失败 252；周增量有值 105,626、月增量 93,826；
货架判定 18,639（其中剔除 3,135）；截图 8,174；图标 6,695；替代品关系 2,870。
`categoryPath` 仍为旧字符串格式的文档在随机样本中占 0.4%（3,000 抽 14 条；头部 2,000 抽 7 条），
change 12 的迁移基本靠重归类完成，但没有收尾。

### 2.3 运行架构

```
droplet 24.199.88.38：1 vCPU / 1.9 GB 内存 / 2 GB swap / 50 GB 盘（已用 8.6 GB）
/opt/edge   Caddy（与 opticdrop.app 共用，独立仓库 droplet-edge）
/opt/ghta   compose: mongo:7 ── api（Go，单进程）── web（Next.js 14 standalone）
```

`api` 进程里住着（UTC）：03:30 抓取 → 紧接指标计算；04:00 开发者同步；05:00 归类；05:30 货架判定；
06:00 替代品推断；06:30 图标；07:00 截图 + 对账。8 个 cron 外加 HTTP 服务，共享一份内存和
一个 CPU。前端所有数据在服务端取，`revalidate` 300–3600 秒，浏览器不直连后端。

### 2.4 代码结构

Go 约 11,000 行：`domain / repository / source / job / service / handler / taxonomy / provider`，
分层清楚。前端 `web/src` 18 路由、40 个组件、`lib/data.ts` 一个 720 行的 API 客户端。
`openspec/` 有 7 份能力规格、15 个 change（3 个归档）。

与现实脱节的文档：`openspec/project.md` 仍写着"前端 Vite + React Router""下一步 App Store /
Chrome / MS Store 源""Google OAuth"，这三项都已被实际路线取代（Next.js、HuggingFace、无登录）；
`api/openapi.yaml` 冻结于 change 1，之后的十几个端点没有进去。

---

## 三、问题诊断

按"结构性"而非"bug 清单"归类。每条给证据，并指出它挡住了规划里的哪一项。

### 3.1 一个进程扛所有事

API 与 8 个作业同进程：作业把内存吃满会连累 API，mongod 被 OOM 时 API 也在同一台机器上排队。
这周的记录（📄 本次排障）：发布时 API 冷启动 + 夜间作业 → mongod 被 kill → 前端 6 分钟无响应。

挡住的规划：刷 star 检测需要按仓库采样 stargazer（上万次 GitHub 请求、跑几小时），这种作业
不应该和公网 API 同生共死；周报需要定时生成快照，同理。

### 3.2 热集合装了冷数据

`tracked_items` 的 78% 是 README。每一次榜单查询即使投影掉 `sourceData.readme`，mongod 仍要
把整条文档读进缓存再丢字段；WiredTiger 缓存上限 0.5 GB，而文档总量 1 GB，命中率结构性地低。
这是昨晚修的 OOM 的根因之一（计数回表读了几千条带 README 的文档）。

同一文档上还叠了 5 个作业各自的状态字段（`analysisStatus`、`store.status`、`iconStatus`、
`shotStatus`、`altStatus`）以及它们的 failCount，写法各不相同。根源是 fetcher 每天整体覆盖
`sourceData`，于是后来每个作业都只能把结果堆到顶层，代码里有 4 处注释解释"为什么放顶层"。

### 3.3 榜单在请求时现场聚合

`/developers`、`/topics`、`/ecosystem`、`/category/heat`、`/apps`、`/alternatives`、`/stats/*`
全部是对主集合的 `$group`/`$unwind` 全表聚合，靠 1 小时 TTL 缓存硬扛。缓存过期的第一个请求
（常常是爬虫）要等几秒到几十秒，而且 8 个缓存各自过期、各自全表扫一遍。详情页的名次
（`Ranks()`）每次 3–4 个计数查询，昨晚加了索引才从几十秒降到毫秒，但依然是请求时计算。

挡住的规划：徽章是被外链的 SVG，流量不可控，不能每次都算名次；周报要的"本周上升 3 名"
需要名次的历史，现在名次根本不落库。

### 3.4 指标计算是 N+1

`MetricsService.Run` 对 129k 条目逐条查一次快照集合（35 天窗口），每天一次。有索引后每次
几毫秒，合计仍是十几分钟的 IO，且随语料线性增长。

### 3.5 三套分类词表、三轮 LLM

领域树（57 节点，`taxonomy.yaml`）、形态 type（8 值，`facets.yaml`）、商店货架（46 子类，
写在 Go 代码 `service.AppShelves` 里）、HF 任务（10 组）。归类、货架判定、替代品推断是三个
独立作业、三轮 LLM 调用，各自的 prompt、批大小、失败重试、版本号互不相通。货架判定做到
18,639 条就停了（只覆盖 app 语料），没有"哪一版 prompt 判的"的统一记录。

### 3.6 运维缺口

- **无自动备份。** `DEPLOY.md` 写了手动命令，没有 cron，没有异地副本。单盘单机。
- **CI 常红。** `internal/job` 两个 categorizer 测试在 main 上失败已 2 个月，大家已习惯忽略红灯。
- **无作业可观测性。** 作业只写日志；没有"上次抓取什么时候、处理了多少、失败多少"的接口，
  `/health` 只回 `ok`。昨晚发现线上跑的是修复前的二进制，也是因为没有地方能看到。
- **`fetch_runs` 永不清理**，`users`/`/user` CRUD 是 NestJS 时代的遗留面。

### 3.7 前端与接口

- 详情页接口 `/trending/item` 把整条文档（含 README）作为 JSON 返回，几百 KB；列表接口返回
  原始 `TrackedItem` 结构，`sourceData` 裸露，前端在 `mapItem` 里再翻译一遍。
- 没有版本前缀，没有 `Cache-Control`/`ETag`，Caddy 和未来的 CDN 无从缓存。
- `@vercel/analytics` 仍在依赖里；GA 组件在但 ID 为空。

---

## 四、设计原则

1. **读写分离，但不分库。** 公网读的全部是"物化好的结果"；聚合、计数、名次在夜间作业里算。
   Mongo 继续做唯一存储。
2. **热冷分离。** 榜单要扫的字段放小文档，README/发行记录放旁边的集合，按需取。
3. **一个条目一套标签模型。** 领域、形态、货架、tagline、替代品都是"标签"，一个子文档、一个
   版本号、一套重跑机制。
4. **作业是一等公民。** 统一接口、统一运行记录、串行调度、独立进程、独立内存上限。
5. **每一步都能单独上线并回滚**，不做"大爆炸"式迁移；每阶段有可量化的验收数字。
6. **为护城河功能铺路**：名次落库（徽章）、榜单历史（周报）、长作业宿主（刷 star 检测）。

---

## 五、目标架构

### 5.1 进程与部署

```
compose（/opt/ghta）
  mongo     mem_limit 1g   wiredTigerCacheSizeGB=0.5（已有）
  api       mem_limit 256m 只读 serving + admin 触发接口，不跑任何 cron
  worker    mem_limit 512m 全部定时作业，串行流水线，可随时重启不影响站点
  web       mem_limit 384m Next.js
```

同一个 Go module、同一个镜像，入口 `cmd/ghta api` / `cmd/ghta worker`（或保留 `cmd/api`
新增 `cmd/worker`，共享 `internal/`）。`release.sh` 增加 `worker` 目标，`all` 的顺序变为
api → worker → web。

内存预算（1.9 GB 机器）：mongo ≈ 0.9（缓存 0.5 + 开销）、worker 0.5、web 0.3、api 0.2、
caddy 0.1，合计 2.0，仍然紧。**建议把 droplet 升到 2 vCPU / 4 GB（约 +12 美元/月）**，这是
所有方案里性价比最高的一项；不升也能跑，但 worker 的并发只能是 1。

### 5.2 数据模型

#### `tracked_items`（热文档，目标平均 ≤ 2 KB）

```jsonc
{
  "source": "github", "externalId": "owner/name",
  "name", "description", "language", "homepage", "license", "topics": [],
  "metrics": { "stars", "forks", "openIssues" },
  "growth": { "d1": 120, "d7": 900, "d30": 3100 },         // 由下面的 history 环算出
  "history": [ { "d": "2026-10-12", "v": 48210 }, … ],     // 最近 35 天主指标，$push + $slice:-35
  "labels": {                                              // 见 5.4
    "domain": ["ai/agents", "devtools/cli"], "type": "cli",
    "shelf": "ai/coding", "shelfOverride": null, "hasGui": false,
    "taglineZh": "…", "taglineEn": "…",
    "alternatives": [ { "name": "Notion", "slug": "notion", "kind": "note-taking" } ],
    "tags": ["…"],                                         // 原 generatedTopics
    "by": "llm", "version": 3, "at": ISODate
  },
  "assets": { "iconUrl", "screenshotUrl", "shotSource" },
  "ranks": { "overall": 812, "language": 61, "domain": { "ai/agents": 4 }, "at": ISODate },
  "jobs": {                                                // 每个作业一条同构状态
    "classify": { "status": "done", "fail": 0, "at": ISODate },
    "store":    { "status": "done", "fail": 0, "at": ISODate },
    "icon":     { … }, "shot": { … }, "alt": { … }
  },
  "gh": { "pushedAt", "createdAt", "owner", "platforms": [], "releaseAssets": [], "latestRelease" },
  "stale": false, "fetchedAt", "createdAt", "updatedAt"
}
```

要点：
- **fetcher 只 `$set` 自己拥有的字段**（`name/description/metrics/gh.*/topics/fetchedAt`），不再
  整体覆盖 `sourceData`。于是所有作业结果都可以放在语义正确的位置，不必"为了不被冲掉而放顶层"。
- `sourceData` 改为按源命名的类型化子文档（`gh` / `hf`），字段显式列出；无类型的 `map[string]any`
  只保留给真正不关心的原始字段。
- `history` 是 35 天环形数组，`growth` 在抓取写入时同步算出。指标作业从"129k 次快照查询"
  变成"0 次"；`metric_snapshots` 继续追加，只服务详情页曲线和归档。
- `ranks` 由夜间作业写入（见 5.3），详情页和徽章直接读。

#### `item_blobs`（冷数据）

`{ source, externalId, readme, readmeAt, releases }`，唯一索引 `(source, externalId)`。只有详情页
和截图作业读它。热集合预计从 1 GB 降到 ≈ 200 MB，索引里不再有任何一条路径会回表读到 README。

#### `boards`（物化榜单）

```jsonc
{ "key": "developers|rising|ai", "computedAt": ISODate, "ttl": 86400,
  "rows": [ … 最多 500 行，已是前端要的 DTO … ] }
```

覆盖今天所有请求时聚合的榜单：developers（board×domain）、topics、ecosystem、heat、
apps（os×kind×shelf×sort 的头部页）、alternatives 索引、apps/best、category 树计数、type 分面、
stats（languages/staleness/licenses）。API 的实现统一成 `boards.Get(key)`，服务层里的 8 个
`ttlCache` 和聚合管道整体删除。

保留历史：`boards_history` 每日一份头部快照（只存 `externalId` 和名次），这是周报"上升 3 名"
和徽章"↑3"的数据来源。

#### 其他

- `fetch_runs` 加 30 天 TTL；新增 `job_runs`（作业名、开始/结束、处理数、失败数、错误），90 天 TTL。
- `users` 集合与 `/user` 路由删除；`categories` 只保留领域树（货架、HF 任务不再写进去）。
- 索引从 32 个收敛：热集合瘦身后，大部分"为了避免回表"的复合索引可以合并；目标 ≤ 18 个，
  以 `TestHotCountsNeverFetchDocuments` 这类执行计划测试守住。

### 5.3 作业框架（worker）

```go
type Job interface {
    Name() string
    Run(ctx context.Context) (Report, error)   // Report{Processed, Failed, Notes}
}
```

- **串行流水线**，一次一个作业，顺序固定：
  `fetch → materialize(growth 已在 fetch 内) → rank → classify → store → alt → icon → shot → reconcile → boards → archive`。
  1 vCPU 上并发作业只会互相拖慢；需要并发的在作业内部控制（如 LLM 批）。
- 每次运行写 `job_runs`；`/internal/jobs` 返回每个作业的上次运行、下次计划；`/health` 带上
  `lastFetchAt`，超过 36 小时视为不健康（前端 `/health` 探活同时就能发现作业停了）。
- 所有作业用游标分批（500 条），不 `cur.All` 整表；LLM 类作业以 `labels.version` 决定是否重跑，
  删除 `reset-analysis` 这类手工开关。
- `rank` 作业：按 `metrics.stars` 排序一次游标遍历，顺序写 `ranks.overall`；语言与领域名次各
  自在分组内计算，整表一遍 O(n log n)，几秒。
- `archive`：change 11 的快照归档（400 天 TTL 前降采样进 `star_history`），顺手并入。
- 新作业的位置就是这里：`stargazers`（刷 star 采样）、`weekly-report`。

### 5.4 分类与标签归一

三套词表保留各自的用途，但统一为**一个 `labels` 子文档、一个版本号、一次 LLM 调用**：

| 词表 | 来源文件 | 用途 | 现状 → 目标 |
|---|---|---|---|
| 领域 domain（多值） | `taxonomy/taxonomy.yaml` | 排行导航 | 规则 → LLM 兜底，不变 |
| 形态 type（单值） | `taxonomy/facets.yaml` | 把教程/清单从趋势榜分开 | 规则判定，不变 |
| 货架 shelf（单值） | **新 `taxonomy/shelves.yaml`**（从 Go 代码搬出） | 应用商店 | LLM |
| tagline / hasGui / alternatives / tags | — | 商店与搜索 | LLM |

LLM 层合并成一个"判定"调用：输入条目摘要，输出 `{domain[], type, shelf, hasGui, taglineZh,
taglineEn, alternatives[], tags[]}`。三轮调用变一轮，批大小、重试、版本号、失败升级一套逻辑。
`taxonomy/golden.yaml`（284 条）扩展为同时标注 shelf，`cmd/eval` 继续作为改 prompt 前后的
回归门槛。HF 条目的任务组不走这条管线，在适配器里直接写 `labels.domain = ["hf/<task>"]`。

风险：一个 prompt 承担更多字段，单项准确率可能下降。对策是先用 golden 对比，低于现基线
就退回"domain 单独一轮、其余合并一轮"。

### 5.5 API 层

- 路径加 `/v1` 前缀（旧路径保留一个版本周期后下线），所有响应是明确的 DTO，不再返回
  `TrackedItem` 原文；详情页拆为 `/v1/repos/{owner}/{name}`（小）、`/readme`（大）、`/history`（曲线）。
- 列表与榜单响应带 `Cache-Control: public, max-age=300` 和 `ETag`；为将来把 api 域名放到
  Cloudflare 橙云做准备。
- `GET /v1/badge/{owner}/{name}.svg`：读 `ranks` 生成 SVG，`max-age=3600`，只对已追踪仓库生成。
  这是规划里的旗舰增长功能，在 `ranks` 落库后是一个不到 200 行的 handler。
- `api/openapi.yaml` 从代码生成（swaggo 已在规划里），CI 校验与 handler 一致。
- 管理面：`/internal/jobs`（列表 / 触发）、`/internal/labels/override`（人工改标签，替代
  `store-override`），其余 `/internal/*` 手工开关删除。

### 5.6 前端

不重写。改动集中在数据层：
- `lib/data.ts` 按领域拆成 `lib/api/{repos,boards,apps,models,blog}.ts`，类型从 OpenAPI 生成。
- 详情页只在需要时取 README；列表页的响应体积预计减半。
- 删除 `@vercel/analytics`，GA ID 补上（`deploy/release.env`）。
- 页面级 `revalidate` 保持；榜单接口本身可缓存后，前端可以把多数页面的 revalidate 放宽到 1 小时。
- 徽章的"复制到 README"按钮进详情页；`/badge` 页说明用法。

### 5.7 运维与质量

- **备份**：worker 内每日作业或主机 cron：`mongodump --gzip` 后 `rclone` 到对象存储
  （DO Spaces 5 美元/月，或任一 S3 兼容），保留 14 天；每月做一次恢复演练并记录在 `DEPLOY.md`。
- **监控**：`/health` 含作业新鲜度；外部拨测（UptimeRobot 免费档）盯 `starrank.dev/en`
  和 `api.starrank.dev/health`。
- **CI**：修好两个 categorizer 测试；集成测试必须跑在 CI 的 mongo service 上，本地无 Mongo
  时 `t.Skip` 而不是失败；加执行计划测试（已有一例）。
- **发布**：`release.sh` 增加 `worker`；compose 内存上限在仓库里是真值，不在服务器上手改。
- **文档**：评审通过后用本文更新 `openspec/project.md` 的"架构决策"与"功能列表"，归档已经
  失效的 change 4/5/6/7/8（认证、Vite 前端、三个商店源）。

---

### 5.8 多源扩展（iOS / Android 应用榜、Chrome / Edge 扩展榜）

后续要接的商店类数据源与 GitHub 的差别不在适配器，而在三处模型，提前留好：

| 差异 | GitHub / HF | 商店类源 | 设计 |
|---|---|---|---|
| 主指标方向 | 越大越好（star / 下载） | 榜单名次越小越好 | `primaryMetric` + `metricDirection` 已有；`growth` 对名次源表示"上升 N 名" |
| 原始数据形态 | 每日一个数值 | 名次 × **国家** × 分类 × 日期 | 新增 `rank_snapshots` 时序集合（meta = source, externalId, country, chart），`history` 环按"默认国家"存 |
| 分类 | 领域树（我们的） | 商店自己的分类 | `labels.domain` 按源命名空间：`gh/ai/agents`、`ios/productivity`、`chrome/developer-tools`；货架 `labels.shelf` **跨源共用** |
| 质量信号 | 刷 star | 刷评分 / 刷下载 / 刷用户数 | `labels.quality` 通用结构（档位 + 理由），特征提取按源实现 |
| 物化榜单 | key = `board|domain` | key = `source|country|chart|date` | `boards` 的 key 加 source 前缀，`boards_history` 天然支持商店榜的位置历史 |

数据获取可行性（📄 `openspec/project.md`）：iOS 有官方 RSS 榜单与 Search API；Chrome Web Store、
Google Play、Microsoft Store 没有官方榜单接口，需抓页面或第三方。每个源一个 change，
各自写明数据通道、合规边界与失败降级；本方案只保证模型与管线不需要为它们返工。

## 六、重构如何直接服务产品规划

| 规划项 | 依赖的重构 | 重构后的成本 |
|---|---|---|
| README 排名徽章 | `ranks` 落库 + API 可缓存 | 1 个 handler + 1 个前端按钮 |
| "本周上升 N 名"、周报 | `boards_history` | 一个读两份快照做 diff 的作业 |
| 刷 star 检测 | worker 独立进程 + `item_blobs` 模式（stargazer 样本是另一种冷数据） | 一个新作业 + 一个 `labels.starQuality` 字段 |
| 开发者榜动态化 | boards 物化 | 已有数据，改为读 boards |
| AdSense / 内容 | 稳定不宕机 + GA 数据 | 运维阶段完成即满足 |
| 付费 API | `/v1` + 缓存头 + DTO | 加鉴权即可 |

---

## 七、分期计划与验收

| 阶段 | 内容 | 验收 | 估时 |
|---|---|---|---|
| **0 止血** | 自动备份 + 异地；`fetch_runs` TTL；补完 `categoryPath` 迁移（约 300 条）；CI 转绿；**api/worker 拆进程**（代码不变，只拆入口与 compose）；`/health` 带作业新鲜度 | 备份文件在对象存储可列出并能恢复；CI 绿；重启 worker 时站点无感知；`/apps` 不再 500 | 1 周 |
| **1 瘦身** | `item_blobs` 拆出 README/releases；fetcher 字段级 `$set`；`labels/assets/jobs` 子文档 + 迁移脚本；`history` 环 + 抓取时算 growth；索引收敛 | `tracked_items` 存储 < 250 MB；mongod 常驻 < 700 MB 且 swap 为 0；指标作业时间 < 1 分钟；执行计划测试全绿 | 1–2 周 |
| **2 物化** | `rank` 作业 + `ranks` 字段；`boards` + `boards_history`；8 个请求时聚合改读 boards；`/v1` DTO 与缓存头；徽章端点 | 详情页 p95 < 200 ms；任一榜单接口冷请求 < 50 ms；夜间作业高峰负载 < 2；徽章可用 | 1–2 周 |
| **3 分类归一** | `shelves.yaml`；单次 LLM 判定；三作业合一；golden 扩展；删手工开关 | golden 上 domain 精度不低于现基线（0.65）；LLM 日调用量减少 ≥ 50%；全语料 `labels.version` 一致 | 2 周 |
| **4 产品** | 刷 star 采样作业与质量分；周报作业；博客选题推进 | 按 `STRATEGY.md` 另行验收 | 持续 |

每个阶段一个 openspec change（编号 16–19），各自 PR、各自发布。阶段 0 和 1 之间、1 和 2 之间
都可以停下来观察一周夜间高峰再继续。

---

## 八、刻意不做的事

- **不换 Mongo**。数据模型和访问模式修正后，129k 文档对单机 Mongo 是小规模；ClickHouse /
  Meilisearch 仍按 `project.md` 的决策留作"需要时加旁路"。
- **不做微服务 / 消息队列**。两个进程、一个串行流水线，足够；队列在单机上只会多一个故障点。
- **不重写前端**，不换框架；Next 14 → 15 的升级与本方案无关，另行安排。
- **不现在做语义搜索**（change 15 v2d），它的门槛是 embedding 端点的 key，不是架构。
- **不扩新数据源**。HF 已经让语料翻倍，先把现有两源的管线做扎实。

---

## 九、风险与取舍

| 风险 | 影响 | 对策 |
|---|---|---|
| 阶段 1 的字段迁移触碰 129k 文档 | 迁移期间读写混合格式 | 新二进制先兼容读两种格式（`PathList` 的做法已验证可行），迁移脚本分批、可重入；完成后再删兼容代码 |
| 物化后数据"新鲜度"从实时变为每日 | 增长类榜单最多滞后一天 | 抓取本来就是每日一次，榜单本质上已是日级；爆款榜继续走 GitHub 官方榜实时爬取 |
| 单 LLM 判定准确率下降 | 货架或领域错放 | golden 门槛 + 回退到两轮方案 |
| 内存仍然紧张 | worker 与 mongo 互抢 | 升级 droplet；或把 worker 的 LLM 作业并发固定为 1 |
| 重构期间产品停滞 | 增长窗口 | 阶段 2 起徽章并行；阶段 0 的备份与拆进程本身就是在还最危险的债 |

---

## 十、需要你拍板的事

1. **droplet 是否升到 2 vCPU / 4 GB**（约 +12 美元/月）。我的建议：升，阶段 0 就升。
2. **README 外置的落点**：Mongo 的 `item_blobs` 集合（简单、备份一致）还是磁盘 gzip 文件
   （更省内存、但备份与迁移多一套）。我的建议：先 `item_blobs`。
3. **LLM 三轮合一**是在阶段 3 做，还是先只做数据模型归一、LLM 保持三轮。我的建议：按阶段 3 做，
   有 golden 兜底。
4. **HF 模型榜**保留与否。它占语料 46%，每天跟着全量抓取与指标计算；如果 `/models` 流量很低，
   可降为每周抓取。需要先补上 GA 再决定。
5. **`users`/`/user`、旧 `/internal/*` 手工开关、`openspec` 失效 change** 是否按本方案清理。

评审通过后，我先起草阶段 0 的 openspec change（16-split-worker-and-backup）并开工。
