# trending-query Specification

## Purpose

对外提供已抓取仓库数据的查询 REST API（`GET /trending`），支持 star/issues 区间、语言过滤与排序。

## Requirements

### Requirement: 趋势仓库查询接口

系统 SHALL 提供 `GET /trending` 接口，接受可选查询参数 stars、issues、language、limit、sort，返回 `{ data: GithubTrend[] }`。接口 SHALL 在 Swagger 中有完整参数文档。

#### Scenario: 无参数查询

- **WHEN** 客户端不带任何参数请求 /trending
- **THEN** 系统按 starCount 降序返回最多 50 条仓库

### Requirement: 查询过滤条件解析

系统 SHALL 支持区间语法过滤：`1000..2000`（闭区间）、`>1000`、`<1000`、`1000`（精确值），分别应用于 starCount 与 openIssuesCount；language 参数为精确匹配。非法区间格式 SHALL 报错。

#### Scenario: star 区间过滤

- **WHEN** 客户端传 `stars=1000..2000`
- **THEN** 查询条件为 starCount ≥ 1000 且 ≤ 2000

#### Scenario: 语言过滤

- **WHEN** 客户端传 `language=TypeScript`
- **THEN** 仅返回 language 等于 TypeScript 的仓库

### Requirement: 排序与数量限制

系统 SHALL 支持 `sort=field:order`（order 为 asc/desc）排序，默认按 starCount 降序；limit 默认 50、上限 50，超出上限 SHALL 报错。

#### Scenario: 显式排序

- **WHEN** 客户端传 `sort=starCount:asc`
- **THEN** 结果按 starCount 升序返回

#### Scenario: limit 超限

- **WHEN** 客户端传 `limit=100`
- **THEN** 系统返回错误 "Limit must be less than 50"
