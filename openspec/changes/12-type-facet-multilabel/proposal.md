# Change: type facet + 领域多标签（分类方案修订）

## Why

change 9 冻结了分类树并建立三级流水线，但沿用「单一分类树」的前提，遗留三个问题：

1. **形态维度混进领域树**：`learning`（教程/清单/面试）是内容形态，不是领域。一个 React 教程
   既是 `web` 又是 `learning`，被迫二选一；更要命的是教程/awesome 清单混在趋势榜里，star 增长
   最猛的常常是资料类仓库，把真正的软件项目挤出榜单——这是趋势类产品的核心痛点。
2. **多领域仓库被 top1 压碎**：规则层可多标签，但 embedding/LLM 层只取 top1。一个「Rust 写的
   K8s CLI」跨 devtools/infra，信息直接丢失。
3. **无评估基线**：change 9 的 golden set / eval（tasks 3.3）一直是 TODO，任何调树/调阈值都是盲调。

本 change 只针对 GitHub 单源场景，不考虑 App Store/Chrome Store 等后续源。

刻意**不做**的两件事（评审后砍掉的方案）：

- **不引入 tech facet**：按语言筛选已有 `language` 参数；按框架筛选可由已落库的原始 topics
  直接暴露（本 change 顺带开放 topic 筛选参数）。为「react/reactjs 归一化」这点增量新增一套
  资产+字段+映射，投入产出比不成立。
- **不做 embedding 向量持久化**：它服务未来的相似推荐/语义搜索，对本次分类质量无直接贡献，
  且 project.md 已定「搜索/分析走旁路、需要时再加」。留待独立 change。

## What Changes

- **新增 `type` facet（形态，单值）**：`taxonomy/facets.yaml` 受控枚举（tutorial/awesome/
  interview/cli/app/library/software 兜底等），由 topics/命名规则确定性判定，不走 embedding/LLM。
  `learning` 大类从领域树移除，其子类全部下放为 type 取值。
- **领域树微调**（`taxonomy.yaml`，人审）：删 `learning`；`lang` 语义收紧为「语言基础设施」
  （implementations + 通用工具库），topic-map 不再把普通库按语言路由到 `lang`；`ai` 拆
  `media-gen`、`data` 并入 `cache` 增 `analytics` 等粒度均衡调整。
- **领域层放开多标签**：embedding/LLM 由 top1 改 top-K（所有过阈/判定命中的都收），
  上限 `DOMAIN_MAX_LABELS`（默认 3）。
- **先建 eval 再动树**：补 golden set（含 domain[] + type 标注）+ `cmd/eval`，调整前后必须对比。
- **查询扩展**：`/trending` 增 `type`（精确）与 `topic`（含匹配）筛选；返回体增 `type`。
  前端筛选为「领域树 + type chip + topic」，排行榜主轴仍是领域。

## Impact

- Affected specs: `ai-categorization`（type facet 层 + 领域多标签）、`category-management`
  （领域树 + type 枚举资产）、`trending-query`（type/topic 筛选）
- Affected code (Go)：
  - `taxonomy/`：`taxonomy.yaml` 微调（删 learning、调 lang/ai/data）；新增 `facets.yaml`；
    `topic-map.yaml` 去掉 learning 映射与语言兜底中路由到 lang 的项
  - `internal/domain`：`TrackedItem` 增 `type string`
  - `internal/taxonomy`：加载 facets.yaml；`ClassifyType` 优先级映射
  - `internal/job/categorizer.go`：type 层前置；领域 ②③ top-K 多标签
  - `internal/handler` + `api/openapi.yaml`：`/trending` 增 `type`/`topic` 参数；返回体增 `type`
  - 新增 `cmd/eval` + `taxonomy/golden.yaml`
  - 前端：type chip / topic 筛选
- 迁移：`learning/*` 分类文档保留但不再作为归类目标；分批重跑分类收敛（沿用 change 9 做法）
- **BREAKING**：`learning/*` 不再是归类目标；查询返回体新增 `type` 字段
