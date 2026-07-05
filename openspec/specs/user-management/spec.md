# user-management Specification

## Purpose

用户数据的基础 CRUD（当前为模板代码，尚无认证关联，保留作为后续 OAuth 的落点）。

## Requirements

### Requirement: 用户 CRUD

系统 SHALL 提供用户的创建、列表、按 id 查询、更新、删除 REST 接口，数据存储于 MongoDB users 集合。

#### Scenario: 创建用户

- **WHEN** 客户端 POST 合法的 CreateUserDto
- **THEN** 系统持久化并返回新用户文档

#### Scenario: 按 id 更新用户

- **WHEN** 客户端 PATCH /user/:id 且 id 存在
- **THEN** 系统更新对应文档并返回结果
