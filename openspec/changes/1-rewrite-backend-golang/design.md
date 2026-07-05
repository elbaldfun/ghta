# Design: 1-rewrite-backend-golang

## Context

现有 NestJS 后端约 2500 行业务逻辑，负载纯 IO 密集。目标是等价功能的 Go 实现，保留 Mongo 库与集合，
一次性建对已知缺陷。前端（React）尚未存在，本 change 需冻结 REST API 契约供其对接。

## Goals / Non-Goals

- Goals：能力平价；P1 正确性与 P2 效率内建；启动配置校验；结构化日志；OpenAPI 保留；API 契约冻结。
- Non-Goals：指标增量计算与增长排行（change 2，本 change 只建通用快照模型不算增量）；AI 批量化（change 3）；
  认证（change 4）；前端（change 5）；GitHub 以外的源适配器（change 6/7/8，本 change 只定义契约 + GitHub 实现）。

## Decisions

### 1. 项目结构

```
cmd/api/main.go            # 装配、启动、优雅退出
internal/
  config/                  # env 加载 + 启动校验（缺 MONGODB_URI/GITHUB_API_TOKEN 即 fatal）
  handler/                 # Gin handler：trending、category、user
  service/                 # 业务逻辑：trend、category、ai
  repository/              # Mongo 访问：索引 ensure、bulkWrite、查询构造
  source/                  # Source 适配器契约 + registry
    github/                #   GitHub 适配器（本 change 实现）
  job/                     # cron：fetcher（驱动 registry）、categorizer
  provider/                # IAiProvider + openai/deepseek 实现
  domain/                  # TrackedItem / Category / User / FetchRun / MetricSnapshot
pkg/                       # 可复用工具（分页、range 解析）
```

### 2. 选型

- HTTP: **Gin**（成熟、中间件生态全）；OpenAPI: swaggo 注解生成，兼顾现有 Swagger 习惯。
- Mongo: 官方 mongo-go-driver；启动时 `EnsureIndexes`（唯一/查询索引）。
- Cron: robfig/cron，表达式来自 config。
- AI: openai-go 官方 SDK，baseURL 可指向 LM Studio；Provider 按 `AI_PROVIDER` 工厂选择。
- 校验: go-playground/validator（handler 层 DTO）。
- 日志: slog（JSON handler），字段化；敏感值不入日志。

### 2.5 通用多源模型

- **TrackedItem**：`source`(枚举)、`externalId`（源内唯一）、name、description、category[]、categoryPath、
  `primaryMetric`（该源主排序指标名，如 stars/rankPosition）、`metrics{}`（源相关键值）、
  daily/weekly/monthly 增量、analysisStatus、`sourceData{}`（源专属字段，如 GitHub 的 releases/README）、fetchedAt。
  `(source, externalId)` 复合唯一索引。
- **Source 适配器契约**：`Fetcher` 接口 `Fetch(ctx) -> []TrackedItem`（各源自行分片/分页/限流），注册进 registry；
  fetcher job 遍历 registry 执行，统一走 bulkWrite upsert + 快照追加。
- **MetricSnapshot**：Time Series 集合，meta=(source, externalId)、capturedAt、metrics{}；同源同条目同日至多一条。
- GitHub 适配器把 repoNameID→externalId、starCount/forkCount/openIssuesCount→metrics、releases/README→sourceData。
- 现有 `github-trend-fetching` / `trending-query` 规格视为 GitHub 适配器在通用模型上的具体实现。

### 3. 抓取管道（内建原 change 3）

- 页级 `bulkWrite`（`UpdateOne` + upsert，match repoNameID），映射函数单一来源。
- 限流：GraphQL query 附 `rateLimit { remaining resetAt cost }`；remaining < 阈值则 sleep 至 resetAt。
- 断点续跑：`fetch_runs` 集合记录每个 star 区间分片状态（pending/running/done/failed）；
  重触发跳过当日 done、重试 failed。
- 重试：429/5xx/超时指数退避，429 优先读 Retry-After。

### 4. 正确性对照（内建原 change 1）

| 缺陷 | Go 版做法 |
|------|-----------|
| categoryPath 丢失 | domain 模型含 categoryPath，写回时持久化 |
| $size:0 漏存量 | 未分类查询用 `$or:[{categoryId:{$exists:false}},{categoryId:{$size:0}}]` |
| 根分类 parentId | parentId 为可空指针，根分类存 null |
| issues 参数 | 统一 string 入参 + 共享 range 解析，`0` 合法边界 |
| sort 白名单 | 字段白名单 + `stars→starCount` 别名，非法返 400 |
| 限流不复位 | 基于 rateLimit 头，无本地永久计数 |
| 密钥入日志 | 启动/运行日志白名单字段，绝不打印 env 全量 |
| user update id | 按 id 定位并返回更新后文档 |
| AI 解析脆 | 先整体 JSON.Unmarshal，失败再提取围栏，再失败报错 |

### 5. 迁移与切换

- 双写期不需要：直接指向同一 Mongo，Go 版起来后停掉 Node 版 cron，避免重复抓取。
- 索引变更需先在存量集合去重（repoNameID）再建唯一索引——迁移脚本随本 change。
- 旧 `src/` 保留到 Go 版通过验证（抓取一轮 + 查询回归 + AI 分类抽查）后再移除。

## Risks / Trade-offs

- 团队 Go 熟练度：若不足，迁移周期拉长——建议先迁抓取管道（独立性最强）跑通再迁 API/AI。
- 唯一索引前需清历史重复数据；迁移脚本要幂等可回滚。
- API 契约需在本 change 冻结并写入 OpenAPI，change 5 前端据此开发，避免返工。
