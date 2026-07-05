# Change: Chrome Web Store 源适配器

## Why

呼应最初 design.md 的重点（Chrome 扩展分析、chrome-stats 替代）。Chrome Web Store **无官方排行 API**，
数据获取需抓取商店页面或采购第三方数据（chrome-stats.com 等），因此落地难度与合规风险显著高于 App Store，
排在 App Store 之后。本 change 先明确数据通道与合规边界，再实现适配器。

## What Changes

- 新增 `chrome` Source 适配器，实现 change 1 的 Fetcher 契约，二选一（config 切换）数据通道：
  - **A. 第三方数据 API**（如 chrome-stats，付费）：稳定、合规清晰，成本换可靠；
  - **B. 自抓取**：解析分类/搜索页与扩展详情页，获取 users(用户数)、rating、ratingCount、category。
- 归一化为 TrackedItem：source=chrome、externalId=扩展 ID（32 位）、primaryMetric=users、
  metrics{ users, rating, ratingCount }，源专属信息（截图/权限/开发者）入 sourceData。
- 复用通用能力：MetricSnapshot 记录每日 users/rating → 增长由 change 2 计算；AI 分类适用。
- 前端：source=chrome 展示（依赖 change 5）。
- **BREAKING**: 无。

## Impact

- Affected specs: 新增 capability `source-chrome-webstore`
- Affected code (Go): `internal/source/chrome`（Fetcher + 数据通道客户端 A/B）；registry 注册；config
- 依赖：change 1/2/3；合规评估结论（见 design.md）决定默认通道
- 风险：自抓取受商店改版/反爬影响且有 ToS 考量；优先评估第三方 API 的成本/合规再定默认
