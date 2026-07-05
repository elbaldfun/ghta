# Tasks: 4-add-authentication

## 1. Auth 包（Go）

- [ ] 1.1 引入依赖：golang.org/x/oauth2、golang-jwt/jwt
- [ ] 1.2 `GET /auth/google` 重定向授权 + `GET /auth/google/callback` 换取用户信息
- [ ] 1.3 JWT 签发（sub=userId, role），过期时间走配置
- [ ] 1.4 config 包校验 GOOGLE_*（clientId/secret/callbackUrl）与 JWT_SECRET 必填（复用 change 1 的启动校验机制）

## 2. 用户绑定

- [ ] 2.1 User 模型扩展 googleId/email/avatar/role；googleId 唯一索引
- [ ] 2.2 OAuth 回调 upsert 用户；ADMIN_EMAILS env 白名单授予 admin

## 3. 中间件与路由保护

- [ ] 3.1 Gin JWT 鉴权中间件 + 角色校验中间件（RequireRole("admin")）
- [ ] 3.2 公开路由：GET /trending、/trending/rising、category 查询、/auth/*（不挂鉴权）
- [ ] 3.3 category/user 写接口挂鉴权 + admin 角色
- [ ] 3.4 OpenAPI bearer 声明；集成测试：未认证写请求 401、非 admin 403、公开读 200
