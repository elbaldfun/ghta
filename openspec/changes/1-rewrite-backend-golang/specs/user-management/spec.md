# user-management 变更

## MODIFIED Requirements

### Requirement: 用户 CRUD

系统 SHALL 提供用户的创建、列表、按 id 查询、更新、删除 REST 接口，数据存储于 MongoDB users 集合。更新接口 SHALL 以 id 字符串定位文档并返回更新后的文档。

#### Scenario: 创建用户

- **WHEN** 客户端 POST 合法的 CreateUserDto
- **THEN** 系统持久化并返回新用户文档

#### Scenario: 按 id 更新用户

- **WHEN** 客户端 PATCH /user/:id 且 id 存在
- **THEN** 对应文档被实际修改，响应返回更新后的字段值
