# user-management 变更

## MODIFIED Requirements

### Requirement: 用户 CRUD

系统 SHALL 提供用户的列表、按 id 查询、更新、删除 REST 接口（创建由 OAuth 回调完成，不再暴露公开创建接口）；全部接口 SHALL 要求 admin 角色。用户模型 SHALL 含 googleId（唯一）、email、avatar、role(admin/user) 字段。

#### Scenario: 管理员查询用户列表

- **WHEN** admin 角色 JWT 请求 GET /user
- **THEN** 返回用户列表

#### Scenario: 普通用户访问被拒

- **WHEN** role=user 的 JWT 请求 GET /user
- **THEN** 返回 403
