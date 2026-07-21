# trending-query 变更

## ADDED Requirements

### Requirement: facet 组合筛选

系统 SHALL 在趋势查询接口支持按 facet 组合筛选：`type`（形态，精确匹配）与 `tech`（技术栈，
含匹配，可传多值取并集），并可与领域主类筛选、指标区间、排序自由组合。相关字段 SHALL 建索引，
参数 SHALL 在 Swagger/openapi 中有文档。

#### Scenario: 按形态筛选

- **WHEN** 客户端传 `type=cli`
- **THEN** 仅返回 `type` 为 cli 的条目

#### Scenario: 按技术栈筛选

- **WHEN** 客户端传 `tech=rust`
- **THEN** 返回 `tech` 数组含 rust 的条目

#### Scenario: 领域 + facet 组合

- **WHEN** 客户端传 `category=infra/containers&type=cli&tech=rust`
- **THEN** 返回同时满足领域主类、形态与技术栈的条目

### Requirement: 查询返回 facet 字段

趋势查询与条目详情返回体 SHALL 包含 `type` 与 `tech` 字段，供前端渲染 facet 标签与筛选态。

#### Scenario: 返回体含 facet

- **WHEN** 客户端查询任一条目
- **THEN** 返回对象含 `type`（字符串）与 `tech`（字符串数组）
