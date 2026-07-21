# Tasks: 12-type-facet-multilabel

## 1. 评估先行

- [ ] 1.1 `taxonomy/golden.yaml`：人工标注 200+ 条目（`domain[]` + `type`）
- [ ] 1.2 `cmd/eval`：领域各级命中率、多标签 P/R、type 准确率；资料类领域覆盖率单列
- [ ] 1.3 对现网存量跑出基线数值并记录（动树之前）

## 2. 分类资产

- [ ] 2.1 新增 `taxonomy/facets.yaml`：type 枚举 + 优先级线索（awesome > interview > tutorial >
      cli > app > library > software 兜底）—— 人审
- [ ] 2.2 `taxonomy.yaml` 微调：删 `learning`；`lang` 收紧为 implementations + utils；
      `ai` 拆 media-gen；`data` 并 cache、增 analytics；补 devtools/ai-coding —— 人审
- [ ] 2.3 `topic-map.yaml`：删 learning 段映射；语言兜底去掉路由到 lang 的项
- [ ] 2.4 `internal/taxonomy`：加载 facets.yaml + `ClassifyType` 优先级映射（含单测）

## 3. 数据模型与流水线

- [ ] 3.1 `TrackedItem` 增 `type string`；`categoryPath` 由单值改 `[]string`（categorizer
      markDone 写全部命中 path；趁前端尚未消费此字段尽早改）
- [ ] 3.2 categorizer 前置 type 层（确定性，topics 空则兜底 software，不进 LLM）
- [ ] 3.3 领域 ② embedding 改收所有过阈叶子、③ LLM prompt/解析支持多 path；
      上限 `DOMAIN_MAX_LABELS`（config，默认 3）；.env.example 补充
- [ ] 3.4 资料类（type ∈ awesome/interview/tutorial）领域落空不计失败升级

## 4. 查询与导航接口

- [ ] 4.1 `/trending` 增 `type`（精确）参数并建索引；openapi.yaml 更新
      （topic 筛选与 topicNames 索引已存在，无需改动）
- [ ] 4.2 查询/详情返回体补 `type`
- [ ] 4.3 `GET /category` 树接口：过滤 createdBy=taxonomy（排除遗留 AI 分类）+ 聚合每节点
      条目 count（多标签用 $addToSet 去重，父类勿简单求和）
- [ ] 4.4 新增 `GET /category/facets`：type 枚举 + 各值 count
- [ ] 4.5 `category` 参数支持按 path 过滤（含 `/` 查 categoryPath，否则查 categoryId）；
      categoryPath 建索引

## 5. 前端导航收敛（消灭双分类系统）

- [ ] 5.1 导航树改为消费 `GET /category`（替换 rank-data.ts 硬编码 TAXONOMY，
      消灭 data.ts 的 N+1 count 请求）
- [ ] 5.2 列表查询由 `topics=...` 组合改为 `category=<path>&type=<key>`
- [ ] 5.3 分类显示名 i18n 方案定夺：taxonomy.yaml 加 nameEn 或前端按 path 映射
- [ ] 5.4 导航 URL/slug 迁移与重定向（SEO）
- [ ] 5.5 type chip 快切 + 默认榜单是否排除资料类在此定夺（产品决策）

## 6. 验证与迁移

- [ ] 6.1 单测：type 优先级/兜底、领域 top-K 与上限、资料类落空不升级、categoryPath 多值
- [ ] 6.2 集成：mongo + fake embedder/LLM 跑通 type + 领域三级多标签路径
- [ ] 6.3 分批重跑存量（先小批 → eval 校准 → 全量；重跑同时回填多值 categoryPath 与 type），
      记录 eval 前后对比
- [ ] 6.4 确认 `learning/*` 历史文档未破坏、不再是归类目标
