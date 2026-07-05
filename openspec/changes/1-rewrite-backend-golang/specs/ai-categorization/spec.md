# ai-categorization 变更

## MODIFIED Requirements

### Requirement: 定时 AI 分类任务

系统 SHALL 每日定时查询未分类仓库（categoryId 字段不存在**或**为空数组，limit 1000），逐个调用 AI 分析并将结果（categoryId、categoryPath）写回仓库文档；categoryPath SHALL 真实落库。单仓库失败 SHALL 只记日志并继续下一个。

#### Scenario: 存量文档也能被选中

- **WHEN** 某仓库文档从未写入过 categoryId 字段
- **THEN** 该仓库出现在未分类查询结果中并被分析

#### Scenario: 分类结果落库

- **WHEN** AI 分析成功返回 categoryId 与 path
- **THEN** 仓库文档的 categoryId 与 categoryPath 均被持久化，重新查询可读到

### Requirement: AI 响应解析

系统 SHALL 依次尝试：(1) 将整个响应作为 JSON 解析；(2) 提取围栏内 JSON 解析。两者均失败 SHALL 报错。解析结果字段为 categoryId、path、isNewCategory、suggestedName。

#### Scenario: 裸 JSON 响应

- **WHEN** AI 直接返回 `{"categoryId": "...", ...}` 无围栏
- **THEN** 解析成功

#### Scenario: 围栏 JSON 响应

- **WHEN** AI 返回包含围栏的 JSON 文本
- **THEN** 解析成功

### Requirement: 新分类自动创建

当 AI 判定 isNewCategory 时，系统 SHALL 先按 path 查询已有分类，存在则复用其 id；不存在才创建新分类（createdBy=ai）；path 只有一段时 SHALL 创建 parentId 为 null 的根分类。

#### Scenario: AI 建议新的根分类

- **WHEN** AI 返回 isNewCategory=true 且 path 为 `blockchain`
- **THEN** 系统创建 parentId=null、level=1 的根分类，不抛校验错误
