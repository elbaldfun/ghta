# category-management Specification

## Purpose

维护层级分类树（物化路径 + parentId 邻接表），供 AI 分类与前端浏览使用。

## Requirements

### Requirement: 分类数据模型

分类 SHALL 包含 name、description、parentId(ObjectId)、level（层级深度）、path（物化路径，如 `ai/llm/agents`）、createdBy（`ai` 或人工）字段。

#### Scenario: AI 创建的分类

- **WHEN** AI 分类任务创建新分类
- **THEN** createdBy 为 `ai`，path 为完整物化路径，level 等于路径段数

### Requirement: 分类 CRUD 接口

系统 SHALL 提供分类的创建、按 id 查询、更新、删除 REST 接口，入参经 class-validator DTO 校验。

#### Scenario: 创建分类

- **WHEN** 客户端 POST 合法的 CreateCategoryDto
- **THEN** 系统持久化并返回新分类文档

### Requirement: 分类树查询

系统 SHALL 提供 findAll 接口，将扁平分类列表按 parentId 递归组装为树形结构（id/name/path/children）返回。

#### Scenario: 查询完整分类树

- **WHEN** 客户端请求分类列表
- **THEN** 返回以根分类（无 parentId）为顶层的嵌套树，无子节点的分类不含 children 属性
