# Tasks: 12-type-facet-multilabel

## 1. 评估先行

- [x] 1.1 `taxonomy/golden.yaml`：人工标注 200+ 条目（`domain[]` + `type`）
      —— 已冻结 v1.0：284 条，44/44 叶子覆盖，Q1-Q32 两轮人工裁决记录在文件头
- [x] 1.2 `cmd/eval`：领域各级命中率、多标签 P/R、资料类覆盖率单列（type 段待 2.4 后补）
- [x] 1.3 基线已记录 → `eval-baseline.md`（rule 79% / embedding@0.35 仅 31% / LLM 71%；
      顺手修复 provider 对 LM Studio 的 response_format 兼容 bug）

## 2. 分类资产

- [x] 2.1 `taxonomy/facets.yaml` 已起草：type 枚举(含新增 skill) + 优先级 —— 待人审
- [x] 2.2 `taxonomy.yaml` v2 已定稿：删 learning、lang/utils 改名、新增 media-gen/analytics/
      ai-coding、边界裁决写进 desc；44 叶与 golden 对齐 —— 待人审
- [x] 2.3 `topic-map.yaml` v2 已定稿：删 23 条歧义规则(平台词/用了≠是/过宽AI词)+重映射 media-gen，
      迁到新树；eval 验证规则层 hit@any 79%→87%、domain precision 0.50→0.65 —— 待人审
- [ ] 2.4 `internal/taxonomy`：加载 facets.yaml + `ClassifyType` 优先级映射（含单测）

## 3. 数据模型与流水线

- [x] 3.1 `TrackedItem` 增 `type string`；`categoryPath` 改 `[]string`；openapi 更新；
      repo 默认值改数组（build 绿；存量 string→array 迁移属 6.3）
- [x] 3.2 categorizer 前置 type 层（`facets.ClassifyType` 命名规则+优先级，不进 LLM）
- [x] 3.3 embedding 分类层删除（含死代码 service/embed.go）；LLM 支持多 path + 同响应返回 type；
      `DOMAIN_MAX_LABELS`（config，默认 3）；provider 适配 grok（baseURLKey）
- [x] 3.4 资料类（awesome/interview/tutorial/skill）领域落空 → markDone 带 type 不失败
- [x] 3.5 no-topic 富化便车版：LLM 响应加 `tags` → `sourceData.generatedTopics`（与作者分开）
      —— 注：readme 塞入 prompt 尚未接（信号薄时增强），留实现细节 TODO
- [x] 3.6 [验证] categorizer 集成测试全绿（本地 mongo:7 容器，隔离；rule/多标签/type/资料类不失败/
      失败升级/repo 层数组全通过）；.env.example 补 `DOMAIN_MAX_LABELS` + grok 后端说明，EMBED_* 标记留给未来语义搜索

## 4. 查询与导航接口 —— 全部完成并连本地 mongo 实测

- [x] 4.1 `/trending`+`/rising` 增 `type` 参数；`type`+`categoryPath` 建索引；openapi 更新
- [x] 4.2 返回体含 `type`（模型字段，实测 k9s 返回 type=cli + 多标签 categoryPath）
- [x] 4.3 `GET /category`（转公开）：createdBy=taxonomy 过滤 + count 聚合（叶子 unwind、
      父类按顶层段 distinct 去重，实测 web=2 不重复、k9s 跨双父类各计一次）
- [x] 4.4 `GET /category/facets`（公开）：type 枚举按优先级 + count，实测正确
- [x] 4.5 `category` 参数支持 path（含 `/` 查 categoryPath），实测 category=infra/containers 命中
      —— 顺带：category 读接口从 admin 拆为公开（前端导航要用），增改留 admin；路由无冲突（已实测启动）

## 5. 前端导航收敛（消灭双分类系统）

- [x] 5.1 导航树消费 `GET /category`（getCategoryTree）；删硬编码 TAXONOMY/taxonomyTopics；
      消灭 22 请求 N+1（count 现由树接口一次带回）
- [x] 5.2 列表查询改 `category=<path>&type=<key>`（URL 参数 cat/sub → category/type）；
      后端 categoryFilter 支持 hex-id/叶子精确/父类前缀三态
- [x] 5.3 i18n：后端 taxonomy.yaml 加 nameEn（57 节点）；前端 categoryLabel(zh→name, 其余→nameEn)
- [x] 5.4 无需做——分类是 URL 查询参数、不在 sitemap，无 SEO/slug 迁移负担
- [x] 5.5 type chip 快切（FilterBar 按 facets 渲染 chip）；默认榜单全显示（产品决策）
- 验证：tsc --noEmit 清、next build 成功（29/29 页）、后端 live 实测（nameEn/父类前缀筛选）
- 遗留：messages/*.json 的 rank.cats/rank.subs 已成 dead key（旧树结构，不再引用），可后续清理

## 6. 验证与迁移 —— 代码就位，生产执行待维护者（见 migration-runbook.md）

- [x] 6.1/6.2 单测+集成全绿（categorizer_test：type 优先级、多标签 top-K、资料类落空不升级、
      父类丢弃、失败升级、categoryPath 多值；本地 mongo 容器实测）
- [x] 坑1 兼容解码 `PathList`：新二进制读旧 string categoryPath 不再 500（实测混合新旧格式 200）
- [x] 坑2 `generatedTopics` 提顶层字段（fetcher 替换 sourceData 不再冲掉）
- [x] 6.4 代码：Sync 启动删除不在 taxonomy.yaml 的旧分类（learning/*、*-framework）——
      实测 `/category` 不含 learning、旧分类文档被清
- [x] 6.3 工具：`POST /internal/reset-analysis?limit=N` 重置 done/failed→pending 供分批重跑——实测
- [ ] 6.3 生产执行：部署新二进制 + 改 .env（`CATEGORIZE_BATCH_SIZE=5`+grok）+ 分批重跑 67k
      **（不可逆，待维护者按 runbook 操作）**
- [ ] 6.4 生产校验：learning 归零、string categoryPath 归零、eval 线上对比 precision
