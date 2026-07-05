# ai-categorization 变更

## MODIFIED Requirements

### Requirement: 定时 AI 分类任务

系统 SHALL 以三级流水线对 pending 条目分类，按成本递增、前级命中即停：
(1) 规则映射——用受控的 topic/language 对照表直接归类；(2) embedding 相似度——条目文本与分类说明
文本的余弦相似度超过配置阈值时归类，无 embedding 后端时该层跳过；(3) LLM 批量兜底（沿用批量、
锚点、失败升级语义）。条目 SHALL 记录 classifiedBy（rule/embedding/llm）。

#### Scenario: 规则命中不调用模型

- **WHEN** 条目的 topics 命中对照表
- **THEN** 直接归类，classifiedBy=rule，不发生 embedding/LLM 调用

#### Scenario: 逐级回退

- **WHEN** 条目未命中规则且相似度低于阈值
- **THEN** 进入 LLM 批量兜底

#### Scenario: embedding 层不可用

- **WHEN** 未配置 embedding 后端
- **THEN** 流水线退化为规则+LLM 两级，正常完成

### Requirement: 新分类自动创建

系统 SHALL NOT 由 AI 自动创建分类。分类树 SHALL 以 git 中的 taxonomy 资产为唯一事实源，
启动时同步进分类集合。当 LLM 判定无合适分类时，系统 SHALL 将建议写入建议队列（按 path 去重、
累计出现次数）并对该条目计一次分析失败；新增分类的唯一途径是人工修改 taxonomy 资产。

#### Scenario: AI 建议新分类

- **WHEN** LLM 返回 isNewCategory=true 且 path 为 `blockchain/defi`
- **THEN** 建议队列中该 path 的计数 +1，分类集合不变，条目分析失败计数 +1

#### Scenario: 人工采纳建议

- **WHEN** 维护者把建议的分类加入 taxonomy 资产并重启/重跑
- **THEN** 新分类出现在分类树中，重跑分类的条目可归入该分类
