# Change: 快照归档压缩（方案 C，待做）

## Why

`metric_snapshots` 是带 400 天 TTL 的时间序列集合：日粒度快照到期即被 Mongo 自动删除。change 10（star_history 按需回填）解决了"过去"——回填 2011 年至今的月度曲线；但**我们自己积累的日粒度快照满 400 天后仍会永久丢失**，完整曲线会在 400 天前出现一个精度断层并逐渐丢数据。

## What Changes

- 新增每月一次的归档任务（cron）：将快照集合中**即将满 400 天**的日粒度点按月降采样（取每月末点），追加写入对应仓库的 `star_history.points`，然后放任 TTL 删除原始点。
- 归档需幂等：同一月份重复归档不产生重复点（按月份去重后 upsert）。
- `/trending/item` 无需改动——它已经合并 star_history + snapshots 两段数据。

## Impact

- Affected specs: `star-history`
- Affected code (Go): 新增 `internal/job/archiver.go`（或并入 metrics 任务链）；cron 注册在 `cmd/api/main.go`
- 时间窗口：首批快照写于 2026-07-18，**2027-08 前完成即可**，无数据丢失风险
- 依赖：change 10 的 `star_history` 集合与合并逻辑（已实现）
