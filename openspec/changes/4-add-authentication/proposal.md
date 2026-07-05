# Change: Google OAuth 认证与接口守卫

## Why

当前所有接口（包括分类删除、用户 CRUD）完全公开，任何人可改库；config 中的 Google OAuth 三件套（clientId/secret/callbackUrl）从未被使用。产品要对外（趋势站/付费 API）就必须有身份与权限边界。

## What Changes

- 新增 auth 包：Google OAuth 2.0 登录（golang.org/x/oauth2 + google endpoint），签发 JWT（golang-jwt）。
- User 模型扩展：googleId、email、avatar、role(admin/user)，与 OAuth 资料绑定；env 白名单（ADMIN_EMAILS）提升 admin。
- Gin 中间件 JWT 鉴权 + 角色校验：读接口（GET /trending、/trending/rising、分类树查询、OAuth 端点）公开；写接口（category/user 的增删改）要求 admin 角色。
- OpenAPI 增加 bearer auth 配置。
- **BREAKING**: category 与 user 的写接口从公开变为需要认证（读接口不变）。

## Impact

- Affected specs: `user-management`、`category-management`；新增 capability `authentication`
- Affected code (Go): 新增 `internal/handler/auth`、`internal/middleware`（JWT/Roles）；修改 `internal/domain`（User 增字段）、路由注册、OpenAPI bearer
- 新依赖: golang.org/x/oauth2、golang-jwt/jwt
