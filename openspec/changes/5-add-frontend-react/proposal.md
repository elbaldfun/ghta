# Change: React 前端 + 专业 UI/产品设计（SEO/GEO/多语言）

## Why

项目目标是多源开发者/软件情报站，需要面向用户的 Web 界面把数据变成价值与流量入口。
作为内容型产品，**自然流量（搜索引擎 + AI 生成引擎）是核心获客渠道**，因此前端从一开始就要按
SEO（搜索引擎优化）、GEO（Generative Engine Optimization，被 AI 答案引擎引用）、多语言（i18n）三项标准建设，
而非事后补。这要求服务端渲染（SSR/SSG）而非纯 SPA。

> 术语约定：本 change 的 **GEO 指 Generative Engine Optimization**（让内容被 ChatGPT/Perplexity/AI Overviews 等引用），
> 通过结构化数据 + 语义清晰内容 + llms.txt 实现。地理维度（geographic）由 i18n 语言/地区 + 源的按国家榜单数据承载。

## What Changes

- **框架选型调整**：Vite SPA → **Next.js（App Router，SSR/SSG）**——为 SEO/GEO 提供可索引的服务端渲染 HTML。
- **设计系统**：设计令牌、明暗双主题、响应式、可访问性（WCAG AA），组件库（shadcn/ui）+ Tailwind；
  数据可视化遵循 dataviz 规范；源为一等过滤维度。
- **多语言（i18n）**：locale 路由（`/[locale]/...`）、翻译资源、语言切换、内容与 SEO 元数据本地化；首批中/英。
- **SEO**：每页 title/description/OG/Twitter 卡、canonical、`hreflang`（多语言/地区）、
  `sitemap.xml`（含所有条目/分类/排行页，多 locale）、`robots.txt`、语义化 HTML、Core Web Vitals 优化。
- **GEO**：schema.org JSON-LD 结构化数据（SoftwareApplication / ItemList / BreadcrumbList）、
  `llms.txt`、清晰事实性内容块（便于 AI 摘录与引用）、稳定的规范 URL。
- **多源信息架构与核心页面**（详情/分类/排行为主要 SEO 落地页）：总览、各源趋势榜、跨源增长排行、
  分类浏览、条目详情（指标曲线 + 结构化数据）、管理后台（需登录）。
- **API 对接层**：基于 change 1 冻结的 OpenAPI 生成类型化 client；SSR 数据获取 + 客户端 TanStack Query 混合。
- **BREAKING**: 无（纯新增前端工程）。

## Impact

- Affected specs: 新增 capability `web-frontend`
- Affected code: 新建 `web/**`（Next.js 工程，独立于 Go 后端）；后端需支持 CORS 及稳定的公开读接口供 SSR 取数
- 前置依赖：change 1（API 契约冻结）；排行页依赖 change 2；管理后台依赖 change 4；各源展示随 change 6/7/8 扩展
- 新依赖（前端）：next、react、next-intl（或等价 i18n）、@tanstack/react-query、图表库、shadcn/ui + tailwind
