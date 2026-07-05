# Change: Star 历史快照与趋势指标

## Why

项目名为 trend analysis，但当前抓取对已存在仓库直接覆盖更新，star 数只保留最新值——无法计算增速、周/月增长、排行变化，即无法回答"什么在流行"这一核心问题。历史快照是本项目区别于"直接搜 GitHub"的唯一数据壁垒，也是后续所有产品形态（趋势站、周报、付费 API）的前提。

通用快照集合（MetricSnapshot）与保留策略由 change 1 提供；本 change 在其之上做增量计算与排行接口，对所有源通用。

## What Changes

- 新增趋势计算任务：每日抓取完成后聚合 MetricSnapshot，逐条目按其 primaryMetric 计算日/周/月增量并回填 TrackedItem 增量字段。
- 新增 `GET /trending/rising` 接口：按日/周/月增量排序返回增长最快条目，支持 source 过滤（不指定跨源）、分类过滤、limit。
- **BREAKING**: 无。全部为新增；现有接口行为不变。

## Impact

- Affected specs: `star-history`（指标计算 + 排行）；`github-trend-fetching`（抓取后触发计算）
- Affected code (Go): 新增 `internal/service/metrics`、rising handler；抓取 job 完成后链式触发
- 依赖：change 1 的 MetricSnapshot 与 TrackedItem 已就绪；GitHub 为首批数据源
