# 分类树 v3 草案与对比

树文件：[taxonomy/taxonomy.yaml](../taxonomy/taxonomy.yaml)，迁移映射：[taxonomy/migrations.yaml](../taxonomy/migrations.yaml)。本文说明它和现有分类的差别，以及差别带来的好处。所有数字来自 2026-10-13 生产库，v3 的叶子条目数是按 topic 分布估算的，误差 ±30%。

## 一、一句话

现在是三套分类各管一摊（领域树、形态、货架），v3 合成一棵树：非应用两层，应用三层，第三层就是货架。叶子按"条目数"和"热门条目占比"两个阈值机械地拆和并，不靠感觉。

## 二、现状的问题，用数据说

### 1. 热度集中在最粗的叶子

按周增长总量排，前三个叶子吃掉 37% 的增长、36% 的热门条目（周增 ≥100 星），但只有 3 个叶子：

| 叶子 | 条目 | 周增总量 | 热门条目 |
|---|---|---|---|
| devtools/ai-coding | 1,786 | 334k | 304 |
| ai/llm | 4,341 | 173k | 287 |
| ai/agents | 2,017 | 82k | 212 |
| 全站合计（44 叶） | 97k | 1.6M | 2,230 |

点进 devtools/ai-coding，Claude Code 插件、skill 包、编码 agent 本体、MCP 服务器全混在一起。而且三者边界已经失效：topic `mcp` 在 ai/agents 出现 218 次、ai/llm 311 次、ai-coding 229 次、devtools/cli 81 次；`claude-code` 在四个叶子里都是高频 topic。

### 2. 两个垃圾桶装了最多的条目

| 桶 | 条目 | 占比 | 周增总量 |
|---|---|---|---|
| lang/utils（"通用工具库"） | 5,649 | 全站第二大叶子 | 11k |
| type = software（形态兜底） | 27,316 | GitHub 条目 39% | 不适用 |

lang/utils 的 topic 前十是 python、javascript、hacktoberfest、go、rust、java、php、algorithms、typescript、data-structures。它只是"某语言的库"，用户进来找不到任何东西。

### 3. 同一批应用分在两棵树里

