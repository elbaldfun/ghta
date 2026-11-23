# 视觉改版线上走查（第 6 批）

2026-11-22～24，对线上 starrank.dev（v2026.11.22.1，`dfb7210`）做的走查。

## 范围和方法

- 22 条路由，每条看中文、英文两个版本，共 44 页。每页看 1440 和 375 两种宽度、浅色和深色两种主题，共 176 个视图。
  - 路由：首页、`/github`、`/github/c/agents`、`/github/c/agents/coding`、`/github/l/go`、`/github/t/app`、`/breakout`、`/developers`、`/ai`、`/models`、`/mcp`、`/ecosystem`、`/map`、`/topics`、`/apps`、`/apps/best/ai/local-llm`、`/alternatives`、`/alternatives/notion`、`/repo/codecrafters-io/build-your-own-x`、`/blog`、`/blog/stale-high-star-repos`、`/privacy`
- 工具：本机 Playwright 加 Chrome。375 宽按手机模拟（触屏、移动端 UA），深浅色用 `prefers-color-scheme` 模拟。每个视图做两件事：
  - 截一张整页截图；
  - 自动检查五项：横向溢出（`scrollWidth` 和超出视口的元素）；有没有原样露出的翻译键；英文页里有没有中文；旧令牌类名（`text-muted`、`border-border`、`font-mono`、`bg-surface2`、`text-up/down/accent`、`rounded-card` 等）；资源加载失败。
- 截图只人工看了已迁移的页面，旧样式页只看了一眼确认现状。

## 总体结果

- 176 个视图全部返回 200。1440 和 375 下都**没有横向滚动**，也没有元素超出视口。
- 没有发现露出的翻译键。自动检查报出的 `prompts.chat`、`call.md`、`codecrafters.io` 是项目名和域名，属于误报。
- `<html lang>` 和 URL 里的语言全部一致。

## 已修（本 PR）

| # | 页面 | 问题 | 处理 |
|---|---|---|---|
| 1 | 应用画廊、货架页 | 图标或截图在 React 接管页面之前就加载失败时，`onError` 接不到，页面上会一直留着坏图。线上 `/zh/apps/best/ai/local-llm` 第一张卡片 VoiceStudio 的图标每次都复现：它的图标地址是 `x.com/apple-touch-icon.png`，被 CORP 拦截 | `AppIcon`、`AppShot` 挂载后检查一次 `complete && naturalWidth === 0`，命中就走原来的回退（头像 → 字母块，截图 → 「无截图」）。本地修复版两次复测都正常回退 |
| 2 | 领域页、语言榜、类型榜、详情页 | 名次变化 `↑n` / `↓n` / 「新」还在用旧令牌（`text-up`、`text-down`、`text-accent`、`font-mono`）：涨是旧绿色，跌是旧红色，和新稿的 Delta 不一致 | 涨用 signal，跌用 ink-3（和 Delta 一样，下跌不报警），「新」用 signal-soft 底；字体换成 font-code |
| 3 | 应用画廊、货架页、详情页 | 应用图标的描边和底色、下载列表的文件大小还在用旧令牌 | 换成 rule / sunk / font-code |
| 4 | AI 专区「按子领域」 | 每一格都有 `pr-6`，所以三列里最右一列的数字离右边线差 24px，和上下其他区块对不齐 | 每行最后一格去掉右内边距（md 两列、lg 三列分别处理），1440 下复测已对齐 |
| 5 | 中文：5 个货架页介绍、加载失败提示 | 混用了半角逗号和冒号（「加权,每日更新」「属于自己:本地存储」） | 7 条全部改成全角标点 |
| 6 | 货架页首选（中文） | 平台列表拼成「有 macOS和Windows 安装包」，「和」两边缺空格。原因是 `Intl.ListFormat` 的中文格式不加空格 | 中文下在拉丁字母和汉字之间补空格，结果为「macOS、Windows 和 Linux」 |
| 7 | 货架页标题（英文、德文） | 模板是「Best Open Source {label} Apps」，拼出来是「Best Open Source Local LLMs Apps」「…Password Managers Apps」；德文拼成「Beste Open-Source-Lokale LLMs-Apps」 | 改成和西、法、葡一样的写法：「Best open-source apps: {label}」「Beste Open-Source-Apps: {label}」。这也是 5 个试点货架页的 `<title>`，如果试点期间不想动标题，回退这一条就行 |

改完复测：本地 `next start` 对接线上 API，改动涉及的 4 条路由（AI 专区、货架页、编码 Agent 领域页、仓库详情）在中文、英文、1440、375、深浅色下共 32 个视图都没有溢出，旧令牌计数归零。另外单独确认了三点：VoiceStudio 图标会回退到头像；首屏以下还没加载的 lazy 图片没有被误判成失败（与线上相同，18 张卡片 0 个字母块）；首选块显示「有 macOS 和 Windows 安装包」。

## 记下没改的

### 要拍板

- **旧样式页还有 9 类。** `/breakout`、`/developers`、`/mcp`、`/ecosystem`、`/map`、`/topics`、`/alternatives`（含详情）、`/blog`（含文章）、`/privacy` 套着新导航，内容区还是旧组件：绿色描边标签、等宽小字、旧卡片和旧配色。这些页是 00 场景地图里「这一轮不做」的，所以这次没动。
  - `/breakout` 顶部还有一排旧的二级标签（Leaderboard / Breakout / AI / AI Stack / AI Topics / Map），和新导航的 GitHub、AI 两个入口重复。
  - 要定的是：下一轮迁移哪几个页，哪几个合并进新页面或者下线。

### 数据侧（转数据管道线）

- **英文页里有中文。** 卡片的英文 tagline 缺失时会回退到仓库原始描述，而一部分仓库的描述是中文。见于英文版首页、`/en/apps`、`/en/ai`、`/en/alternatives/notion` 等。要补的是英文 tagline 的覆盖率，前端不用改。
- **抽错了图标。** VoiceStudio 的 `iconUrl` 是 `x.com/apple-touch-icon.png`：主页填的是 x.com，于是抽到了 X 的图标。图标抽取应跳过社交平台域名。
- **截图被浏览器拦截。** `cdn.jsdelivr.net/gh/…@HEAD/…png` 有两张被 ORB 拦（ARTEX、open-webui），Colibri 的 svg 图标也被 ORB 拦。修复 1 之后页面会正常回退，但源数据可以在入库时检查一下 content-type。

### 小问题，没改

- **375 宽的导航。** 导航条是横向滚动的。英文下「Developers」被截掉一半，「Insights」要滑动才看得到，也没有渐隐提示。中文能完整显示到「洞察」。这是一个设计取舍：要不要加右侧渐隐，等定。
- **1440 宽的模型榜。** 任务 chip 换行后「其他」单独占一行；右侧排序分段按垂直居中排，左边的 chip 是顶对齐，两边高度不齐。
- **领域页的项目数对不上。** 顶部统计写「项目 4,474」，列表底部写「共 4,432 条」（`/zh/github/c/agents`，11-22）。两个数可能来自不同口径（热力统计和列表筛选），**待确认**。

### 不能确认的

- **GA 采集请求失败。** 每一页到 `google-analytics.com/g/collect` 的请求都是 `ERR_CONNECTION_CLOSED`，但本机用 curl 访问同一个域名也连不上，所以更像是本机网络的问题，不能据此判断线上统计坏了。**待确认**：换一个网络打开页面，或者看 GA4 实时报告里有没有访问。
