# github-trend-fetching Specification

## Purpose

定时从 GitHub GraphQL API 抓取高 star 仓库的结构化数据并持久化到 MongoDB，是整个系统的数据来源。

## Requirements

### Requirement: 定时抓取任务

系统 SHALL 每日定时（cron 表达式硬编码）按 star 数区间分片遍历抓取仓库数据。区间从高到低（800000★ → 1000★），star 数越低步长越小（100000 → 10），以规避 GitHub search 单查询 1000 条结果上限。

#### Scenario: 每日全量抓取

- **WHEN** cron 触发抓取任务
- **THEN** 系统按区间字典逐片调用 GraphQL 查询，直至所有区间抓取完成
- **AND** 任一区间失败时记录错误日志，任务整体终止不影响下次调度

### Requirement: GraphQL 数据查询与重试

系统 SHALL 通过 GitHub GraphQL API `search` 查询获取仓库的 name、owner、description、stargazerCount、forkCount、topics(前20)、releases(前5)、license、homepage、README(HEAD:README.md) 等字段，并用游标（endCursor）分页拉取。对 429/5xx/超时/连接重置错误 SHALL 指数退避重试，最多 5 次。

#### Scenario: 分页拉取一个 star 区间

- **WHEN** 某区间首次查询返回 hasNextPage=true
- **THEN** 系统以 endCursor 作为 after 参数继续查询，每页间隔 300ms，直到 hasNextPage=false 或 startCursor 为 null

#### Scenario: 请求瞬时失败

- **WHEN** GraphQL 请求返回 429、5xx、ECONNABORTED 或 ECONNRESET
- **THEN** 系统按 2^n 秒指数退避重试，最多 5 次，仍失败则抛出错误并记录日志

### Requirement: API 限流保护

系统 SHALL 维护每小时请求计数，超过 5000 次（GitHub API 限额）时拒绝继续请求。

#### Scenario: 达到限额

- **WHEN** 当前计数达到 5000
- **THEN** 系统抛出 "Exceeded GitHub API hourly limit" 错误并停止请求

### Requirement: 仓库数据持久化

系统 SHALL 以 `repoNameID`（`owner/name`，从仓库 URL 提取）为业务主键持久化仓库数据：已存在则整体覆盖更新并刷新 fetchedAt，不存在则新建。单条仓库处理失败 SHALL 只记日志、不中断批次。

#### Scenario: 仓库已存在

- **WHEN** 抓取到的仓库 repoNameID 在库中已存在
- **THEN** 系统用最新抓取值覆盖更新该文档的全部数据字段

#### Scenario: 新仓库

- **WHEN** repoNameID 不存在
- **THEN** 系统创建新文档，包含全部抓取字段及 fetchedAt
