# github-trend-fetching 变更

## MODIFIED Requirements

### Requirement: 定时抓取任务

系统 SHALL 每日定时按 star 数区间分片抓取仓库数据，cron 表达式与区间字典 SHALL 来自配置。每个分片的执行状态（pending/running/done/failed）SHALL 持久化到进度集合；任务重新触发时 SHALL 跳过当日已完成分片并重试失败分片，使抓取可断点续跑。

#### Scenario: 每日全量抓取

- **WHEN** cron 触发抓取任务
- **THEN** 系统按区间字典逐片抓取，各分片状态写入进度集合

#### Scenario: 进程中断后续跑

- **WHEN** 抓取进行到第 N 个分片时进程重启，任务再次触发
- **THEN** 前 N-1 个 done 分片被跳过，从中断处继续

#### Scenario: 失败分片重试

- **WHEN** 某分片状态为 failed
- **THEN** 下次触发时该分片被重新执行

### Requirement: API 限流保护

系统 SHALL 依据 GitHub GraphQL 响应中的 `rateLimit`（remaining/resetAt/cost）实施限流：remaining 低于配置阈值时暂停请求直到 resetAt。SHALL NOT 使用无法自动恢复的本地手写计数器。

#### Scenario: 配额将尽

- **WHEN** rateLimit.remaining 低于阈值（默认 200）
- **THEN** 系统 sleep 至 resetAt 后继续，不丢弃未完成的分片

#### Scenario: 自动恢复

- **WHEN** 到达 resetAt 后配额恢复
- **THEN** 抓取继续，无需重启进程

### Requirement: 仓库数据持久化

系统 SHALL 将每页抓取结果映射后通过单次 `bulkWrite`（以 repoNameID 匹配的 upsert）写入，payload 定义单一来源；repoNameID SHALL 有唯一索引，starCount/language/fetchedAt/categoryId SHALL 有查询索引。单条映射失败 SHALL 只记日志、不中断批次。

#### Scenario: 一页数据入库

- **WHEN** GraphQL 返回一页 N 条仓库
- **THEN** 系统发起一次含 N 个 upsert 操作的 bulkWrite，新仓库插入、旧仓库覆盖更新

#### Scenario: 并发重复写入

- **WHEN** 两次写入携带相同 repoNameID
- **THEN** 唯一索引保证库中仅存一条该仓库文档
