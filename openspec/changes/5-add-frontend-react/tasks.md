# Tasks: 5-add-frontend-react

## 1. 工程与设计系统

- [ ] 1.1 初始化 `web/`：Next.js（App Router）+ TS strict + Tailwind + shadcn/ui
- [ ] 1.2 设计令牌：color/space/radius/shadow/typography CSS 变量；明暗双主题（data-theme）
- [ ] 1.3 源标识（badge/icon）+ 分类色板，全站图表/标签复用
- [ ] 1.4 基础组件：条目卡片、指标徽章、增长 badge、源/分类 chip、数据表、骨架/空/错误态
- [ ] 1.5 可访问性基线：语义标签、键盘可达、对比度 AA、图表非仅靠颜色区分

## 2. i18n 多语言

- [ ] 2.1 next-intl（或内建）locale 路由 `/[locale]/...`，默认 en + zh，语言切换器
- [ ] 2.2 翻译资源按命名空间拆分；UI 文案全部走 i18n
- [ ] 2.3 SEO 元数据本地化 + 每页 hreflang（各 locale + x-default）+ canonical

## 3. SEO

- [ ] 3.1 Next Metadata API：逐页 title/description/OpenGraph/Twitter 卡
- [ ] 3.2 动态 `sitemap.xml`（条目/分类/排行 × 各 locale，分片）+ `robots.txt`
- [ ] 3.3 语义化 HTML、标题层级、图片 alt；条目可读化 slug + 稳定 canonical URL
- [ ] 3.4 Core Web Vitals 预算（LCP/CLS/INP），图片优化、字体优化

## 4. GEO（生成引擎优化）

- [ ] 4.1 schema.org JSON-LD：详情 SoftwareApplication、列表 ItemList、导航 BreadcrumbList
- [ ] 4.2 `llms.txt` 站点结构与可引用数据入口
- [ ] 4.3 事实性内容块（增长/排名事实以清晰句式呈现，便于 AI 摘录引用）

## 5. API 层

- [ ] 5.1 由 openapi.json 生成类型化 client；SSR 直调 + 客户端 TanStack Query
- [ ] 5.2 三态（加载/空/错误）统一组件

## 6. 多源核心页面

- [ ] 6.1 总览首页：各源趋势榜 + 快捷入口
- [ ] 6.2 趋势列表（源过滤/指标区间/分类/排序/分页）
- [ ] 6.3 跨源增长排行（日/周/月 tab，source 过滤；依赖 change 2）
- [ ] 6.4 分类浏览（分类树 + 该分类跨源条目）
- [ ] 6.5 条目详情（元信息 + 主指标历史曲线 + 结构化数据；SSG/ISR）
- [ ] 6.6 管理后台（登录后，依赖 change 4）

## 7. 可视化与集成

- [ ] 7.1 主指标历史曲线（依赖 change 2；无数据降级）、增长条、分类分布，遵循 dataviz
- [ ] 7.2 后端 CORS/部署对接（Node 运行时或静态托管 + ISR）
- [ ] 7.3 前端 CI：typecheck + lint + build + 组件测试（Vitest/Testing Library）
- [ ] 7.4 E2E（Playwright）：列表过滤、排行切换、详情曲线、语言切换、SEO 元数据/JSON-LD 断言
- [ ] 7.5 上线后接入 Google Search Console / 结构化数据校验
