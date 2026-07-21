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

- [ ] 3.1 `TrackedItem` 增 `type string`
- [ ] 3.2 categorizer 前置 type 层（确定性，topics 空则兜底 software，不进 LLM）
- [ ] 3.3 领域 ② embedding 改收所有过阈叶子、③ LLM prompt/解析支持多 path；
      上限 `DOMAIN_MAX_LABELS`（config，默认 3）；.env.example 补充
- [ ] 3.4 资料类（type ∈ awesome/interview/tutorial）领域落空不计失败升级

## 4. 查询与前端

- [ ] 4.1 `/trending` 增 `type`（精确）、`topic`（含匹配）参数；`type` 与 `sourceData.topicNames`
      建索引；openapi.yaml 更新
- [ ] 4.2 查询/详情返回体补 `type`
- [ ] 4.3 前端：type chip 快切 + topic 筛选；默认榜单是否排除资料类在此定夺（产品决策）

## 5. 验证与迁移

- [ ] 5.1 单测：type 优先级/兜底、领域 top-K 与上限、资料类落空不升级
- [ ] 5.2 集成：mongo + fake embedder/LLM 跑通 type + 领域三级多标签路径
- [ ] 5.3 分批重跑存量（先小批 → eval 校准 → 全量），记录 eval 前后对比
- [ ] 5.4 确认 `learning/*` 历史文档未破坏、不再是归类目标
