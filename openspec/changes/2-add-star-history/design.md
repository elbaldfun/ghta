# Design: 2-add-star-history

## Context

抓取任务每天全量扫过 1000★+ 仓库。需要在不显著增加存储与查询成本的前提下积累时间序列。

## Goals / Non-Goals

- Goals: 可计算任意仓库的日/周/月 star 增量；支持"增长最快"排行查询；存储可控。
- Non-Goals: 小时级精度；对 1000★ 以下仓库的历史；回填历史数据（从启用日开始积累）。

## Decisions

### 1. 快照集合用 MongoDB Time Series Collection

```js
db.createCollection('repo_snapshots', {
  timeseries: { timeField: 'capturedAt', metaField: 'repoNameID', granularity: 'hours' }
})
```

- 字段：`repoNameID`(meta)、`capturedAt`、`starCount`、`forkCount`、`openIssuesCount`。
- 只追加不更新；同仓库同一 UTC 日期去重（写入前按 `capturedAt` 当日范围查询，或在应用层用 `fetchedAt` 日期判断跳过）。
- 备选方案：主文档内嵌 history 数组 —— 否决，文档会无限增长且逼近 16MB 上限（README 已占大头）。

### 2. 趋势指标冗余到主文档

列表页需要"按周增量排序"，跨集合聚合太慢。抓取批次结束后跑一个聚合 pipeline：

```
snapshot(today) - snapshot(today-1/-7/-30) → bulkWrite 回填主文档
  starIncreaseDaily / starIncreaseWeekly / starIncreaseMonthly
```

窗口起点缺快照时（新收录仓库）对应指标记 null，不参与排行。

### 3. 保留策略

快照保留 400 天（覆盖年同比），TTL 索引或月度归档任务清理。粗估：50 万仓库 × 400 天 × ~100B ≈ 20GB，可接受；若超预算可对 <5000★ 仓库降采样为每周一条。

## Risks / Trade-offs

- 抓取中断日会出现快照缺口 → 指标计算取"窗口起点最近一条"而非严格等距，容忍 ±2 天。
- 本 change 依赖 change 1（Go 重写）已内建的 bulkWrite/索引才能高效回填；执行时应在 Go 代码基上进行。
