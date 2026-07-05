# Change: Microsoft Store 源适配器

## Why

第四个数据源，覆盖 Windows 桌面/UWP 软件生态。Microsoft Store **无公开排行 API**，榜单入口零散，
数据获取难度与合规风险在四源中最高，故排在最后。本 change 先确定可行的数据通道与合规边界再实现。

## What Changes

- 新增 `msstore` Source 适配器，实现 change 1 的 Fetcher 契约，可配置数据通道：
  - **A. 官方 StoreEdgeFD/DisplayCatalog 等非公开但可访问的接口**（若可行且合规）；
  - **B. 第三方数据源**（若有）；
  - **C. 受控自抓取**商店分类/榜单页与产品详情页。
- 归一化为 TrackedItem：source=msstore、externalId=Product Store ID（如 9WZDNCRFHVJL）、
  primaryMetric=rating 或榜位（视通道），metrics{ rating, ratingCount, rankPosition? }，源专属信息入 sourceData。
- 复用通用能力：MetricSnapshot + change 2 增量/排行 + change 3 分类。
- 前端：source=msstore 展示（依赖 change 5）。
- **BREAKING**: 无。

## Impact

- Affected specs: 新增 capability `source-microsoft-store`
- Affected code (Go): `internal/source/msstore`（Fetcher + 通道客户端）；registry 注册；config
- 依赖：change 1/2/3；合规与通道评估结论（design.md）
- 风险：接口非公开/易变、榜单口径不统一、ToS 与法律需评估——四源中最不确定
