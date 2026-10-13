# StarRank 重构方案（切换版）

> 写于 2026-10-13。负责人已同意重构方向，并接受停服更新。
> 本文取代 `docs/REDESIGN.md` 第七节的"线上分四期原地演进"；REDESIGN 的现状盘点、问题诊断和
> 目标架构（第二到第六节）仍然有效，本文只在与它不同的地方重新说明。
> 待办与进度以 Obsidian 为准（父条目 `StarRank 重构 v2`），本文是方案，不是清单。

---

## 一、做法：新集合、一次导入、停服切换

1. **代码在同一个仓库里重构，不另起空仓库。** 能用的模块原样留下，结构有问题的模块重做。
2. **数据放进新集合，不在旧集合上原地改。** 新代码只读写新集合（`items`、`item_blobs`、`boards` 等），
   旧集合 `tracked_items` 一个字段都不动。
3. **导入器把旧数据一次性规范成新结构。** 所有历史遗留形态在导入时处理掉，新代码里不写任何兼容分支。
4. **停服切换，旧系统原样保留 30 天。** 回滚等于把旧镜像重新启动，不需要反向迁移数据。

为什么不是"新数据库"：`metric_snapshots`（时序集合，逻辑 729 MB）、`star_history`、`developers`
的结构没有问题，新旧代码写入格式相同。留在同一个库里共用，省掉最重的一段拷贝，也避开时序集合
跨库复制的坑。要换的只有主集合，所以用"同库新集合"。

为什么这样比原地演进好：10 月 13 日的 v3 分类迁移在生产上中断，原因是 231 条条目的 `categoryPath`
还是多标签之前的字符串形态。原地迁移要求每一段新代码都兼容全部旧形态；导入式切换把这件事
收敛到导入器一个地方，并且可以在本地用生产备份反复演练到零差异再上。

---

## 二、范围

**本次做**

- 进程拆分：`api`（只读服务）与 `worker`（全部定时任务）
- 主集合瘦身：大字段外置、各任务状态归一、标签归一
- 抓取改为字段级写入，增长值在抓取时算出
- 榜单与名次改为夜间物化，接口只读结果
- 三轮 LLM 合成一轮判定
- `/v1` 接口与明确的返回结构，前端数据层改接新接口
- 自动备份到异地、任务运行记录、健康检查带数据新鲜度
- 删除 `users` 接口、旧的手工开关、失效的 openspec change

**本次不做**（切换完成后各自单独立项）

- 按设计稿 v2 改前端界面。本次前端只换数据层，页面外观保持不变
- star 质量分、周报、RSS
- 新数据源（iOS、Chrome 等）。数据模型按 REDESIGN 5.8 留好位置，不实现
- 导航收敛与定位调整，等负责人对 `StarRank 定位改为可信信号源并收敛导航` 作出结论

---

## 三、复用与重做

行数是当前非测试 Go 代码（合计约 12,300 行）。

| 模块 | 行数 | 处理 | 说明 |
|---|---|---|---|
| `internal/source/github`、`huggingface` | 1,460 | **复用**，改输出 | 请求、限流、解析不动；归一化的输出从 `map[string]any` 改为带类型的 `gh` / `hf` 结构 |
| `internal/taxonomy`（树、规则、迁移器、审计） | 1,070 | **复用** | 只改读写的字段名；迁移器在新结构上大幅变简单 |
| `internal/provider`、`service/ai.go` | 440 | **复用** | LLM 客户端与批量解析 |
| `internal/icon`、`internal/shot` | 360 | **复用** | 图标与截图提取 |
| `service/starhistory.go` | 350 | **复用** | GH Archive 回填 |
| `cmd/eval` | 770 | **复用**，扩展 | golden 增加货架与形态的评分 |
| `internal/domain` | 280 | **重做** | 新的 `Item` 结构，见第四节 |
| `internal/repository` | 790 | **重做** | 按新集合拆成 items / blobs / boards / runs |
| `internal/job`（8 个任务） | 1,720 | **重做**框架，迁移逻辑 | 统一 `Job` 接口、运行记录、游标分批；分类、货架、替代品三个任务合一 |
| `internal/service`（榜单、应用、开发者、生态等） | 3,500 | **重做** | 请求时聚合全部改为夜间物化；保留排序与筛选的业务规则 |
| `internal/handler`、`cmd/api` | 1,700 | **重做** | `/v1`、明确的返回结构、缓存头 |
| `service/user.go`、`handler/crud.go` | 300 | **删除** | 无功能使用 |
| 前端 `lib/data.ts` | 720 | **重做** | 拆成 `lib/api/*`，接 `/v1` |
| 前端页面与组件 | — | **复用** | 只改取数调用 |

