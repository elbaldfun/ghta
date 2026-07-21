# trending-query 变更

## ADDED Requirements

### Requirement: type 与 topic 筛选

系统 SHALL 在趋势查询接口支持 `type`（形态，精确匹配枚举值）与 `topic`（原始 GitHub topic，
含匹配，可传多值取并集）筛选，并可与领域、language、指标区间、排序自由组合。`type` 与
topics 字段 SHALL 建索引，参数 SHALL 在 Swagger/openapi 中有文档。

#### Scenario: 按形态筛选

- **WHEN** 客户端传 `type=cli`
- **THEN** 仅返回 type 为 cli 的条目

#### Scenario: 排除资料类

- **WHEN** 客户端传 `type=software`（或前端等效的资料类排除筛选）
- **THEN** 返回结果不含 awesome/tutorial/interview 类条目

#### Scenario: 按 topic 筛选

- **WHEN** 客户端传 `topic=react`
- **THEN** 返回 topics 含 react 的条目

#### Scenario: 领域 + facet 组合

- **WHEN** 客户端传 `category=infra/containers&type=cli&language=Rust`
- **THEN** 返回同时满足三个条件的条目

### Requirement: 查询返回 type 字段

趋势查询与条目详情返回体 SHALL 包含 `type` 字段，供前端渲染形态标签与筛选态。

#### Scenario: 返回体含 type

- **WHEN** 客户端查询任一条目
- **THEN** 返回对象含 `type`（字符串枚举值）
