# Change: 分类流水线（固定分类树 + 规则/相似度/LLM 三级）

## Why

现有 AI 分类（change 3）有三个结构性弱点：分类树由 AI 自由生长会语义漂移（`ai/llm` vs
`machine-learning/llm` 并存）；结果不可复现、无法评估准确率；且每个仓库都走 LLM，成本结构不对——
大多数仓库的 topics（作者自标）已直接给出答案。

## What Changes

- **固定分类树**：`taxonomy/taxonomy.yaml` 进 git 作为受控资产（两级，~100 叶子，每叶一句说明），
  启动时同步进 categories 集合。**AI 不再能创建分类**，只能向 `category_suggestions` 集合提建议，
  由人审后改 YAML。
- **三级分类流水线**（按成本递增，前一级命中即停）：
  1. **规则映射**（免费、确定性）：`taxonomy/topic-map.yaml` 把 GitHub topics/语言映射到分类节点；
  2. **embedding 相似度**（便宜、可复现）：仓库 `name+description+topics` 与各叶子说明文本的余弦相似度，
     超阈值即归类；无 API key 时自动跳过此层；
  3. **LLM 批量兜底**（复用 change 3 管道）：前两级未命中的长尾；`isNewCategory` 改为写建议 + 计一次失败。
- 条目记录 `classifiedBy`(rule/embedding/llm) 便于评估各级占比与质量。
- **BREAKING**: AI 自动建分类的行为被移除（改为建议队列）。

## Impact

- Affected specs: `ai-categorization`（新分类自动创建 → 建议队列；任务改为流水线）
- Affected code (Go): 新增 `taxonomy/`（YAML 资产）、`internal/taxonomy`（加载/同步/规则映射）、
  provider 增 Embed；`internal/service/ai.go`（去掉 ensureCategory 自动建）、`internal/job/categorizer.go`（流水线）
- 存量 AI 建的分类保留但不再作为归类目标；重跑分类即可迁移到新树