---

## 四、新数据模型

在 REDESIGN 5.2 的基础上，按分类树 v3 上线后的实际情况修订。

### 4.1 `items`（热文档，目标平均 ≤ 2.5 KB）

```jsonc
{
  "source": "github", "externalId": "owner/name",
  "name", "description", "language", "homepage", "license", "topics": [],
  "primaryMetric": "stars", "metricDirection": "desc-better",
  "metrics": { "stars", "forks", "openIssues" },
  "growth": { "d1", "d7", "d30" },                    // 抓取写入时由 history 算出，缺数据为 null
  "history": [ { "d": "2026-10-12", "v": 48210 } ],   // 主指标最近 35 天，每天一个点
  "labels": {
    "domain": ["agents/frameworks", "apps/ai/assistant"],  // 分类树叶子，含 apps 分支
    "type": "app",
    "shelf": "ai/assistant",          // 生效货架：有人工修正用修正值；"excluded" 表示不上架；空表示未判定
    "shelfOverride": "",              // 人工修正，非空时永远胜出
    "hasGui": true,
    "taglineZh": "", "taglineEn": "",
    "alternatives": [ { "name", "slug", "kind" } ],
    "tags": [],                       // 原 generatedTopics
    "by": "llm",                      // rule | llm | manual | migrated
    "version": 3,                     // 判定时的分类树版本
    "prev": null,                     // 迁移前的 domain 快照，回滚窗口过后清除
    "at": ISODate
  },
  "assets": { "iconUrl", "screenshotUrl", "shotSource" },
  "ranks": { "overall", "language", "domain": { "<path>": n }, "at" },
  "jobs": {                           // 每个任务一条同构状态
    "judge": { "status", "fail", "at" },   // 分类、货架、替代品合一后的判定任务
    "icon":  { "status", "fail", "at" },
    "shot":  { "status", "fail", "at" }
  },
  "gh": { "owner", "pushedAt", "createdAt", "archived", "platforms": [], "releaseAssets": [], "latestRelease" },
  "hf": { "task", "library", "downloads30d", "likes" },   // 仅 HuggingFace 条目
  "stale": false, "staleReason": "",
  "fetchedAt", "createdAt", "updatedAt"
}
```

与旧结构相比去掉的东西：

- `categoryId`：与 `categoryPath` 平行的一份冗余。新结构里分类只用路径标识
- `sourceData`（无类型 map）：拆成带类型的 `gh` / `hf` 和 `item_blobs`
- 顶层散落的 `analysisStatus`、`altStatus`、`iconStatus`、`shotStatus`、`store.status` 及各自的失败计数
- `dailyIncrease` / `weeklyIncrease` / `monthlyIncrease`：并入 `growth`
- `store` 子文档与 `store.version`：并入 `labels`，只保留一个版本号

### 4.2 `item_blobs`（冷数据）

`{ source, externalId, readme, readmeAt, releases }`，唯一索引 `(source, externalId)`。
只有详情页、截图任务和判定任务读它。

### 4.3 其他集合

