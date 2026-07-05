# app-configuration 变更

## ADDED Requirements

### Requirement: 启动配置校验

系统 SHALL 在启动时校验环境变量：MONGODB_URI 与 GITHUB_API_TOKEN 必填，AI_PROVIDER 限于 openai/deepseek，PORT 为数字。任一校验失败 SHALL 使进程启动失败并在错误信息中指明缺失/非法的字段名。配置默认值 SHALL NOT 包含任何环境特定地址。

#### Scenario: 缺少必填配置

- **WHEN** 启动时未设置 GITHUB_API_TOKEN
- **THEN** 进程启动失败，错误信息包含 "GITHUB_API_TOKEN"

#### Scenario: 非法枚举值

- **WHEN** AI_PROVIDER=gemini
- **THEN** 进程启动失败并提示合法取值

### Requirement: 敏感信息日志约束

系统 SHALL NOT 将环境变量全量、API token、数据库连接串等敏感信息写入日志。

#### Scenario: 启动日志

- **WHEN** 应用正常启动
- **THEN** 日志中不出现 GITHUB_API_TOKEN 等密钥值
