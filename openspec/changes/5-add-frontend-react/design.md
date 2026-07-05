# Design: 5-add-frontend-react

## Context

多源数据密集型内容产品，获客高度依赖搜索引擎与 AI 生成引擎的自然流量。因此可索引性（SSR/SSG）、
国际化、结构化数据是一等需求，而非附加项。前端独立工程，通过 change 1 冻结的 REST API 取数。

## Goals / Non-Goals

- Goals：专业设计系统；多源核心页面；SEO + GEO + i18n 三项从零建设；SSR/SSG 可索引；类型安全 API；明暗主题；AA 可访问性。
- Non-Goals：地理定位广告/风控；移动原生 App；超出中/英的语言（首批二语，架构预留扩展）。

## Decisions

### 1. 框架：Next.js（App Router）

- SEO/GEO 要求爬虫与 AI 引擎能拿到完整 HTML → 必须 SSR/SSG，排除纯 Vite SPA。
- 内容页（条目详情、分类、排行）用 SSG/ISR（增量静态再生）兼顾新鲜度与可缓存；
  交互过滤用客户端 TanStack Query 叠加。
- 目录：`app/[locale]/(routes)`；`app/sitemap.ts`、`app/robots.ts`、`app/[locale]/.../opengraph-image`。

### 2. i18n

- next-intl（或 next 内建 i18n routing）：locale 前缀路由 `/[locale]/...`，默认 en，支持 zh。
- 翻译资源按命名空间拆分；SEO 元数据（title/description）随 locale 本地化。
- 每页输出 `hreflang` 备用链接（各 locale + x-default），canonical 指向当前 locale 版本。

### 3. SEO

- Next Metadata API 逐页生成 title/description/OpenGraph/Twitter。
- `sitemap.xml` 动态生成：枚举条目详情、分类、排行页 × 各 locale；分片应对大规模 URL。
- `robots.txt`；语义化标签与标题层级；图片 alt；Core Web Vitals（LCP/CLS/INP）预算。
- 规范 URL 稳定（条目用 `source/externalId` 可读化 slug）。

### 4. GEO（Generative Engine Optimization）

- schema.org JSON-LD：条目详情用 SoftwareApplication/ItemList，列表用 ItemList，导航用 BreadcrumbList。
- `llms.txt` 描述站点结构与可引用数据入口。
- 事实性内容块（如"该项目近 30 天 star 增长 X"）以清晰句式呈现，便于 AI 摘录并带来源引用。
- 稳定、语义化、可直达的 URL，降低 AI 引用门槛。

### 5. 设计系统与多源可视化

- 设计令牌（color/space/radius/shadow/typography）+ 明暗主题（data-theme）。
- 源标识（source badge/icon）+ 分类色板，全站一致；源为一等过滤/切换维度。
- 图表遵循 dataviz：条目详情的主指标历史曲线（依赖 change 2），增长趋势条，分类分布；
  明暗适配、色盲安全、轴/图例/tooltip 规范、非仅靠颜色区分。

### 6. API 层

- 由后端 openapi.json 生成 TS 类型与 client；SSR 端直接调用，客户端交互用 TanStack Query。
- 加载/空/错误三态组件化统一。

## Risks / Trade-offs

- SSR/SSG 提升运维复杂度（Node 运行时或静态托管 + ISR）——但 SEO/GEO 收益是产品获客命脉，值得。
- 依赖 change 1 冻结 API 契约；条目详情曲线依赖 change 2 快照积累，无数据时优雅降级。
- i18n 使页面 × locale 倍增 URL，sitemap 需分片；翻译维护成本随语言数上升，首批限中/英。
- 大规模 SSG 构建时间：用 ISR/按需生成而非全量预构建。
