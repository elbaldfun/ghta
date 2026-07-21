# Tasks: 12-facet-taxonomy

## 1. 评估先行（先建再动树）

- [ ] 1.1 `taxonomy/golden.yaml`：人工标注 200+ 条目，每条含 `domain[]` / `type` / `tech`
- [ ] 1.2 `cmd/eval`：跑现网存量，输出领域各级命中率/准确率、facet 准确率、多标签 P/R
- [ ] 1.3 记录当前基线数值，作为调树前后对比锚点

## 2. 分类资产重构

- [ ] 2.1 `taxonomy/taxonomy.yaml` 重构为领域主树：移出 `lang`/`learning`，重排失衡枝
      （`ai` 拆 media-gen、`data` 并 cache 增 analytics、补 devtools/ai-coding 等）—— 人审
- [ ] 2.2 新增 `taxonomy/facets.yaml`：`type` 枚举 + 优先级线索、`tech` 语言/框架映射 —— 人审
- [ ] 2.3 `topic-map.yaml` 拆为领域段与 facet 段（或迁入 facets.yaml），去掉已下放的 lang/learning 映射
- [ ] 2.4 `internal/taxonomy`：加载 facets.yaml；type/tech 规则映射（`ClassifyFacets`）

## 3. 数据模型与流水线

- [ ] 3.1 `TrackedItem` 增 `type string`、`tech []string`；`categoryPath` 语义收窄为领域主类
- [ ] 3.2 categorizer 前置 facet 层（确定性，先于领域三级）
- [ ] 3.3 领域 ② embedding、③ LLM 由 top1 改 top-K 多标签，受 `DOMAIN_MAX_LABELS`（默认 3）约束
- [ ] 3.4 embedding 向量持久化：仓库向量写 `tracked_items.embedding`（或旁路集合），②复用
- [ ] 3.5 config 增 `DOMAIN_MAX_LABELS`；.env.example 补充

## 4. 查询与前端

- [ ] 4.1 `/trending` 增 `type`（精确）、`tech`（含匹配）筛选参数；建索引；openapi.yaml 更新
- [ ] 4.2 查询/详情返回体补 `type`/`tech`
- [ ] 4.3 前端筛选：领域树下钻 + `type`/`tech` 多选 chip；排行榜主轴仍为 domain

## 5. 验证与迁移

- [ ] 5.1 单测：facet 优先级映射、tech 多值去重、领域 top-K 多标签、上限约束、向量落库/复用
- [ ] 5.2 集成：mongo + fake embedder/LLM 跑通 facet + 领域三级 + 多标签路径
- [ ] 5.3 分批重跑存量分类，用 eval 校准后再全量；记录 eval 前后对比
- [ ] 5.4 确认老 `lang/*`、`learning/*` 分类不再作为归类目标、历史文档未被破坏
