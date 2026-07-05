# authentication 变更

## ADDED Requirements

### Requirement: Google OAuth 登录

系统 SHALL 提供 `GET /auth/google` 发起 Google OAuth 2.0 授权，回调成功后按 googleId upsert 用户（记录 email、avatar）并签发含 userId 与 role 的 JWT。

#### Scenario: 首次登录

- **WHEN** 新用户完成 Google 授权回调
- **THEN** 系统创建用户记录并返回 JWT

#### Scenario: 再次登录

- **WHEN** 已存在 googleId 的用户再次登录
- **THEN** 系统复用该用户并返回新 JWT

### Requirement: 接口访问控制

系统 SHALL 默认要求所有接口携带有效 JWT；标记 @Public 的读接口（趋势查询、分类树查询、OAuth 端点）SHALL 免认证；分类与用户的写接口 SHALL 要求 admin 角色。

#### Scenario: 未认证写请求

- **WHEN** 无 JWT 的客户端 DELETE /category/:id
- **THEN** 返回 401

#### Scenario: 非管理员写请求

- **WHEN** role=user 的 JWT 调用 POST /category
- **THEN** 返回 403

#### Scenario: 公开读

- **WHEN** 无 JWT 的客户端 GET /trending
- **THEN** 正常返回 200

### Requirement: 管理员授予

系统 SHALL 支持通过 ADMIN_EMAILS 环境变量白名单在 OAuth 登录时授予 admin 角色。

#### Scenario: 白名单邮箱登录

- **WHEN** 登录用户 email 在 ADMIN_EMAILS 中
- **THEN** 其 role 为 admin
