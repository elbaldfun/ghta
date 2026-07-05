# source-chrome-webstore 变更

## ADDED Requirements

### Requirement: Chrome 扩展数据抓取

系统 SHALL 通过可配置的数据通道（第三方数据 API 或自抓取）获取 Chrome Web Store 扩展的
id、name、users、rating、ratingCount、category、开发者等，归一化为 TrackedItem
（source=chrome、externalId=扩展 ID、primaryMetric=users、metrics 含 users/rating/ratingCount，源专属信息入 sourceData）。
通道 SHALL 抽象为可切换接口，核心归一化逻辑共用。

#### Scenario: 抓取扩展条目

- **WHEN** 适配器运行且通道可用
- **THEN** 拉取扩展数据并写入/更新对应 TrackedItem

#### Scenario: 通道失效降级

- **WHEN** 数据通道不可用（第三方故障或页面改版）
- **THEN** 记录错误并跳过本次，不影响其他源与已入库数据

### Requirement: 无官方榜单的趋势口径

由于 Chrome Web Store 无官方排行，系统 SHALL 以 users 增长（change 2 指标计算）作为热度/趋势口径，
而非依赖现成榜单。

#### Scenario: 用户数增长排行

- **WHEN** 某扩展一周内 users 显著增长
- **THEN** 其出现在增长排行（source=chrome）

### Requirement: 合规约束

系统 SHALL 在实施前完成数据通道的合规评估（第三方授权或自抓取 ToS/法律边界），自抓取时 SHALL 尊重 robots、
限速并仅取公开展示数据。

#### Scenario: 抓取节流

- **WHEN** 走自抓取通道
- **THEN** 按限速与缓存策略请求，遵守 robots，失败退避
