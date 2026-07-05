# Design: 4-improve-ai-categorization

## Context

数十万仓库需要分类；LLM 每仓库一次调用在成本、耗时、失败率上都不可持续。

## Decisions

### 1. 批量 prompt，每批 10–20 仓库

- 输入仅 name/description/language/topics（README 不进 prompt：token 爆炸且有 prompt 注入风险）。
- 输出为 JSON 数组，元素含 repoNameID 回填锚点；单元素解析失败只标记该仓库 failed，不废弃整批。
- 批大小可配置（LMSTUDIO 本地小模型建议 10，云端模型 20）。

### 2. 结构化输出优先，解析降级链

`json_schema`（OpenAI）→ json mode（LM Studio）→ 裸 JSON.parse → 失败标记。不再用正则围栏提取。

### 3. 失败与重试语义

- analysisStatus: pending(默认) / done / failed
- 每次失败 analysisFailCount+1；≥3 置 failed，退出每日队列
- 提供内部接口/脚本批量重置 failed（模型升级后重跑）

### 4. 长期方向（本期不做，记录备查）

预定义分类体系 + embedding 相似度归类，LLM 只兜底低置信度长尾。
成本可再降 10x，且分类稳定可复现。等分类树经 AI 冷启动收敛后实施。

## Risks / Trade-offs

- 批量模式下模型可能漏答/错位个别仓库 → 以 repoNameID 锚点校验，缺失者标记重试。
- 分类树缓存与并发新建分类存在竞态 → 分类任务保持单实例串行（cron 天然如此），path 查重兜底。