| 集合 | 处理 |
|---|---|
| `metric_snapshots`、`star_history`、`developers` | 原样共用，不导入 |
| `categories` | 只保留分类树节点；导入时重建，旧的货架与 HF 任务节点不带走 |
| `boards`、`boards_history` | 新建，见 REDESIGN 5.2 |
| `job_runs` | 新建，90 天过期 |
| `fetch_runs` | 共用，加 30 天过期 |
| `tracked_items`、`users`、`category_suggestions` | 不导入；切换后保留 30 天再删 |

### 4.4 导入映射

| 旧字段 | 新字段 | 规范化 |
|---|---|---|
| `categoryPath`（数组或字符串） | `labels.domain` | 字符串包成数组，空串为空数组；去掉不在 v3 树里的路径并记入报告 |
| `categoryId` | 丢弃 | — |
| `type` | `labels.type` | — |
| `store.category` / `categoryOverride` | `labels.shelf` / `labels.shelfOverride` | `shelf` 取生效值；生效货架对应的 apps 叶子补进 `labels.domain` |
| `store.taglineZh/En`、`hasGui` | `labels.*` | — |
| `alternativeTo` | `labels.alternatives` | — |
| `generatedTopics` | `labels.tags` | — |
| `classifiedBy`、`taxonomyVersion` | `labels.by`、`labels.version` | 缺版本号记为 2 |
| `analysisStatus`、`store.status`、`altStatus` | `jobs.judge` | 三者都完成才算 `done`；任一待处理则 `pending` |
| `iconStatus`、`shotStatus` | `jobs.icon`、`jobs.shot` | — |
| `iconUrl`、`screenshotUrl`、`shotSource` | `assets.*` | — |
| `daily/weekly/monthlyIncrease` | `growth.d1/d7/d30` | 以 `history` 重算的值为准，与旧值的差异记入报告 |
| `metric_snapshots` 最近 35 天 | `history` | 每天取最后一个点 |
| `sourceData.readme`、`releases` | `item_blobs` | — |
| `sourceData` 其余字段 | `gh.*` / `hf.*` / 顶层 | 逐字段列白名单，名单外的字段丢弃并在报告里计数 |

导入器是一条可重复执行的命令：每次运行先清空新集合再全量导入，输出一份校验报告。
它只读旧集合，任何时候运行都不影响线上。

---

## 五、里程碑

每个里程碑一个或几个 PR，合并进 `main`。切换之前 `main` 上的新代码不发布到生产。

| 编号 | 内容 | 完成标准 | 估时 |
|---|---|---|---|
| **M0 前置** | 跑完 v3 分类迁移；droplet 升配；v1 热修通道；每日自动备份到异地；用备份搭本地演练库 | 迁移计数入档；备份可恢复；本地有一份完整生产数据 | 2 天，含等待 |
| **M1 模型与导入器** | 新 `Item` 结构；新仓储层；导入命令与校验报告 | 本地导入零报错；校验报告全绿（第六节）；`items` 存储 < 250 MB | 4 天 |
| **M2 抓取与任务框架** | `worker` 进程；`Job` 接口与运行记录；抓取字段级写入；`history` 与增长；名次、对账、图标、截图、开发者同步 | 本地连跑两天的抓取，增长值与旧系统一致；重启 worker 不影响 api | 5 天 |
| **M3 判定合一** | 一次 LLM 调用输出领域、形态、货架、文案、替代品；按版本号重跑；golden 扩展 | golden 上领域精度不低于现基线；每条目 LLM 调用数从 3 降到 1 | 3 天 |
| **M4 物化与接口** | `boards` 与历史；`/v1` 接口、返回结构、缓存头；徽章端点；任务状态与健康检查 | 任一榜单接口冷请求 < 50 ms；详情接口 < 20 KB；新旧接口榜单前 100 名一致 | 5 天 |
| **M5 前端接入** | `lib/api/*`；所有页面改接 `/v1`；补 GA | 18 个路由在本地对着新后端全部正常；构建通过 | 4 天 |
| **M6 切换** | 两次完整演练；停服切换；观察 7 天；30 天后清理旧集合 | 第七节的切换验收全部通过 | 2 天加观察 |