领域树 apps/* 四叶 10,063 条，货架树 53 格 17,900 条，是同一批仓库的两种切法。用户找笔记应用，一条路到 apps/productivity（3,596 条，笔记、任务、白板、远程桌面混排），另一条路到 productivity/notes（219 条）。

### 4. 消费者商店的最大一类是开发者工具

| 货架大类 | 条目 |
|---|---|
| devtools/*（8 格） | 4,178 |
| security/pentest | 1,042 |
| 消费者货架合计 | 12,700 |

商店首屏第一排是 devops 工具，第二排是渗透测试。另有 4 格不足 60 条（smart-home 26、photos 35、mail 52、adblock 57），其中 smart-home 放在 games 大类下。

### 5. 按叶子条目数看，大小失衡

最大叶子 web/frontend 7,610 条，最小 blockchain/contracts 283 条，相差 27 倍。mobile/native 5,498 条把 Android 和 Swift 放在一起，而两边的开发者几乎没有交集。

## 三、v3 怎么改

### 结构

| | 现在 | v3 |
|---|---|---|
| 分类体系 | 领域树 + 形态 + 货架，三套 | 一棵树 + 形态 |
| 层级 | 领域树 2 层；货架 2 层 | 非应用 2 层；apps 分支 3 层 |
| 顶层大类 | 12 | 12（apps 保留，mobile 改 client，新增 agents） |
| 领域叶子 | 44 | 52 |
| 货架 | 53 格 + excluded | 39 格（在 apps 分支下，含补上的浏览器扩展）；excluded 变成标记 |
| 拆并规则 | 无 | 条目 > 3,000 或热门占比 > 6% 拆；< 400 并 |
| 形态 | 8 种，software 兜底 27k | 8 种不变，但 software 不再对用户展示 |

### 叶子级改动

**拆（热度或体量过大）**

| 现在 | v3 | 理由 |
|---|---|---|
| ai/agents + devtools/ai-coding + ai/llm 的应用层 | agents/coding、skills、mcp、frameworks、memory、assistants | 三叶合计 8,144 条、590k 周增，边界已失效 |
| ai/llm 剩余 | ai/llm-serving、ai/llm-training | 推理部署和训练微调的受众不同 |
| web/frontend 7,610 | web/frontend + web/data-viz；组件类并入 web/ui-components | 图表库被塞在框架里 |
| mobile/native 5,498 | client/android、client/apple | 两个平台的开发者互不看对方 |
| security/pentest 3,185 | security/offensive、reverse、recon | 逆向和 OSINT 是独立的工具族 |
| lang/utils 5,649 | lang/algorithms、parsing、concurrency-net，残余 lang/utils ≈ 2,400 | 把能命名的拿出来 |
| devtools/cli 4,038 | 解散：终端类 → devtools/terminal，linter → code-quality，git → vcs，API 类 → api-tools | cli 是形态不是领域，形态分面已有 cli |

**并（体量过小）**

| 现在 | v3 |
|---|---|
| infra/ci-cd 421 + infra/iac 420 | infra/automation 841 |
| data/search 412 + data/analytics 664 | data/analytics 1,076 |
| blockchain/protocols 795 + contracts 283 | blockchain/core 1,078 |
| 货架 selfhosted/photos 35 → cloud；social/mail 52 → messaging；security/adblock 57 → privacy；games/smart-home 26 → selfhosted/automation；ai/image-gen + media-gen → ai/generation；新增 network/browser-ext 承接领域树的 apps/browser-ext | 53 格 → 39 格 |

**移（放错地方）**

| 现在 | v3 |
|---|---|
| 货架 devtools/* 8 格 4,178 条 | 落到领域叶子 devtools/*、data/database、infra/*、gamedev/engines；商店里以"开发者应用"视图呈现（`/apps?shelf=dev`，子项仍可单独筛选，如 `?shelf=devtools/terminal`） |
| 货架 security/pentest 1,042 | security/offensive |
| 货架 ai/coding 442 | agents/coding |
| 领域 apps/* 四叶 | 删除，由 apps 三层分支替代 |

### 新树的叶子大小

| 指标 | 现在 | v3（估） |
|---|---|---|
| 最大领域叶子 | 7,610（web/frontend） | 4,500（web/backend、web/ui-components） |
| 最小领域叶子 | 283 | 500 |
| 最大 / 最小 | 27 倍 | 9 倍 |
| 前三叶热门条目占比 | 36% | 约 18% |
| 最大货架 | 1,081（devtools/devops） | 535（selfhosted/automation） |
| 不足 60 条的货架 | 4 | 0 |
| 无法命名的桶 | lang/utils 5,649 + software 27,316 | lang/utils ≈ 2,400，software 不展示 |

## 四、用户路径对比

**找 MCP 服务器**

- 现在：不知道去 ai/agents 还是 devtools/ai-coding，两边各有两百多个，还要在 ai/llm 里再找一遍。
- v3：agents/mcp 一个页面，约 800 条，按真涨排。

**找一个笔记应用**

- 现在：领域树 apps/productivity 3,596 条混排；或者去商店 productivity/notes 219 条。两条路结果不同，页面样式也不同。
- v3：apps/productivity/notes 一个 URL，GitHub 开源笔记和 App Store 笔记应用在同一货架，按来源切换。

**找 SwiftUI 库**

- 现在：mobile/native 5,498 条，Android 占六成，要再用语言筛选。
- v3：client/apple 约 2,400 条，直接是 Swift 和 Objective-C。

**找 Claude Code 的 skill**

- 现在：devtools/ai-coding 1,786 条，skill、agent 本体、插件混排；形态筛选 skill 能过滤到 703 条但跨所有领域。
- v3：agents/skills 约 700 条，形态和领域一致。

**在商店里找代理软件**

- 现在：首页第一排货架是 devops。
- v3：首页只有 10 个消费者大类，开发者应用在独立 tab。

## 五、对应用商店来源的意义

App Store 和 Chrome 商店接进来之后，它们的条目直接落在 apps 分支，用一张静态映射表把 Apple 26 个 genre、Chrome 12 个分类对到 38 个货架，不走 LLM。榜单页用商店原生维度（国家 × 榜型 × genre），货架页用 apps 分支。领域树其他分支对商店来源不适用，这是设计上的明确边界，不需要硬套。

## 六、成本与迁移

| 项 | 数量 |
|---|---|
| 机械映射（叶子改名或合并，不需 LLM） | 约 54k 条 |
| 需要 LLM 重判的条目（被拆的 8 个叶子） | 约 29k 条 |
| LLM 批调用（12 条一批） | 约 2,400 次，一次性 |
| 货架重判（devtools 大类移出，其余机械映射） | 约 4,200 条，350 次 |
| 代码改动 | 加载器允许 apps 三层；categorizer 与 storefinder 合并成一个 prompt；前端路由 `/{source}/c/{path}` 按 REDESIGN 已有方案 |

合并 prompt 后，每日增量分类从两次调用变一次，这是顺带的节省。

## 七、风险和没解决的

- agents 大类是跟着当下热度拆的。热度退了之后按同一条阈值规则再并回去，规则本身比树重要。
- 估算条目数误差 ±30%，拆出来的叶子定稿前要跑一遍真实分类看分布。
- lang/utils 还剩约 2,400 条，这是 GitHub 本身的噪音，再拆意义不大。
- devtools/devops 货架 1,081 条要拆到 infra 的三个叶子，这是迁移里唯一需要 LLM 二选的货架。
- 形态里 software 兜底 27k 的根因是 topic 稀疏，v3 不解决，要靠 README 规则或 LLM 补判。

## 八、待你决定

1. agents 独立成顶层大类，还是作为 ai 下面的叶子？独立的理由是它的受众和"训练模型的人"不同，而且体量已经和整个 web 相当。
2. 货架 38 格是否够？Apple 的 genre 有 26 个，我们比它细；Chrome 分类 12 个，我们比它细得多。
3. 商店里"开发者应用"做视图而不是分类，接受吗？实现上它们仍可按叶子筛选（`?shelf=devtools/terminal`），只是不在消费者货架首屏。

## 九、实施（2026-10-13 已落地，待发布）

### 代码

| 位置 | 内容 |
|---|---|
| `taxonomy/taxonomy.yaml` | v3 树，根节点带 `version: 3`，apps 分支三层 |
| `taxonomy/migrations.yaml` | v2→v3 的操作表：rename / merge / split / shelf（领域）、rename / merge / moved / rejudge（货架） |
| `taxonomy/topic-map.yaml` | 规则表 v3：目标全部换成新叶子，新增 agents/*、client/*、security 三叶、lang 三叶的信号 |
| `taxonomy/golden.yaml` | 路径机械迁移；拆分叶子按仓库名自动落位，行尾 `REVIEW v3` 的 81 条需人审；新叶子补样 48 条 |
| `internal/taxonomy` | `LoadTree`（版本、三层校验）、`LoadMigrations`（启动时校验目标都是叶子）、`Migrator`（幂等执行、回滚、清快照）、`Aliases`；`Sync` 只删没有条目引用的旧节点 |
| `internal/job/categorizer.go` | 按 `taxonomyVersion < 树版本` 重排队；按星数倒序；低星门槛（`CATEGORIZE_MIN_STARS` / `CATEGORIZE_HOT_WEEKLY`，默认 1000 / 50）；重分类时保留货架标签 |
| `internal/job/taxaudit.go` | 夜间审计：每叶条目占比与热门占比，超 4% / 6% 标 split，低于 0.5% 标 merge；只打标不改树 |
| `internal/service/storefinder.go` | 货架枚举 = apps 分支 39 格 + 13 个开发者叶子；`CurrentStoreVersion` 升到 2 |
| `internal/job/storefinder.go` | 判定结果同时写入 categoryPath（一棵树） |
| `internal/service/app.go` | `?shelf=dev` 视图 |
| `cmd/api/main.go` | `POST /internal/taxonomy-migrate`、`POST /internal/taxonomy-rollback?to=2`、`GET/POST /internal/taxonomy-audit`、公开 `GET /category/aliases` |
| `web/.../apps/page.tsx`、`web/src/messages/*.json` | 货架枚举与 8 语言词条 |

### 上线顺序

1. 发布后端。启动时 `Sync` 写入新叶子，旧叶子因仍被引用而保留；分类任务开始按版本重排队，但此时还没有条目落后（`taxonomyVersion` 缺失视为落后，所以其实当晚就会开始按星数倒序重分类，每晚 1,000 条）。想控制节奏就先执行第 2 步。
2. `POST /internal/taxonomy-migrate`：后台执行（返回 202；每个货架操作要扫一遍集合，全程约 25 分钟），用 `GET /internal/taxonomy-migrate` 或日志里的 `taxonomy migrate done` 取计数。改名、合并、apps 四叶换标签、货架改写在这一步完成；拆分叶子约 29k 条标回待分类，devtools/devops 的 1,081 条标回待判定。
3. `POST /internal/categorize?concurrency=2` 反复触发，或等每晚 05:00，直到 `GET /category` 的旧叶子计数归零。规则层能免费吃掉一半以上。
4. `POST /internal/store-find?drain=1` 把 devops 货架重判完。
5. 重启一次后端，`Sync` 自动删掉已无引用的旧节点。
6. 发布前端。
7. 30 天后 `ClearRollbackSnapshots`（目前只有 Go 接口，没接端点；需要时加一行）。

### 回滚

`POST /internal/taxonomy-rollback?to=2` 把所有仍带 `categoryPathPrev` 的条目恢复原标签并标为 v2 已完成；然后把 `taxonomy/*.yaml` 回退到 v2 重新发布。

### 验收

- `go test ./...`（迁移测试需要 `MONGODB_URI`）
- 迁移前后各取一份 `GET /category` 计数对照
- `REVIEW v3` 的 81 条 golden 人审后跑 `cmd/eval`
- 拆分叶子各抽 200 条人工看
