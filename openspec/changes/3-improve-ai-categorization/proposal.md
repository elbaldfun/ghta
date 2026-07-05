# Change: AI 分类改造（结构化输出 / 批量 / 失败标记）

## Why

当前 AI 分类逐仓库调用（1000 次/天）、靠正则从自由文本抠 JSON（脆弱）、失败仓库无标记导致每天无限重试同一批坏数据。分析吞吐与稳定性都不足以消化抓取量（数十万仓库），成本（token/时间）也高出必要水平一个数量级。

## What Changes

- **结构化输出**: Provider 接口升级为 `analyzeStructured(prompt, schema)`，OpenAI 走 `response_format: json_schema`；LM Studio 走 json mode + 解析回退。彻底移除围栏正则。
- **批量分类**: 一次 prompt 携带 10–20 个仓库（name/description/language/topics，不含 README），AI 返回数组结果，单次调用成本摊薄。
- **失败标记**: GithubTrend 新增 `analysisStatus`(pending/done/failed) 与 `analysisFailCount`；失败 3 次的仓库退出每日队列，可手动重置。
- **分类树缓存**: 分类树在批次开始时构建一次并缓存，批次内新建分类后增量更新，不再每仓库全量查询。
- **新分类防重**: 创建新分类前按 path 查重，已存在则直接复用，避免 AI 并发建议重复分类。

## Impact

- Affected specs: `ai-categorization`（全部 requirement 修改/新增）
- Affected code (Go): `internal/service/ai`、`internal/provider`（IAiProvider 接口 + openai/deepseek 实现）、`internal/domain`（GithubTrend 增 analysisStatus/analysisFailCount）
- 在 Go 代码基（change 1 重写）上进行；change 1 已内建正确的 categoryPath 落库与未分类查询
