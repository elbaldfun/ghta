# star-history 变更

> 快照集合模型与保留策略由 change 1 的 `trend-aggregation`（MetricSnapshot）统一提供；
> 本 change 负责基于快照的**增量计算**与**增长排行接口**，对所有源通用（GitHub 主指标为 stars）。

## ADDED Requirements

### Requirement: 趋势指标计算

系统 SHALL 在每日抓取完成后，对每个 TrackedItem 计算其主指标（如 GitHub 的 stars）的日/周/月增量
（当前值减窗口起点最近快照值）并回填条目的 daily/weekly/monthly 增量字段；窗口起点无快照的条目对应指标 SHALL 为 null。
计算 SHALL 与源无关，逐源使用其 primaryMetric。

#### Scenario: 正常计算周增量

- **WHEN** 某 GitHub 仓库 7 天前快照 stars=1000，今日 stars=1500
- **THEN** weeklyIncrease 回填为 500

#### Scenario: 新收录条目

- **WHEN** 条目仅有今日一条快照
- **THEN** 三个增量指标均为 null，不参与排行

### Requirement: 增长排行查询接口

系统 SHALL 提供 `GET /trending/rising` 接口，按指定窗口（daily/weekly/monthly，默认 weekly）的主指标增量降序返回条目，
支持 source 过滤（不指定则跨源）、分类过滤与 limit（默认/上限 50），增量为 null 的条目不出现在结果中。

#### Scenario: 查询单源周增长排行

- **WHEN** 客户端请求 /trending/rising?source=github&window=weekly&limit=20
- **THEN** 返回 GitHub 源 weeklyIncrease 最大的 20 个条目，降序排列

#### Scenario: 跨源增长排行

- **WHEN** 客户端不指定 source 请求 rising
- **THEN** 返回各源按各自主指标增量归一后的增长条目
