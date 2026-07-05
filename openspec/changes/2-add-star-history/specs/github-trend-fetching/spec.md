# github-trend-fetching 变更

> 快照追加已由 change 1 的通用抓取管道完成；本 change 仅在抓取完成后追加**指标计算触发**。

## MODIFIED Requirements

### Requirement: 定时抓取任务

系统 SHALL 每日定时按 star 数区间分片抓取仓库数据；全部区间抓取完成后 SHALL 触发趋势指标计算任务，
指标计算失败不影响抓取结果与已入库的快照。

#### Scenario: 抓取后触发指标计算

- **WHEN** cron 触发的抓取任务全部分片完成
- **THEN** 系统触发趋势指标计算

#### Scenario: 指标计算失败

- **WHEN** 抓取成功但指标计算抛错
- **THEN** 错误被记录，已入库的抓取数据与快照不受影响
