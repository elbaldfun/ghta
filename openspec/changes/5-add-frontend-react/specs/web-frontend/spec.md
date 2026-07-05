# web-frontend 变更

## ADDED Requirements

### Requirement: 服务端渲染可索引性

前端 SHALL 采用 SSR/SSG（Next.js）使内容页对搜索引擎与 AI 生成引擎返回完整可索引 HTML；
内容页（条目详情、分类、排行）SHALL 以 SSG/ISR 生成，交互过滤在客户端叠加。

#### Scenario: 爬虫抓取条目详情

- **WHEN** 搜索引擎爬虫请求某条目详情页
- **THEN** 返回含完整内容与元数据的服务端渲染 HTML，无需执行 JS 即可读到主体内容

### Requirement: 多语言 i18n

前端 SHALL 支持 locale 前缀路由（首批 en/zh），UI 文案与 SEO 元数据随 locale 本地化，
提供语言切换，并为每页输出 hreflang 备用链接（各 locale + x-default）与指向当前 locale 的 canonical。

#### Scenario: 切换语言

- **WHEN** 用户从 en 切到 zh
- **THEN** 路由前缀与页面文案、元数据切换为中文版本，URL 反映 locale

#### Scenario: 多语言 hreflang

- **WHEN** 渲染任一内容页
- **THEN** 输出该页各 locale 的 hreflang 及 x-default，canonical 指向当前 locale

### Requirement: SEO 元数据与站点地图

前端 SHALL 为每页生成 title/description/OpenGraph/Twitter 元数据，SHALL 动态生成包含条目/分类/排行页
（× 各 locale）的 sitemap.xml 与 robots.txt，条目 URL SHALL 使用稳定可读 slug 与规范 canonical。

#### Scenario: 站点地图覆盖

- **WHEN** 请求 /sitemap.xml
- **THEN** 列出全部条目详情、分类、排行页的各 locale URL（大规模时分片）

### Requirement: GEO 结构化数据

前端 SHALL 输出 schema.org JSON-LD（详情用 SoftwareApplication、列表用 ItemList、导航用 BreadcrumbList），
SHALL 提供 llms.txt，SHALL 以清晰事实性句式呈现增长/排名事实，便于 AI 生成引擎摘录与引用。

#### Scenario: 详情页结构化数据

- **WHEN** 渲染条目详情页
- **THEN** 页面内嵌 SoftwareApplication JSON-LD 与面包屑 BreadcrumbList，可通过结构化数据校验

### Requirement: 设计系统与主题

前端 SHALL 建立设计令牌驱动的设计系统（color/space/radius/shadow/typography），支持明暗双主题并随用户/系统偏好切换，
满足 WCAG AA 对比度，图表 SHALL NOT 仅靠颜色区分信息（辅以形状/标签）。source 与分类 SHALL 有一致的视觉标识。

#### Scenario: 切换暗色主题

- **WHEN** 用户切换到暗色或系统为暗色偏好
- **THEN** 全站组件与图表使用暗色主题令牌，保持 AA 对比度

### Requirement: 多源趋势与排行页

前端 SHALL 提供趋势列表与增长排行页，支持 source 过滤/切换、分类过滤、指标区间过滤、按白名单字段排序、
分页与日/周/月窗口切换；SHALL 有加载、空、错误三态。

#### Scenario: 按源过滤趋势

- **WHEN** 用户选择 source=github 且分类=AI 并按主指标降序
- **THEN** 页面请求对应参数并渲染结果，切页保留上一页数据避免闪烁

#### Scenario: 跨源周增长榜

- **WHEN** 用户在增长排行选择"周"窗口且不限源
- **THEN** 页面请求跨源 rising 数据并以趋势条展示各源增量

### Requirement: 分类浏览与条目详情

前端 SHALL 提供分类树导航（选中后展示其下跨源条目）与条目详情页（元信息 + 主指标历史曲线 + releases/源专属信息 + README 等）；
指标历史缺失时 SHALL 优雅降级。

#### Scenario: 查看指标曲线

- **WHEN** 该条目有历史快照数据
- **THEN** 页面渲染主指标时间曲线（GitHub 为 star 曲线，标注 release）

#### Scenario: 无历史数据

- **WHEN** 条目尚无快照
- **THEN** 曲线区显示降级占位而非报错

### Requirement: 类型安全 API 层

前端 SHALL 基于后端 OpenAPI 生成类型化 API client，请求与响应类型与后端契约一致。

#### Scenario: 契约变更可见

- **WHEN** 后端 OpenAPI 字段变更且重新生成 client
- **THEN** 类型不匹配在编译期暴露，而非运行时静默出错
