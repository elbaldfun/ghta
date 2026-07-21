# ai-categorization 变更

## MODIFIED Requirements

### Requirement: 定时分类任务

系统 SHALL 对 pending 条目产出 **一个领域主类（可多标签）+ 正交 facet**。facet 层（`type`、`tech`）
SHALL 用确定性规则表映射，不调用 embedding/LLM。领域主类 SHALL 走三级流水线（规则/embedding/LLM，
按成本递增、前级命中即停），其中 embedding 与 LLM SHALL 返回 top-K 多标签（不超过 `DOMAIN_MAX_LABELS`）。
条目 SHALL 记录 classifiedBy（rule/embedding/llm，针对领域主类）。

#### Scenario: facet 确定性映射

- **WHEN** 条目 topics 含 `cli`、主语言为 Rust
- **THEN** `type=cli`、`tech` 含 `rust`，不发生 embedding/LLM 调用

#### Scenario: 领域多标签

- **WHEN** 某条目在 embedding 层有多个叶子相似度超过阈值
- **THEN** 领域主类收录所有过阈叶子（至多 `DOMAIN_MAX_LABELS` 个），而非仅 top1

#### Scenario: 规则命中不调用模型

- **WHEN** 条目 topics 命中领域段对照表
- **THEN** 领域主类直接归类，classifiedBy=rule，不发生 embedding/LLM 调用

#### Scenario: 逐级回退

- **WHEN** 领域未命中规则且相似度低于阈值
- **THEN** 进入 LLM 批量兜底

#### Scenario: embedding 层不可用

- **WHEN** 未配置 embedding 后端
- **THEN** 领域流水线退化为规则+LLM，facet 层不受影响，正常完成

### Requirement: embedding 向量持久化

系统 SHALL 将条目 embedding 向量持久化，供 embedding 分类层复用，并为相似仓库推荐与语义搜索提供基础。
叶子分类向量 MAY 仅内存缓存。

#### Scenario: 向量复用

- **WHEN** 条目已存在持久化 embedding 向量且内容未变
- **THEN** 分类任务复用该向量，不重复请求 embedding 后端

### Requirement: 新分类自动创建

系统 SHALL NOT 由 AI 自动创建分类。领域树与 facet 枚举 SHALL 以 git 中的 taxonomy 资产
（`taxonomy.yaml` + `facets.yaml`）为唯一事实源，启动时同步。LLM 判定无合适领域时 SHALL 将建议
写入建议队列（按 path 去重计数）并对该条目计一次分析失败。

#### Scenario: AI 建议新分类

- **WHEN** LLM 返回 isNewCategory=true 且 path 为 `data/timeseries`
- **THEN** 建议队列该 path 计数 +1，分类集合不变，条目分析失败计数 +1
