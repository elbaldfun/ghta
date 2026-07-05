# Design: 8-add-source-microsoft-store

## 数据通道（高不确定性）

Microsoft Store 无稳定公开排行 API。候选：

- **A. 官方后端接口**：StoreEdgeFD / DisplayCatalog 等应用与网页使用的接口，可获取产品元信息与部分排行，
  但非正式公开、可能变更、需评估使用条款。
- **B. 第三方数据**：覆盖 MS Store 的第三方较少，需调研。
- **C. 受控自抓取**：解析商店分类/榜单页与产品详情页，取 rating/ratingCount/category 等公开展示数据。

> 决策留待实施期评估三者的可行性/稳定性/合规，择优并以通道接口抽象支持切换。

## Decisions

- externalId = Product Store ID（如 9WZDNCRFHVJL，稳定）；primaryMetric 视通道（有榜位用 rankPosition，
  否则用 rating + change 2 增长口径）。
- 若主指标为榜位，复用 change 6 引入的指标 direction（越小越好）。
- 抓取（若走 C）：尊重 robots、限速、缓存、失败降级，仅取公开数据。

## Risks / Trade-offs

- 通道最不稳定：非公开接口可能随时变更，需监控与快速修复。
- 榜单口径不统一（分类榜/热门/新品分散），需定义平台内一致的"热度"口径。
- 合规同 Chrome 源：实施前完成 ToS/法律评估，文档不预设结论。
