# category-management 变更

## MODIFIED Requirements

### Requirement: 分类数据模型

分类 SHALL 包含 name、description、parentId（可空，默认 null；根分类为 null）、level（层级深度）、path（物化路径，如 `ai/llm/agents`）、createdBy（`ai` 或人工）字段。

#### Scenario: 创建根分类

- **WHEN** 创建 parentId 为空的分类
- **THEN** 持久化成功，parentId 存为 null，level 为 1

#### Scenario: AI 创建的分类

- **WHEN** AI 分类任务创建新分类
- **THEN** createdBy 为 `ai`，path 为完整物化路径，level 等于路径段数
