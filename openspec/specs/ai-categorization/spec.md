# ai-categorization Specification

## Purpose

用 LLM 将已抓取的仓库自动归类到分类树：读取未分类仓库 → 构建含现有分类树的 prompt → 解析 AI 返回 → 写回分类信息，必要时自动创建新分类。

## Requirements

### Requirement: 定时 AI 分类任务

系统 SHALL 每日定时查询未分类仓库（categoryId 为空数组，limit 1000），逐个调用 AI 分析并将结果（categoryId、categoryPath）写回仓库文档。单仓库失败 SHALL 只记日志并继续下一个。

#### Scenario: 批次执行

- **WHEN** cron 触发分类任务
- **THEN** 系统取最多 1000 个未分类仓库，逐个完成 AI 分析与写回

#### Scenario: 单仓库分析失败

- **WHEN** 某仓库 AI 调用或解析失败
- **THEN** 记录错误日志，继续处理下一个仓库

### Requirement: AI Provider 抽象

系统 SHALL 通过 `IAiProvider` 接口（`analyze(prompt): Promise<string>`）抽象 AI 后端，按 `AI_PROVIDER` 环境变量在 OpenAiProvider（gpt-3.5-turbo）与 DeepseekProvider（LM Studio 本地服务，OpenAI 兼容接口，失败重试 3 次）之间切换，默认 OpenAI。

#### Scenario: 切换到本地模型

- **WHEN** AI_PROVIDER=deepseek
- **THEN** 请求发往 LMSTUDIO_BASE_URL（默认 http://localhost:1234/v1），使用 LMSTUDIO_LOCAL_MODULE_NAME 指定的模型

### Requirement: 分类 prompt 构建

系统 SHALL 将现有分类树渲染为缩进文本连同仓库的 name/description/language/topics 组装进 prompt，要求 AI 优先复用现有分类、无匹配时建议新分类，并以固定 JSON 结构返回（categoryId、path、isNewCategory、suggestedName）。

#### Scenario: 构建 prompt

- **WHEN** 对某仓库发起分析
- **THEN** prompt 包含完整分类树（含每个分类的 ID 与 path）及该仓库的元信息

### Requirement: AI 响应解析

系统 SHALL 从 AI 返回文本中提取 ```json 围栏内的 JSON 并解析为 ICategoryAnalysisResult，无法提取或解析失败 SHALL 抛错。

#### Scenario: 解析失败

- **WHEN** AI 返回中不含 ```json 围栏或 JSON 非法
- **THEN** 抛出 "Failed to parse AI response"（或 "No JSON content found"）错误

### Requirement: 新分类自动创建

当 AI 判定 isNewCategory 时，系统 SHALL 按返回的 path 拆解出 name/level/parentPath，查找父分类并创建新分类（createdBy=ai），然后将新分类 id 用于该仓库。

#### Scenario: AI 建议新的二级分类

- **WHEN** AI 返回 isNewCategory=true 且 path 为 `ai/agents`
- **THEN** 系统以 `ai` 为父分类创建 name=agents、level=2 的新分类，并将其 id 写回仓库
