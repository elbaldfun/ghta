# Change: Apple App Store 源适配器

## Why

多源平台的第二个数据源。App Store 在四个目标源中数据获取最正规——Apple 提供官方
iTunes RSS 榜单 feed（Top Free/Paid/Grossing，按国家与品类）与 iTunes Search/Lookup API（应用元信息），
免费、可用、合规风险低，适合作为验证通用多源架构的第一个非 GitHub 源。

## What Changes

- 新增 `appstore` Source 适配器，实现 change 1 的 Fetcher 契约：
  - 从 iTunes RSS 榜单 feed 拉取各国家/品类 Top N 应用（含 rankPosition）；
  - 用 iTunes Lookup API 补全元信息（name、developer、rating、ratingCount、category、icon、version、releaseNotes）；
  - 归一化为 TrackedItem：source=appstore、externalId=trackId（appId）、primaryMetric=rankPosition（越小越靠前）、
    metrics{ rankPosition, rating, ratingCount }，源专属信息（截图/开发者/国家榜位）入 sourceData。
- 复用通用能力：MetricSnapshot 记录每日榜位/评分 → 增长/掉榜由 change 2 的指标计算得出（榜位变化为主指标增量）；
  AI 分类（change 3）对 App Store 条目同样适用。
- 配置：抓取的国家列表、品类、榜单类型、Top N、频率走 config。
- 前端：/trending 与排行页增加 source=appstore 展示（依赖 change 5）。
- **BREAKING**: 无（纯新增源）。

## Impact

- Affected specs: 新增 capability `source-appstore`
- Affected code (Go): `internal/source/appstore`（Fetcher 实现 + iTunes 客户端）；注册进 registry；config 增字段
- 依赖：change 1（Fetcher 契约/TrackedItem/快照）、change 2（增量/排行）、change 3（分类）
- 合规：使用 Apple 官方公开 feed/API，遵守其使用条款与配额；无需抓取网页
