# ai-categorization 变更

## MODIFIED Requirements

### Requirement: 定时 AI 分类任务

系统 SHALL 每日定时查询 analysisStatus=pending 的仓库，按可配置批大小（默认 10–20）分批调用 AI 分析；成功者写回 categoryId/categoryPath 并置 analysisStatus=done，失败者 analysisFailCount+1，累计失败 3 次 SHALL 置 failed 并退出每日队列。

#### Scenario: 批量执行

- **WHEN** cron 触发且存在 100 个 pending 仓库、批大小 20
- **THEN** 系统发起 5 次 AI 调用完成全部分析

#### Scenario: 失败仓库不再无限重试

- **WHEN** 某仓库连续 3 天分析失败
- **THEN** 其 analysisStatus 为 failed，次日不再进入队列

### Requirement: 分类 prompt 构建

系统 SHALL 将现有分类树（批次开始时构建并缓存，批内新建分类后增量更新）连同一批仓库的 name/description/language/topics 组装进 prompt（不含 README），要求 AI 为每个仓库返回含 repoNameID 锚点的 JSON 数组元素。

#### Scenario: 批量 prompt

- **WHEN** 对 15 个仓库发起一批分析
- **THEN** prompt 含分类树与 15 个仓库的元信息，要求返回 15 元素数组

### Requirement: AI 响应解析

系统 SHALL 优先使用 Provider 的结构化输出能力（json_schema / json mode）获取 JSON，降级为直接 `JSON.parse`；按 repoNameID 锚点将数组元素与仓库对齐，缺失或非法的元素 SHALL 仅使对应仓库标记失败，不影响同批其他仓库。

#### Scenario: 批内个别元素错位

- **WHEN** AI 返回的数组缺少某个 repoNameID 的元素
- **THEN** 该仓库计一次失败，其余仓库正常写回

### Requirement: 新分类自动创建

当 AI 判定 isNewCategory 时，系统 SHALL 先按 path 查询已有分类，存在则直接复用其 id；不存在才创建新分类（createdBy=ai，根分类 parentId 为 null），并同步更新批次内的分类树缓存。

#### Scenario: 重复建议同一新分类

- **WHEN** 同批内两个仓库都被建议归入尚不存在的 `ai/agents`
- **THEN** 该分类只被创建一次，两个仓库复用同一 id

## ADDED Requirements

### Requirement: 分析状态管理

GithubTrend SHALL 含 analysisStatus（pending/done/failed，默认 pending，有索引）与 analysisFailCount 字段；系统 SHALL 提供批量重置 failed 为 pending 的维护手段（脚本或内部接口），供模型升级后重跑。

#### Scenario: 重置后重跑

- **WHEN** 运维将 failed 仓库批量重置为 pending
- **THEN** 这些仓库重新进入下一次分类队列，analysisFailCount 归零
