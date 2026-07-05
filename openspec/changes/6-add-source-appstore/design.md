# Design: 6-add-source-appstore

## 数据通道

- **榜单**：iTunes RSS marketing feed，形如
  `https://itunes.apple.com/{country}/rss/top{free|paid|grossing}applications/limit={N}/genre={id}/json`，
  返回有序应用列表（隐含 rankPosition = 数组序）。官方、免费、无需鉴权。
- **元信息**：iTunes Lookup API `https://itunes.apple.com/lookup?id={trackId}&country={c}`
  或 Search API，补全 rating/ratingCount/genre/developer/artwork/version。
- 榜单按 (country, chartType, genre) 维度；同一 app 可在多榜出现，externalId 用 trackId 保证唯一，
  各榜位存入 sourceData.charts[]。

## Decisions

- externalId = trackId（Apple 全局稳定）；primaryMetric = rankPosition（升序榜，越小越好）。
- 增量语义：主指标为榜位时，"上升"= rankPosition 减小；change 2 的增量计算需支持"越小越好"的指标方向
  （在 TrackedItem.primaryMetric 上标注 direction，或适配器声明）。→ 需给 change 2 的指标计算加 direction 概念。
- 频率：榜单日更足够；Lookup 批量（一次最多 ~200 id）减少请求。
- 限流：iTunes API 无官方硬配额但需自我节流（间隔 + 重试退避），复用 change 1 的重试工具。

## Risks / Trade-offs

- feed 字段有限（无精确下载量）；rating/ratingCount 作为辅助指标。
- 国家 × 品类 × 榜型组合多，首批限定少量重点组合（如 us/cn 的 top free + top grossing 主要品类），可配置扩展。
- "越小越好"指标方向需 change 2 支持——本 change 依赖或附带对指标计算的小扩展（direction 感知）。
