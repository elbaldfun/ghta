# trending-query 变更

## MODIFIED Requirements

### Requirement: 查询过滤条件解析

系统 SHALL 支持区间语法过滤：`1000..2000`（闭区间）、`>1000`、`<1000`、`1000`（精确值），分别应用于 starCount 与 openIssuesCount；stars 与 issues 参数 SHALL 均以字符串接收并使用同一解析函数；数值合法性 SHALL 用非 NaN 判断，0 为合法边界值；language 为精确匹配；非法格式 SHALL 返回 400。

#### Scenario: star 区间过滤

- **WHEN** 客户端传 `stars=1000..2000`
- **THEN** 查询条件为 starCount ≥ 1000 且 ≤ 2000

#### Scenario: issues 区间过滤不再崩溃

- **WHEN** 客户端传 `issues=10..50`
- **THEN** 查询条件为 openIssuesCount ≥ 10 且 ≤ 50，接口正常返回

#### Scenario: 0 值边界

- **WHEN** 客户端传 `issues=0..10`
- **THEN** 区间被接受，查询 openIssuesCount ≤ 10 的仓库

### Requirement: 排序与数量限制

系统 SHALL 支持 `sort=field:order`（order 为 asc/desc）排序，排序字段 SHALL 限于白名单（starCount、forkCount、openIssuesCount、fetchedAt），并 SHALL 支持别名 `stars`→`starCount`；白名单外字段返回 400。默认按 starCount 降序；limit 默认 50、上限 50，超出上限 SHALL 报错。

#### Scenario: 别名排序

- **WHEN** 客户端传 `sort=stars:desc`
- **THEN** 结果按 starCount 降序返回

#### Scenario: 非法排序字段

- **WHEN** 客户端传 `sort=readme:desc`
- **THEN** 系统返回 400 错误