合计约 25 个工作日。M3 依赖 LLM 中继恢复；其余里程碑不依赖 LLM。

### 切换前的生产维护

- 切换前生产一直跑 v1。v3 迁移收尾发版后打标签 `v1-final`，并从它拉出分支 `v1`。
- 生产需要热修时，改动提到 `v1` 分支。`scripts/release.sh` 增加 `RELEASE_BASE` 变量，
  允许从 `origin/v1` 发版；默认值仍是 `origin/main`。
- 重构期间不给 v1 加功能，只修影响可用性的问题。

---

## 六、导入校验（M1 的完成标准，也是切换的闸门）

导入命令结束时输出，任何一项不通过则切换中止。

| 项 | 标准 |
|---|---|
| 条目数 | `items` 与 `tracked_items` 按来源分别相等 |
| 领域标签 | 逐条比较规范化后的路径集合，100% 相等 |
| 货架 | 生效货架逐条相等；各货架条目数与旧系统一致 |
| 形态、文案、替代品、标签 | 逐条相等 |
| 资源 | 图标、截图地址逐条相等 |
| 主指标 | `metrics` 逐条相等 |
| 增长值 | 与旧值的差异条数与分布入报告；周增长前 500 名的集合重合度 ≥ 98% |
| README | `item_blobs` 条数等于旧集合中有 README 的条数；随机 200 条内容逐字节相等 |
| 体积 | `items` 存储 < 250 MB；平均文档 ≤ 2.5 KB |
| 丢弃字段 | 白名单外的 `sourceData` 字段逐个列出名称与出现次数，由人确认 |

---

## 七、切换手册（M6 时细化）

1. 提前一天：用当天备份在本地完整演练一次，记录导入耗时。
2. 停服：Caddy 切到维护页；停 `api`、`web`。
3. 备份：`mongodump` 到异地，确认文件可列出。
4. 导入：在服务器上运行导入命令，检查校验报告。
5. 启动：新的 `api`、`worker`、`web`。`worker` 先只跑物化任务，把榜单算出来。
6. 验收：
   - 首页、排行、应用、模型、开发者、详情页各抽查，无 5xx
   - 新旧榜单前 100 名一致
   - mongod 常驻内存 < 700 MB，swap 不增长
7. 开放：Caddy 切回。
8. 当晚：看第一轮夜间抓取、增长、名次、物化是否按序完成。

预计停服 60 分钟以内，以演练实测为准。

**回滚**：停新容器，启动标签为 `v1-final` 的旧镜像。旧集合没有被动过；切换后新增的指标快照
格式与旧系统相同，旧系统可以直接继续使用。回滚窗口 30 天。

---

## 八、需要负责人做的事

| 事项 | 什么时候需要 | 说明 |
|---|---|---|
| 合并 PR | 每个里程碑 | Claude 不能合并 |
| droplet 升到 2 vCPU / 4 GB | M0，最迟 M6 之前 | 导入时新旧主集合并存，1.9 GB 不够 |
| LLM 中继充值 | M3 之前 | 判定合一要在存量上校验 |
| 备份用的对象存储 | M0 | 需要一个 S3 兼容的桶和一对密钥，放进服务器的 `.env` |
| HF 模型榜抓取频率 | M2 之前 | 默认按现状每日抓取；改成每周可以省一半抓取量 |
| 停服时间窗口 | M6 | 建议北京时间工作日上午，避开夜间任务 |

---

## 九、风险

| 风险 | 对策 |
|---|---|
| 重构周期内生产出问题 | v1 热修通道；不给 v1 加功能 |
| 导入有遗漏，上线后才发现 | 逐条校验报告是切换闸门；旧集合保留 30 天，漏掉的字段可以补导 |
| 判定合一后准确率下降 | golden 作为门槛；不达标就退回"领域单独一轮、其余合一轮" |
| 新旧系统榜单不一致引起误判 | M4 的验收直接对比新旧接口前 100 名 |
| 工期拖长 | 每个里程碑独立可验收；M5 之前随时可以停，生产不受影响 |
