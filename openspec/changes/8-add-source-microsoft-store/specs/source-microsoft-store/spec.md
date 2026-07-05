# source-microsoft-store 变更

## ADDED Requirements

### Requirement: Microsoft Store 数据抓取

系统 SHALL 通过可配置的数据通道（官方后端接口 / 第三方 / 受控自抓取）获取 Microsoft Store 产品的
Product Store ID、name、rating、ratingCount、category 及可得的榜位，归一化为 TrackedItem
（source=msstore、externalId=Product Store ID、primaryMetric 视通道、metrics/sourceData）。
通道 SHALL 抽象为可切换接口，核心归一化逻辑共用。

#### Scenario: 抓取产品条目

- **WHEN** 适配器运行且通道可用
- **THEN** 拉取产品数据并写入/更新对应 TrackedItem

#### Scenario: 通道失效降级

- **WHEN** 数据通道不可用（接口变更或页面改版）
- **THEN** 记录错误并跳过本次，不影响其他源与已入库数据

### Requirement: 统一热度口径

系统 SHALL 定义平台内一致的热度/趋势口径：有官方榜位时用榜位（配合 change 6 的指标方向），
否则以 rating/评价增长（change 2）为口径。

#### Scenario: 榜位或增长排行

- **WHEN** 某产品榜位上升或评价数显著增长
- **THEN** 其出现在增长排行（source=msstore）

### Requirement: 合规约束

系统 SHALL 在实施前完成数据通道的合规评估，自抓取时 SHALL 尊重 robots、限速并仅取公开展示数据。

#### Scenario: 抓取节流

- **WHEN** 走自抓取通道
- **THEN** 按限速与缓存策略请求，遵守 robots，失败退避
