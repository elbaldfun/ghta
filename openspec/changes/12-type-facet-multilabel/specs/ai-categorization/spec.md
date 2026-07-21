# ai-categorization 变更

## MODIFIED Requirements

### Requirement: 定时分类任务

系统 SHALL 对 pending 条目产出**领域主类（可多标签）+ type 形态（单值）**。type SHALL 由受控
规则表（facets.yaml）按优先级确定性判定，不调用 embedding/LLM，全部未命中时落兜底值。
领域主类 SHALL 走三级流水线（规则/embedding/LLM，按成本递增、前级命中即停），embedding 与
LLM 层 SHALL 支持多标签（不超过 `DOMAIN_MAX_LABELS`）。条目 SHALL 记录 classifiedBy
（rule/embedding/llm，针对领域主类）。

#### Scenario: type 确定性判定

- **WHEN** 条目 topics 含 `awesome` 与 `cli`
- **THEN** type 按优先级取 `awesome`，判定不发生 embedding/LLM 调用

#### Scenario: type 兜底

- **WHEN** 条目 topics 为空或全部未命中 type 线索
- **THEN** type 取兜底值，不进入 LLM

#### Scenario: 领域多标签

- **WHEN** embedding 层有多个叶子相似度超过阈值
- **THEN** 领域主类收录所有过阈叶子（至多 `DOMAIN_MAX_LABELS` 个），而非仅 top1

#### Scenario: 规则命中不调用模型

- **WHEN** 条目 topics 命中领域对照表
- **THEN** 领域直接归类，classifiedBy=rule，不发生 embedding/LLM 调用

#### Scenario: 逐级回退

- **WHEN** 领域未命中规则且相似度低于阈值
- **THEN** 进入 LLM 批量兜底

#### Scenario: 资料类领域落空不升级

- **WHEN** type 为资料类（awesome/interview/tutorial）且三级均未能给出领域
- **THEN** 条目保留 type、领域为空，不计入分析失败升级

#### Scenario: embedding 层不可用

- **WHEN** 未配置 embedding 后端
- **THEN** 领域流水线退化为规则+LLM，type 层不受影响，正常完成

### Requirement: 新分类自动创建

系统 SHALL NOT 由 AI 自动创建分类。领域树与 type 枚举 SHALL 以 git 中的 taxonomy 资产
（`taxonomy.yaml` + `facets.yaml`）为唯一事实源，启动时同步。LLM 判定无合适领域时 SHALL 将
建议写入建议队列（按 path 去重、累计出现次数）并对该条目计一次分析失败；新增分类/枚举的
唯一途径是人工修改资产。

#### Scenario: AI 建议新分类

- **WHEN** LLM 返回 isNewCategory=true 且 path 为 `data/timeseries`
- **THEN** 建议队列该 path 计数 +1，分类集合不变，条目分析失败计数 +1
