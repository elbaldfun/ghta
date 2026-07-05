# Tasks: 1-rewrite-backend-golang

## 1. 项目骨架

- [ ] 1.1 初始化 Go module，目录结构（cmd/internal/pkg），Gin + slog + mongo-go-driver 接入
- [ ] 1.2 config 包：env 加载 + 启动校验（MONGODB_URI/GITHUB_API_TOKEN 必填，AI_PROVIDER 枚举，PORT 数字），缺失 fatal 并指明字段
- [ ] 1.3 Mongo 连接 + 启动时 EnsureIndexes；优雅退出（signal + context）
- [ ] 1.4 .env.example、Makefile（build/run/test/lint）、Dockerfile（多阶段，单二进制）

## 2. 通用多源数据层

- [ ] 2.1 domain：TrackedItem（source/externalId/metrics/category/增量/sourceData）、Category（parentId 可空）、User、FetchRun、MetricSnapshot
- [ ] 2.2 Source 适配器契约 `Fetcher` + registry；GitHub 适配器实现（映射 repoNameID/starCount/releases→通用字段）
- [ ] 2.3 迁移脚本：存量 GitHub 数据映射进 TrackedItem；`(source, externalId)` 去重后建复合唯一索引；metrics/category/fetchedAt 查询索引
- [ ] 2.4 repository：bulkWrite upsert（match source+externalId，单一 payload 映射）

## 3. 抓取管道

- [ ] 3.1 GitHub GraphQL 客户端：查询含 rateLimit 字段；游标分页
- [ ] 3.2 限流：remaining < 阈值 sleep 至 resetAt；重试指数退避（429 读 Retry-After）
- [ ] 3.3 fetcher job：区间分片遍历 + FetchRun 状态机（断点续跑/失败重试）
- [ ] 3.4 页级 bulkWrite 入库；单条映射失败只记日志

## 4. 查询 API（建对）

- [ ] 4.1 GET /trending：stars/issues 共享 range 解析（0 合法）、language、sort 白名单 + stars 别名、limit≤50
- [ ] 4.2 分类 CRUD + 分类树递归组装（根分类 parentId=null）
- [ ] 4.3 user CRUD（update 按 id 返回更新后文档）
- [ ] 4.4 swaggo 生成 OpenAPI；**冻结 API 契约并导出 openapi.json 供前端**

## 5. AI 分类（逐仓库，解析建对）

- [ ] 5.1 IAiProvider + openai/deepseek(LM Studio) 实现，工厂按 AI_PROVIDER 选择
- [ ] 5.2 categorizer job：未分类查询用 $or（$exists:false 或 $size:0），逐仓库分析写回 categoryId+categoryPath
- [ ] 5.3 响应解析：整体 Unmarshal → 围栏提取 → 报错；prompt 去除自增矛盾规则
- [ ] 5.4 新分类创建支持根分类；createNewCategory path 查重复用

## 6. 卫生与验证

- [ ] 6.1 单测：range 解析、sort 白名单、bulkWrite 映射、分片状态机、AI 解析（裸/围栏/非法）
- [ ] 6.2 GitHub Actions：go build + go vet + golangci-lint + go test
- [ ] 6.3 端到端：跑一个小 star 区间抓取入库、GET /trending 各参数回归、AI 分类抽查
- [ ] 6.4 后端 README（启动/env/架构指向 openspec）；验证通过后移除旧 NestJS src/
