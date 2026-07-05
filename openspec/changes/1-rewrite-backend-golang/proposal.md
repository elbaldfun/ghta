# Change: 用 Go 重写后端服务（保留 MongoDB）

## Why

后端从 NestJS/TS 重写为 Go（团队/产品选型决策，见 project.md 架构决策）。数据库保留 MongoDB。
关键点：这是从零重建，因此现有实现里已知的 P1 正确性缺陷与 P2 效率问题（见 project.md「已知问题」）
SHALL 由新架构直接规避，而非先复刻再修补——即 Go 版本一次性交付「现有能力平价 + 正确 + 高效」。
原计划中独立的 `fix-critical-bugs` 与 `refactor-fetch-pipeline` 两个 change 因此被本 change 吸收。

## What Changes

- **通用多源模型**：引入 `TrackedItem`（source + externalId + 通用 metrics/category/增量字段 + sourceData 子文档）
  与 `Source` 适配器契约（统一 `Fetcher` 接口 → 归一化为 TrackedItem，注册进 source registry）。
  GitHub 成为第一个适配器，抓取/查询/分类/排行能力围绕通用模型构建，后续源（App Store/Chrome/MS Store）即插即用。
- **平价迁移**：将现有 5 项能力（抓取、趋势查询、分类管理、AI 分类、用户）迁移到 Go，行为对齐 specs/ 现有规格。
- **内建正确性（原 P1）**：categoryPath 落库、未分类查询兼容字段缺失、Category 根分类可空 parentId、
  issues 参数统一解析且支持 0 边界、sort 白名单 + `stars` 别名、限流可自动恢复、不打印密钥、user 更新按 id、
  AI 响应解析兼容裸 JSON。
- **内建效率（原 P2）**：repoNameID 唯一索引 + 查询字段索引；页级 bulkWrite upsert；
  依据 GraphQL `rateLimit` 头限流；FetchRun 分片进度支持断点续跑。
- **配置/日志卫生**：启动时校验必填环境变量（缺失即失败并指明字段）；结构化日志（slog）不含敏感信息；
  删除硬编码内网地址默认值。
- **API 契约不变**：REST 路径与响应结构保持兼容，前端（change 5）据此对接；OpenAPI 文档保留。
- **BREAKING**：运行时/部署方式变化（Node → Go 二进制），但对外 HTTP 契约兼容。

## Impact

- Affected specs: `github-trend-fetching`、`trending-query`、`ai-categorization`、`category-management`、
  `user-management`（正确性/效率修正）；新增 `trend-aggregation`（通用条目/源模型 + 跨源查询）、`app-configuration`
- Affected code: 新建 Go 项目（cmd/、internal/handler|service|repository|job|provider、pkg/）；
  旧 NestJS `src/` 在迁移完成、验证通过后归档或移除
- 前置：无（作为基础 change 最先执行）；后续 2/3/4 均在 Go 代码基上继续
