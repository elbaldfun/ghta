# category-management 变更

## MODIFIED Requirements

### Requirement: 分类 CRUD 接口

系统 SHALL 提供分类的创建、按 id 查询、更新、删除 REST 接口，入参经 class-validator DTO 校验；创建/更新/删除 SHALL 要求 admin 角色，查询接口保持公开。

#### Scenario: 管理员创建分类

- **WHEN** admin 角色 JWT POST 合法的 CreateCategoryDto
- **THEN** 系统持久化并返回新分类文档

#### Scenario: 匿名删除被拒

- **WHEN** 无 JWT 的客户端 DELETE /category/:id
- **THEN** 返回 401，分类不被删除
