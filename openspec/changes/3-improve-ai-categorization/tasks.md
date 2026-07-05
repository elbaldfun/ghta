# Tasks: 3-improve-ai-categorization

## 1. Provider 与解析

- [ ] 1.1 IAiProvider 增加 `analyzeStructured(prompt, jsonSchema)`；OpenAI provider 用 response_format
- [ ] 1.2 Deepseek/LM Studio provider 用 json mode，失败降级裸 JSON.parse
- [ ] 1.3 删除围栏正则解析路径

## 2. 批量分类

- [ ] 2.1 prompt 改为批量结构（10–20 仓库/批，批大小走配置），输出含 repoNameID 锚点的数组
- [ ] 2.2 ai.service 按批取数、按锚点回填，元素级失败标记
- [ ] 2.3 分类树批次内缓存 + 新建分类后增量更新

## 3. 失败标记

- [ ] 3.1 GithubTrend 新增 analysisStatus/analysisFailCount 字段与索引
- [ ] 3.2 队列查询条件改为 analysisStatus=pending
- [ ] 3.3 失败 3 次置 failed；提供重置脚本（docs 记录用法）

## 4. 新分类防重与验证

- [ ] 4.1 createNewCategory 前按 path 查重复用
- [ ] 4.2 单测：批量解析（正常/错位/缺元素）、失败计数状态机、path 查重
- [ ] 4.3 用 50 个真实仓库端到端跑一批，人工抽查分类合理性并记录准确率
