# source-appstore 变更

## ADDED Requirements

### Requirement: App Store 榜单抓取

系统 SHALL 通过 Apple 官方 iTunes RSS 榜单 feed 按 (country, chartType, genre) 抓取 Top N 应用及其榜位，
并用 iTunes Lookup/Search API 补全应用元信息，归一化为 TrackedItem（source=appstore、externalId=trackId、
primaryMetric=rankPosition、metrics 含 rankPosition/rating/ratingCount，源专属信息入 sourceData）。

#### Scenario: 抓取某国某品类榜

- **WHEN** 配置 country=us、chartType=top-free、genre=某品类
- **THEN** 系统拉取该榜有序应用，写入/更新对应 TrackedItem 并记录榜位

#### Scenario: 同应用多榜出现

- **WHEN** 同一 trackId 出现在多个榜单
- **THEN** 作为同一 TrackedItem，各榜位记录于 sourceData.charts，不重复建条目

### Requirement: 榜位指标方向

系统 SHALL 支持"越小越好"的主指标方向（rankPosition），使增长/上升计算正确（榜位减小视为上升）。

#### Scenario: 榜位上升

- **WHEN** 某应用榜位从第 20 升到第 5
- **THEN** 增量计算视其为上升，可出现在增长排行

### Requirement: 合规与节流

系统 SHALL 仅使用 Apple 官方公开 feed/API，遵守其使用条款并自我节流（请求间隔 + 重试退避），SHALL NOT 抓取需鉴权或禁止抓取的页面。

#### Scenario: 请求节流

- **WHEN** 批量补全元信息
- **THEN** 按批（Lookup 单次上限）+ 间隔请求，失败指数退避重试
