# trend-aggregation 变更

## ADDED Requirements

### Requirement: 通用条目模型

系统 SHALL 用统一的 TrackedItem 模型表示任意来源的可追踪条目：字段包含 source（来源枚举）、
externalId（源内唯一标识）、name、description、category[]、categoryPath、primaryMetric（主排序指标名）、
metrics（源相关指标键值）、日/周/月增量、analysisStatus、sourceData（源专属字段子文档）、fetchedAt。
`(source, externalId)` SHALL 为复合唯一索引。

#### Scenario: GitHub 条目归一化

- **WHEN** GitHub 适配器抓到一个仓库
- **THEN** 映射为 source=github、externalId=owner/name、metrics.stars=starCount，releases/README 存入 sourceData

#### Scenario: 跨源唯一性

- **WHEN** 不同源存在相同 externalId
- **THEN** 二者作为不同 TrackedItem 共存（source 不同），互不覆盖

### Requirement: 源适配器契约

系统 SHALL 定义统一的 Source 适配器接口（Fetcher），每个源实现该接口自行完成分片/分页/限流并返回归一化的 TrackedItem 列表；适配器 SHALL 注册进 source registry，由统一的抓取任务驱动执行、统一入库（bulkWrite upsert）与快照追加。

#### Scenario: 注册与驱动

- **WHEN** 抓取任务运行
- **THEN** 遍历 registry 中已注册的适配器，逐源抓取并统一持久化

#### Scenario: 新增源零改动核心

- **WHEN** 新增一个实现 Fetcher 契约的源适配器并注册
- **THEN** 无需改动入库/快照/分类/排行逻辑即可被纳入平台

### Requirement: 通用指标快照

系统 SHALL 用 MetricSnapshot 时间序列集合记录任意源条目的指标快照（meta=(source, externalId)、capturedAt、metrics），同源同条目同一 UTC 日期至多一条，只追加不更新，保留至少 400 天后自动清理。

#### Scenario: 当日快照去重

- **WHEN** 同一条目当日已有快照
- **THEN** 不再追加新快照

#### Scenario: 超期清理

- **WHEN** 快照 capturedAt 早于 400 天前
- **THEN** 被 TTL 或归档任务删除

### Requirement: 跨源趋势查询

系统 SHALL 支持按 source 过滤的统一趋势查询（不指定 source 时可跨源），复用同一套指标区间/分类/排序/分页语义。

#### Scenario: 指定单源查询

- **WHEN** 客户端请求 source=github 的趋势列表
- **THEN** 仅返回 GitHub 源条目，按其 primaryMetric 排序

#### Scenario: 跨源查询

- **WHEN** 客户端不指定 source
- **THEN** 返回跨源条目，按统一指标/分类维度过滤与排序
