# iOS 与 Android 应用数据获取方案

> 2026-11-21 · 状态：方案，待负责人拍板（见第九节）· 不含正式抓取代码
>
> 依据：`docs/REDESIGN.md` 5.8（多源扩展）、`docs/TAXONOMY-V3.md` 第五节（商店条目落 apps 分支、静态映射、不走 LLM）、
> `docs/DESIGN-SPEC.md` 2.2–2.3（维度模型与 URL）、`docs/PRODUCT-REVIEW.md` 第六节（"不扩新数据源"）、
> `internal/source/source.go`、`internal/source/huggingface`、`internal/item/item.go`。

## 结论先说

1. **iOS 先做，Android 暂不自抓。** Apple 有公开、免 key 的榜单 RSS 和 Lookup 接口，一天约 500 次请求就能覆盖
   5 个国家的免费、付费、畅销三种榜型 × 总榜和 25 个 genre 的前 100 名，外加这些应用的评分、评分数、版本、价格、开发者、
   图标和截图。整套抓取在本机实测都能跑通，生产 droplet 负担很小。
2. **Google Play 没有官方榜单接口。** 详情页 robots 允许抓，里面有**精确安装数**（实测 Obsidian 6,476,683）。
   但完整榜单要用 robots 禁止的 `/_/` 内部接口拿，开源库 google-play-scraper 走的就是这条路。
   所以 Android 有两条路：用第三方的榜单数据（DataForSEO 按量计费，估计每月 30 美元左右），或者只抓
   robots 允许的分类页和详情页（覆盖面小，合规上处在灰色地带）。这条要负责人定。
3. **"在涨"用名次动量做主信号，评分数增量做验证。** 商店没有 star。名次一天一变，
   最接近"今天什么在冒头"；评分数涨得慢，但很难靠买量做出来，适合拿来判断是不是真涨。Android 的精确安装数
   如果每天都更新（待确认，要观察一周），会是三个信号里最好的一个。
4. **数据模型不新造。** 条目进 `items`，`source` 用 `ios` / `android`，主指标用评分数，所以 history 环和
   growth 的代码都能直接复用。名次是"条目 × 国家 × 榜型 × genre"的多维数据，放进一个新集合
   `chart_snapshots`，一张榜一天一个文档。货架由 genre 静态映射加关键词规则判定，不走 LLM，
   生产上现在也没有可用的模型。

---

## 一、对"先不扩新数据源"的回应

`PRODUCT-REVIEW.md` 第六节（10-13）和 `REDESIGN.md` 第八节都写了"不扩新数据源"，理由是先把 GitHub 的真伪
做成权威，先把管线做扎实。现在重新评估：

| 当时的前提 | 现在的状态（11-21） |
|---|---|
| 重构未完成，管线不稳 | 10-17 已切换；夜间流水线 35 天稳定，只有一次 hot 失败；旧集合已删 |
| 新源会挤占重构人力 | 重构已收尾，现在有专职 session 线 |
| 商店源需要 LLM 分类 | 不需要：TAXONOMY-V3 已定为静态映射。生产判定没有可用模型这件事对商店源反而没有影响 |

**仍然成立的部分**：GitHub 的"真涨"仍然是主线，新源不能拖慢它。所以本方案的约束是：

- iOS 一期只占夜间流水线**之后**的时段，不和 GitHub 抢 droplet（见第五节）；
- 一期只做数据和一个 `/ios` 榜单页，不动首页、不改 GitHub 页面；
- Android 在合规渠道定下来之前不写代码。

接下来值得做的理由是**受众增量**：GitHub 榜的受众是开发者，iOS 榜的受众更广，而且"美区今天什么应用在涨"
天然是一个可以被搜索、被分享的问题。`/apps` 货架上同时出现开源应用和商店应用，也是 TAXONOMY-V3 设计
那张跨源货架表的本意。

---

## 二、能拿到什么、从哪拿

### 2.1 iOS（App Store）

| 渠道 | 地址 | 鉴权 | 内容 | 实测结论 |
|---|---|---|---|---|
| **旧版 iTunes RSS 榜单** | `itunes.apple.com/{cc}/rss/{chart}/limit=200/genre={id}/json` | 无 | 免费 / 付费 / **畅销**榜，**支持 genre**，每条带 trackId、bundleId、名称、开发者、价格、genre、图标、简介、上架日期 | ✅ 可用，**实际最多 100 条**（limit=200 也返回 100），单次约 0.4–0.5 MB，0.5–1.8 s |
| 新版 RSS 生成器 | `rss.marketingtools.apple.com/api/v2/{cc}/apps/{top-free\|top-paid}/{≤100}/apps.json` | 无 | 只有免费、付费总榜，字段少（id、名称、开发者、图标、URL，`genres` 为空） | ⚠️ 无畅销榜（404）、**忽略 genre 参数**、上限 100（请求 200 返回 500），测试期间出现过一次 502 |
| **iTunes Lookup API** | `itunes.apple.com/lookup?id=a,b,c…&country={cc}` | 无 | 详情：评分、评分数（总的和当前版本的）、价格、版本、更新时间、开发者、genre、截图、图标、大小、年龄分级、更新说明 | ✅ **一次可查 100 个 id**（1.06 MB，2.6 s）；也可以 `bundleId=` 查；`cache-control: max-age=86400` |
| iTunes Search API | `itunes.apple.com/search?term=…&entity=software` | 无 | 按关键词搜索，字段同 Lookup | ✅ 可用。只用来做"GitHub 应用 ↔ 商店应用"的关联，不用于发现 |
| genre 列表 | `itunes.apple.com/WebObjects/MZStoreServices.woa/ws/genres?id=36` | 无 | App Store 的 27 个一级 genre，以及 Games 下的 18 个子 genre | ✅ 可用。只在建映射表时用一次，结果写进 yaml |
| apps.apple.com 网页榜 | `apps.apple.com/{cc}/charts/iphone/...` | 无 | 网页 | 未测。只作为旧 RSS 下线后的备选 |

限流：Apple 文档对 Search/Lookup 的说法是约每分钟 20 次（可能调整），RSS 没有公开限额。本次对 Apple 一共请求 17 次，
没有遇到 429。**我们按每 3 秒最多 1 次来设计**，比文档的限额还低。

条款：Apple 没有给这几个接口单独写使用条款。RSS 生成器挂在 Apple Performance Partners 的工具页下，
用途是在网站上展示榜单。展示名次、名称、图标并链回 App Store 是这些接口的正常用法。截图属于开发者的版权，
**只保存 URL、热链 mzstatic，不转存**（现有 `assets.screenshotUrl` 本来就是 URL）。
旧版 RSS 没有 SLA，随时可能下线，这是 iOS 方案最大的单点风险，降级方案见 6.3。

### 2.2 Android（Google Play）

| 渠道 | 地址 | 内容 | 实测 / 合规 |
|---|---|---|---|
| Google Play Developer API | 官方 | 只能查**自己账号下**的应用 | ❌ 不适用 |
| **详情页 HTML** | `play.google.com/store/apps/details?id={pkg}&hl=en_US&gl={cc}` | JSON-LD 里有名称、分类、评分、评分数、价格、开发者、图标；页内数据里有安装区间（`5,000,000+`）和**精确安装数**（`6476683`）、更新时间、版本 | ✅ robots 允许；单页约 1.3 MB，2.7–3.1 s。解析依赖页面结构，Google 改版就会失效 |
| 分类页 HTML | `play.google.com/store/apps/category/{CAT}?gl={cc}` | 首屏是"该分类免费榜"（`apps_topselling_free`），本次解析出 40 个包名 | ✅ robots 允许。付费、畅销榜要点按钮切换，切换走的是 `/_/` 接口 |
| `/_/PlayStoreUi/data/batchexecute` | 内部 RPC | 完整榜单、搜索、评论（google-play-scraper 用的就是它） | ❌ **robots 明确 `Disallow: /_`**，也 `Disallow: /store/search`。不用 |
| 首页 `/store/apps/top` | — | 302 到 `/store/apps` 首页，**没有榜单** | ❌ |

合规评估：Google 服务条款禁止"违反网页上机器可读指令（如 robots.txt）的自动化访问"（条款原文**待负责人复核**）。
按这个读法，只抓 robots 允许的详情页和分类页、低频、不登录，处在灰色地带但站得住；走 `/_/` 接口是明确违规。
另有两个现实风险：一是页面数据是混淆过的数组，按位置取字段，Google 每次改版都可能悄悄取错；
二是 droplet 的 IP 是机房段，被限流或验证码拦下的概率比本机高，而绕过验证码不在我们的边界内。

### 2.3 第三方数据服务

| 服务 | 覆盖 | 价格（**多为第三方报价，均待确认**） | 条款要点 | 适合我们吗 |
|---|---|---|---|---|
| **DataForSEO** App Data API | Google Play / App Store 榜单（App List）、详情、搜索 | 榜单按量：标准队列 **$12 / 100 万条**（45 分钟内返回），优先队列 $24 / 100 万；预付充值 | 数据可以用在自己的产品里；结果 30 天内可以免费重取 | ✅ **Android 首选**：5 国 × 3 榜 × 约 50 分类 × 100 名 ≈ 每天 7.5 万条 ≈ **每月 27 美元** |
| SerpApi `google_play` 引擎 | Google Play 榜单、搜索、详情 | 按次：$75 / 5,000 次，$275 / 3 万次（月付，额度不结转） | 次数制，一次一张榜 | 一样的榜单量每月约 2.2 万次，要 275 美元一档，比 DataForSEO 贵约 10 倍 |
| Apify 社区 actor | iOS / Android 榜单 | 约 $1 / 1,000 条名次 | 由第三方开发者维护，稳定性参差 | ❌ 同样的量每天 75 美元 |
| AppTweak API | 两商店的元数据、关键词、名次；**下载量估算只有企业版有** | 约 $166 / 月起（2.5 万 credit），一个应用的一个指标 1 credit，历史数据 5 credit | 不能转售原始数据 | ❌ 对榜单站来说按 credit 计费太贵 |
| 42matters（已被 Similarweb 收购） | 两商店，100+ 国家，10.8 万张榜，每天更新 | 有报价称 €349 / 月起，也有说 $150+ | 14 天试用 | 可以做备选，价格要询价 |
| Appfigures | 两商店，名次、评分、估算下载 | 订阅 $7.99–29.99 / 月；API 是附加项，价格没查到公开报价 | 公开数据的 API 有调用上限 | 待询价 |
| Sensor Tower（2024-03 收购 data.ai） | 最全，含下载和收入估算 | 不公开报价；第三方估计 API 附加项每年 5,000–20,000 美元 | 企业合同，限制再分发 | ❌ 超出预算，而且条款不允许公开展示 |

注：只有 Sensor Tower、AppTweak 企业版和 Appfigures 提供**下载量估算**。这类估算是模型算出来的，
不同厂商的数字能差出几倍，条款也普遍禁止公开展示，所以**不进入本方案**。

### 2.4 字段来源对照

| 字段 | iOS | Android（自抓详情页） | Android（DataForSEO） |
|---|---|---|---|
| 榜单名次 | 旧版 RSS（国家 × 榜型 × genre，前 100） | 分类页首屏，仅免费榜、约 40 名 | App List（三种榜型 × 分类） |
| 评分 | Lookup `averageUserRating`（按国家） | JSON-LD `ratingValue`（全球） | 详情接口 |
| 评分数 | Lookup `userRatingCount`（按国家，所有版本累计） | JSON-LD `ratingCount`（全球） | 详情接口 |
| 下载量 | **拿不到**（Apple 不公开） | 区间 `5,000,000+` 加**精确值** `6476683` | 区间（精确值待确认） |
| 价格 | RSS `im:price`、Lookup `price` | JSON-LD `offers.price` | 有 |
| 版本 / 更新时间 | Lookup `version`、`currentVersionReleaseDate` | 页内数据（"Updated on"） | 有 |
| 开发者 | Lookup `artistId`、`artistName`、`sellerUrl` | JSON-LD `author` | 有 |
| 图标 | Lookup `artworkUrl512` | JSON-LD `image` | 有 |
| 截图 | Lookup `screenshotUrls`（iPhone）、`ipadScreenshotUrls` | 页内数据 | 有 |
| 分类 | `primaryGenreId` 和 `genreIds` | `applicationCategory` | 有 |

---

## 三、小规模实测记录

时间：2026-11-21 12:47–12:48（UTC+8），在本机经代理请求。**共 23 次请求**（新版 RSS 8 次、旧版 RSS 4 次、Lookup/Search/genre 5 次、Google Play 6 次，其中 robots.txt 读了 2 次），全部只读，没有登录，也没有用 cookie。
UA 是自报的 `starrank-probe/0.1`。注意：本机的 Apple 请求经过国内 CDN 缓存（响应头里有 `KS-CLOUD`），
所以延迟和缓存行为和生产 droplet（DO，美区）**不一样**，上线前要在 droplet 上用同样的少量请求再测一次。

| # | 请求 | 结果 |
|---|---|---|
| 1–2 | 新 RSS 美区 top-free / top-paid 100 | 200，52–54 KB，3.8–4.3 s；`genres: []`；feed `updated` 为 04:47 UTC |
| 3 | 新 RSS top-grossing | **404** |
| 4 | 新 RSS top-free/200 | **500** |
| 5 | 新 RSS 日区 top-free/50 | 200，30 KB |
| 6–7 | 新 RSS 加 `?genre=6007` 与不加 | 加 genre 的返回 200；不加的那次 **502**（Apple 网关），只能判断 genre 参数不在官方文档里，结果不可靠 |
| 8–11 | 旧 RSS 美区 free / grossing、美区 paid × Productivity、日区 free × Business | 全部 200，406–517 KB，0.5–1.8 s；**每张 98–100 条**；genre 过滤生效（条目分类全是 Productivity / ビジネス） |
| 12 | Lookup 100 个 id（美区免费榜） | 200，**返回 100 条**，1.06 MB，2.6 s；`cache-control: max-age=86400` |
| 13 | Search `obsidian` | 200，59 KB，0.6 s，Obsidian 评分数 2,831 |
| 14 | genre 树 | 200，27 个一级 genre，Games 下 18 个子 genre |
| 15–16 | Lookup 日区单个 id、按 bundleId 查 | 200 |
| 17 | play.google.com/robots.txt | `Disallow: /_`、`/store/search`、`/store/apps/datasafety*`、`/store/getreviews`；详情页和分类页没有被禁止 |
| 18–19 | GP 详情页 `md.obsidian`、`com.openai.chatgpt` | 200，1.28–1.31 MB，2.7–3.1 s；JSON-LD：Obsidian 评分 4.43、评分数 19,480；ChatGPT 评分数 69,231,926；安装区间 5M+ / 1B+；**精确安装数 Obsidian 6,476,683** |
| 20 | GP `/store/apps/top` | 302 到首页，2.2 MB，没有榜单 |
| 21 | GP 分类页 PRODUCTIVITY | 200，1.7 MB；首屏"Top free productivity apps"，解析出 40 个包名（ChatGPT、Docs、Claude、Gemini、Word…）；付费、畅销是切换按钮 |

### 3.1 droplet 重测（P0，2026-11-22 10:46 UTC）

在生产 droplet 上用一次性 curl 发了 9 次请求，没装任何东西，临时文件用完就删了。

| 请求 | 结果 |
|---|---|
| 旧版 RSS：us 免费、cn 免费、cn 畅销 × Productivity、jp 付费、gb 免费 × Games、de 畅销 | 全部 200，385–498 KB，**0.15–1.15 s**；每张 99–100 条；**cn 区可用** |
| Lookup：us、cn 各 100 个 id | 都是 200，各返回 100 条，约 1.06 MB，1.6–2.3 s |
| 新版 RSS：us 免费 100 | 200，53 KB |

feed 里的 `updated` 等于请求时刻，说明榜单是按请求实时生成的，所以每天要在固定时刻去读。当时 droplet 的负载是 0.26，磁盘用了 72%（剩 14 GB）。

观察到的稳定性：Apple 新版 RSS 生成器 8 次请求里出了 2 次 5xx 和 1 次 404（其中一次是参数越界），旧版 RSS 和 Lookup 全部成功。
样本太小，不足以下稳定性结论，所以适配器从第一天起就按"会失败"来写：单张榜失败可以重试，单张榜缺失不影响别的榜。

---

## 四、"增长"怎么定义

商店没有 star。三个候选信号：

### 4.1 名次动量（主信号，两个商店都有）

名次变化不是线性的：从 50 名升到 40 名很平常，从 5 名升到 1 名很难。所以用对数尺度：

```
score(r) = ln(101 / r)          r ∈ 1..100；不在榜上记 r = 101，score = 0
momentum_d1 = score(r_t) − score(r_{t−1})
momentum_d7 = score(r_t) − median(score(r_{t−7..t−1}))   ← 用中位数，压住一天的抖动
```

- "今天在涨"：在同一张榜（国家 × 榜型 × genre）里按 `momentum_d7` 排序，界面上显示"上升 N 名"（`r_{t−1} − r_t`），
  新上榜的显示"新上榜"。
- 跨榜合成（比如某个应用在美区整体在涨）：取这个应用在各个 genre 榜里 `momentum_d7` 的最大值，不求和。
  否则同时上了总榜和分类榜的应用会被算两遍。

**局限**：只有前 100 名，榜外的变化看不到；名次是相对量，同 genre 里头部应用掉下去，别人就会被动上升；
畅销榜看的是收入，游戏的一次活动就能冲上去；名次可以靠买量冲上去，这正是 4.2 存在的原因。

### 4.2 评分数增量（验证信号，iOS 主用）

```
Δratings_d7 = ratingCount_t − ratingCount_{t−7}            （同一国家商店）
ratingRate  = Δratings_d7 / max(ratingCount_{t−7}, 100)    （相对增速，100 是平滑项）
```

这就是 `items.metrics.ratings` 和现有的 `growth.d1/d7/d30`，代码不用改。

**局限**：评分数大约是下载量的 1–2%，而且受应用内评分弹窗影响很大，开发者一次改版就能让增速跳一档；
Apple 会清理刷出来的评分，所以评分数会出现负增长，要允许负值；评分数按国家分开算，美区和日区不能相加。

**用法**：名次在冲但评分数没跟上，打一个"可能是买量"的标签。复用 `labels.quality` 的结构：档位加理由。

### 4.3 下载量（只有 Android）

- 区间跳档（`1,000,000+` → `5,000,000+`）：每档是上一档的 2 倍或 5 倍，一个应用几个月才跳一次，只能做成
  "里程碑"事件（"本周突破 500 万下载"），不能做日增长。
- **精确安装数**（页内数据，Obsidian 6,476,683）：如果每天都更新，`Δinstalls_d1` 会是三个信号里最直接的。
  **待确认**：要对同一批包名连续观察 7 天，看数字是不是每天在变。这个精确值不在 Google 公开的界面上，
  展示时只给相对值（"7 天 +3.2%"），不展示绝对数。

### 4.4 推荐

`/ios` 的"今天在涨"用 4.1 排序，用 4.2 做真涨校验。`/today` 跨源区块里 iOS 只取经过校验的条目。
Android 先用 4.1；4.3 确认之后，再把 `Δinstalls` 换成 Android 的主指标。

---

## 五、范围与成本

### 5.1 首批范围

| 维度 | 一期建议 | 理由 |
|---|---|---|
| 国家 | **us、cn、jp、gb、de**（5 个） | 美区是全球风向标；cn 和 jp 对应站点的中文、日文用户；gb 和 de 覆盖欧洲。cn 区在旧版 RSS 里可用，但上线前要再测一次 |
| 榜型 | 免费、付费、畅销 | 三种都有现成接口 |
| genre | 总榜加 25 个一级 genre（去掉 Stickers；Catalogs 在美区三种榜都是空的，11-22 也去掉了） | Games 的 18 个子 genre 二期再说 |
| 榜深 | 100（接口上限） | |

### 5.2 每天的请求量（iOS 一期，按 11-22 冒烟实测修正）

11-22 在本机对 us 一个国家完整跑了一遍：读 81 张榜，查到 **6,422 个不重复的应用**，65 次 Lookup 全部成功，0 失败，用时 13 分钟。原先估的"5 国合计约 1 万个应用"偏低。下面按一个国家约 6,400 个应用、跨国家约三成重复来估。

| 项 | 计算 | 次数 / 天 | 下载量 / 天 |
|---|---|---|---|
| 榜单 | 5 国 × 3 榜型 × 26（总榜 + 25 genre） | 390 | 约 180 MB |
| 详情 | 去重后约 2–2.5 万个应用，每个只在主国家查一次，100 个一批 | 约 230 | 约 240 MB |
| **合计** | 每 3 秒最多 1 次；droplet 上单次 0.15–2.3 s | **约 620 次，45–60 分钟** | **约 420 MB** |

榜单是一张一张流式解析的，内存峰值是 1 MB 左右的 JSON，对 worker 可以忽略。DO 的月流量额度远超每月 13 GB，不另外计费。

### 5.3 存储增长（按冒烟实测的文档大小）

实测：一个 iOS 条目平均 **2.7 KB**，一个榜单快照文档平均 **1.9 KB**。

| 集合 | 每天（压缩前） | 一年（压缩前） | 说明 |
|---|---|---|---|
| `chart_snapshots` | 390 × 1.9 KB ≈ **0.75 MB** | 约 270 MB | TTL 400 天，过期以后就不再增长 |
| `metric_snapshots`（iOS 部分） | 每个条目每天 1 个点，约 2.5 万 × 150 B ≈ **3.7 MB** | 约 1.35 GB | 时序集合压缩率高，落盘约 1/5，约 0.27 GB |
| `items`（ios） | 前几周增长，之后基本持平 | 2.5–4 万条 × 2.7 KB ≈ 70–110 MB | 掉出榜单的条目保留历史 |
| `item_blobs`（ios） | 只有简介变了才重写 | 约 100 MB | 完整的商店简介 |
| **合计** | **约 4.5 MB** | **约 1.8 GB，落盘约 0.5 GB** | |

生产盘 11-22 实测已用 72%，剩 14 GB。一年 0.5 GB 落盘的量放得下，但要注意以下几点：

- `chart_snapshots` 加 TTL，**保留 400 天**（一年的同比，加一个月余量）；
- 不按"应用 × 国家 × 榜 × 天"一条一个文档存（REDESIGN 5.8 原来写的时序集合），那样每天 4 万个文档，元数据比数据还大；改成一张榜一个文档，读榜单页只要一次查询；
- `metric_snapshots` 里 iOS 的点占了大头。如果以后磁盘紧张，可以让 iOS 不写这个集合：growth 用的是条目里 35 天的 history 环，用不到它。这需要改 `WriteFetched`，一期先不做。

### 5.4 抓取时段

GitHub 全量流水线 UTC 03:30 开始，实际要跑到约 07:30（"全量抓取耗时过长"那条）。iOS 作业排在 **UTC 09:00**：

- 晚于 08:30 的窗口，不和 GitHub 流水线抢 CPU 和 mongo；
- Apple 的榜单在白天也会更新，每天固定在同一时刻取，才能保证两天的名次可以比较；
- 作业由 worker 的调度器触发，**在 GitHub 流水线未结束时跳过并等待**（检查 `fetch_runs` 的最近一次运行），
  不和它并发。

---

## 六、数据模型

### 6.1 `items`

一个应用一个条目，不是一个应用 × 一个国家一个条目。

| 字段 | iOS | Android |
|---|---|---|
| `source` | `ios` | `android` |
| `externalId` | trackId 字符串，如 `"6448311069"`（URL 为 `/ios/id6448311069`，与 DESIGN-SPEC 2.3 一致） | 包名，如 `"md.obsidian"` |
| `name`、`description` | trackName；简介前 1 KB（完整版放 `item_blobs.readme`） | 同 |
| `homepage` | `sellerUrl` | JSON-LD `author.url` |
| `primaryMetric` / `metricDirection` | `ratings` / `desc` | `ratings` / `desc`（4.3 确认后改成 `installs`） |
| `metrics` | `ratings`、`rating`、`price`、`bestRank`（当天所有榜里最好的名次） | 同上加 `installs`、`installsFloor` |
| `history` / `growth` | 主指标的 35 天环，**现有代码直接用** | 同 |
| `labels.domain` | `ios/<genre-slug>`，如 `ios/productivity`（按源命名空间，与 HF 的 `hf/<task>` 同一做法） | `android/<category>` |
| `labels.shelf` | 静态映射得出（6.3），映射不到就留空 | 同 |
| `labels.by` | `source` | `source` |
| `assets.iconUrl` / `screenshotUrl` | `artworkUrl512` / `screenshotUrls[0]` | 同 |
| 来源专有字段 | 新增 `App *StoreApp`（bson `app`，不用 `store`，避免和旧系统的 `store.*` 字段混淆）：`bundleId`、`developerId`、`developer`、`genreId`、`genreIds`、`version`、`currency`、`contentRating`、`screenshots`（前 3 张）、`updatedAt`、`releasedAt`、`homeCountry`、`countries`（今天在哪些国家上榜） | 同一个结构，`bundleId` 存包名 |

- **主国家** `homeCountry`：应用第一次被看到的那天，按国家列表顺序（us、cn、jp、gb、de）取它上榜的第一个国家，**之后固定不变**。以后哪怕它只在别的国家上榜，也照样去主国家查详情，否则一条 history 里会混进两个国家的评分数（codex 审查 11-22 发现）。
- **各榜名次** `app.charts` 和 `app.chartsDay`（11-25 批次 0，给 iOS 详情页和列表卡片用，见 Obsidian「StarRank iOS 应用在网页上展示」前端小节第 2 节）：
  - `app.charts` 是 `[{country, chart, genre, rank, prev}]`，记应用当天上过的每一张榜，覆盖全部配置国家，不只主国家。排序是国家、榜型、genre（all 在前）。`genre` 和 `chart_snapshots` 一致，取值是 "all" 或 genre id。
  - `prev` 是这张榜上一份快照（上一个读到它的那天，不一定是昨天）里的名次。那份快照里没有这个应用、或者没有更早的快照时，不写 `prev`。前端和榜单页比较的是同两份快照。
  - 跟着同一次 upsert 写进 `app`，不多一轮全量写。
  - 全部请求都成功的那天，`chartsDay` 不是当天的条目（当天不在任何榜上）会去掉 `app.charts`，`chartsDay` 留着，表示最后一次上榜的日子。有请求失败的那天不清，因为可能只是失败漏掉了。
  评分、评分数只在主国家取，每个应用一天只查一次 Lookup。代价是只在日区上榜的应用，展示的就是日区的评分数，
  详情页要写明"日本区评分"。
- `StoreApp` 是 iOS 和 Android 共用的一个结构，不像 `Gh`、`Hf` 那样每个源一个，因为两个商店的字段几乎一样。
- `domain.Source` 里已有一个 `SourceAppStore = "appstore"`，是旧数据模型留下的，`items` 里没有用过。
  新代码用 `item.SourceIOS = "ios"`、`item.SourceAndroid = "android"`，和 URL 前缀一致，旧常量不动。

### 6.2 `chart_snapshots`（新集合）

```
{
  _id:     "ios|us|free|6007|2026-11-21",
  source:  "ios", country: "us", chart: "free" | "paid" | "grossing",
  genre:   "6007" | "all",
  day:     "2026-11-21",
  fetchedAt: ISODate,
  entries: [ "6448311069", "…", … ]   // 按名次排列，名次 = 下标 + 1，最多 100 条
}
索引：{ source, country, chart, genre, day: -1 }；TTL：fetchedAt 400 天
```

- 榜单页（`/ios?country=us&chart=free&genre=productivity`）读今天和 7 天前的文档，在内存里算 4.1 的动量，
  再物化进 `boards`。boards 的 key 按 REDESIGN 5.8 的写法：`ios|us|free|6007|<date>`。
- `bestRank` 回写 `items.metrics`，用来做详情页和货架页排序。

### 6.3 货架映射（不走 LLM）

`taxonomy/store-genres.yaml`，两层规则，加载时检查每个目标货架在 `taxonomy.yaml` 里存在：

1. **genre → 默认货架**：Graphics & Design → `creative/image`；Photo & Video → `creative/video`；Music → `media/music`；
   Books → `media/reader`；Social Networking → `social/messaging`；Games → `games/games`；
   Developer Tools → 不进货架（v3 已把 devtools 移出 apps 分支）。
2. **genre + 名称或简介关键词 → 货架**：Productivity 加 note 关键词 → `productivity/notes`，加 task、todo、calendar
   → `productivity/todo`，加 pdf、office、docs → `productivity/docs`，加 AI、chat、assistant → `ai/assistant`；
   Utilities 加 VPN → `network/proxy`，加 browser → `network/browser`，加 password → `security/passwords`，
   加 file → `system/files`，加 backup → `system/backup`……
3. 其余（Finance、Shopping、Travel、Health、Food、Sports、News、Weather、Navigation、Lifestyle、Medical、
   Education、Reference、Business、Entertainment 等）**不进货架**，只在 `/ios/c/<genre>` 榜单页出现。

这和 TAXONOMY-V3 第五节"26 个 genre 对到 38 个货架"的说法有出入：39 个货架是为开源软件设计的，
大约一半的消费类 genre 没有对应的货架。这里不补货架，补货架要改分类树并写 migrations，等商店数据上线、
看到量以后再说。规则表配一个 golden 文件（50 个人工标注的应用），`go test ./internal/taxonomy` 一起校验。

### 6.4 适配器与任务（P1 实现，11-22）

没有照 HF 的写法实现 `ItemFetcher`。原因有两个：一是 iOS 不在夜间流水线里，有自己的时间表；二是它要分两步，先读完所有榜单，才能知道每个应用的主国家，再去查详情，这和"分片互不依赖"的模型合不上。

- `internal/source/appstore`：`Client` 负责旧版 RSS 榜单和 Lookup，请求间隔至少 3 秒（`IOS_PACE_MS`）。5xx 和超时按 5 s、15 s、45 s 退避重试；429 和 403 记为 `ErrThrottled`；404 记为 `ErrNoChart`。`ToFetched` 把查到的详情转成 `item.Fetched`。
- `internal/pipeline/ios.go`：任务名 `ios`。先按国家 × 榜型 × genre 的顺序读 390 张榜，每张写一个 `chart_snapshots` 文档；再按主国家每 100 个一批调 Lookup，用现有的 `WriteFetched` 写进 `items`，最后用 `AgeGrowth` 把当天掉出所有榜单的应用的增长归零。
- 调度：`cmd/worker` 里单独挂一个 cron，`IOS_CRON` 默认 `0 0 9 * * *`（UTC），设成 `off` 就关掉；国家列表在 `IOS_COUNTRIES`，默认 `us,cn,jp,gb,de`。它和夜间流水线共用一个 runner，任务严格串行，所以流水线超时跑到 09:00 以后，iOS 任务会排队等它跑完，不会和它并发。也可以用 `worker -run ios` 单独跑一次。
- 货架：留了 `appstore.Shelver` 接口。现在没有配置，所以 iOS 条目不带 `labels.shelf`。映射表等榜单与内容规则线出草案、负责人确认后再接（见第十一节）。

### 6.5 降级

| 失败 | 处理 |
|---|---|
| 单张榜 5xx / 超时 | 重试 3 次，指数退避；还是失败就记录缺失，那张榜当天不写快照，动量用前一天的值；当天**不跑 AgeGrowth**（只在那张榜上的应用并没有掉榜） |
| 旧版 RSS 整体下线（整批 404 / 410） | 一张榜都读不到时任务失败，`job_runs` 里记为 failed，这就是报警。P1 没有实现自动降级，到时候再接新版 RSS 生成器（只有免费、付费总榜，没有 genre），并评估 apps.apple.com 网页榜 |
| Lookup 限流（429 / 403）或部分批次失败 | 停止（限流时）或跳过失败的批次，已查到的照常写入；当天**不跑 AgeGrowth**，没查到的应用保留原来的增长（沿用 #61 的做法：抓取失败不改增长）。一张榜都读不到时任务失败，不写任何条目 |

---

## 七、分阶段

| 阶段 | 内容 | 上线标准 | 负责人要做的 |
|---|---|---|---|
| **P0 准备** ✅ 11-22 | 在 droplet 上用 10 次以内的请求重测 2.1（核实 cn 区、延迟、有没有拦截） | 结果见 3.1 | 无 |
| **P1 iOS 数据**（3–4 天） | 适配器、`chart_snapshots`、`items` 写入、genre 映射表加 golden、UTC 09:00 调度 | develop 上连续 7 天无人值守跑完；每张榜的条数 ≥ 95；不影响 GitHub 流水线的完成时间 | 合并 PR、发版 |
| **P2 iOS 页面**（3 天） | `/ios` 榜单页（国家 × 榜型 × genre）、`/ios/id{n}` 详情页、`/v1` 接口；`/apps` 货架加平台切换 | 7 天的动量数据就绪后才开放；sitemap 收录 | 确认首页和导航要不要露出 |
| ~~**P3 Android 观察**~~（负责人 11-22 定：随 Android 暂缓，重新考虑时先复核 Google 条款再做） | 50 个包名的详情页每天抓一次，记录精确安装数是否每天变化 | 给出 4.3 是否可用的结论 | 定 Android 走哪条渠道（第九节第 2 条） |
| **P4 Android 数据和页面** | 按第 2 条的决定实现；数据模型与 iOS 同构 | 同 P1、P2 | 选了 DataForSEO 的话：注册账号、预充值（最低充值额待确认） |
| 以后 | Games 子 genre；跨源关联（GitHub 仓库 ↔ App Store ↔ Google Play，用 bundleId、包名和主页域名匹配）；更多国家 | | |

**iOS 一期不需要负责人花钱，也不需要开发者账号。** Apple 开发者账号（99 美元一年）在本方案里没有用处。

---

## 八、不做的事

- 不抓 `play.google.com/_/` 内部接口，不抓搜索页和评论，不绕过验证码，不登录，不轮换 IP。
- 不展示第三方的下载量估算或收入估算。
- 不转存截图和图标，只保存 URL。
- 不为消费类 genre 新增货架（6.3）。
- 不抓用户评论正文。评论正文涉及个人数据，Google Play 的条款明确禁止收集用户的个人数据。

---

## 九、需要负责人拍板的事

1. **是否同意 iOS 一期按本方案做**：用旧版 iTunes RSS 加 Lookup，接受它没有 SLA、随时可能下线的风险
   （降级方案见 6.5）。Claude 的建议：做。
2. **Android 走哪条渠道**：
   - A. DataForSEO 按量付费，估计每月 30 美元左右，要注册和预充值。合规风险转给了供应商，覆盖最全。
   - B. 只自抓 robots 允许的分类页和详情页，免费，但只有免费榜前约 40 名，页面改版就会坏，条款上是灰色地带。
   - C. 暂不接 Android，等 iOS 上线、看到流量再定。
   Claude 的建议：先做 P3 的一周观察（50 个包名、每天 50 次请求，不论选哪条都需要这一步），然后选 **A**；
   预算不批就选 C，不选 B。
3. **首批国家**：us、cn、jp、gb、de 行不行，还是只做 us。
4. **货架缺口**：消费类 genre 暂时不进 `/apps` 货架（6.3），只在 `/ios` 榜单页出现，可以接受吗。
5. **Google 服务条款里关于自动化访问的原文**：请负责人复核 2.2 的读法（如果选了 B，或者 P3 要在生产上跑，必须先确认）。

---

## 十、session 规划

> 2026-11-22 追加。11-21 负责人定的"iOS 不做、Android 暂缓"，11-22 改为重新讨论。

| 阶段 | session | 职责 | 边界 |
|---|---|---|---|
| **讨论期**（现在） | 只有「移动应用数据」一条 | 和负责人把第九节的事定下来；需要时在本机做只读重测 | 不写抓取代码、不开 PR、不碰生产 |
| **定了要做之后** | iOS 采集 | 6.4 的 iOS 适配器、`chart_snapshots`、`items` 写入、降级（P0、P1） | 只开 PR 到 develop，不发版 |
| | Android 采集 | P3 观察，以及按负责人选定的渠道实现 P4 的数据部分 | 同上；渠道没定之前不开工 |
| | 前端功能线（已有） | `/ios`、`/android` 榜单页和详情页，`/apps` 货架的平台切换（P2、P4 的页面部分） | 数据接口就绪后再开工 |
| | 榜单与内容规则线（已有） | `taxonomy/store-genres.yaml` 货架映射和 golden（6.3）、增长口径（第四节）、`boards` 物化 | 改分类树要按 CLAUDE.md 写 migrations |
| | 数据管道线（已有） | UTC 09:00 调度、和 GitHub 流水线的互斥、`chart_snapshots` 的 TTL 和磁盘水位（5.3、5.4） | 生产磁盘已用 77%，上线前先给出余量 |

两条采集 session 共用 6.1 的 `Store` 结构，由先开工的那条定义，后开工的那条只做补充。

---

## 十一、2026-11-22 负责人结论

1. **先 iOS 后 Android。** iOS 一期按本方案做，Android 等 iOS 上线之后再说。
2. **Android 继续暂缓。** DataForSEO 和自抓都不做；P3 的一周观察：负责人 11-22 定，随 Android 一起暂缓；以后重新考虑 Android 时，先复核 Google 条款再做。
3. **首批国家：us、cn、jp、gb、de。**
4. **现在就补消费类货架。** 推翻 6.3 第 3 条：财务、购物、旅行、健康等消费类 genre 要有自己的货架。
   做法是改 `taxonomy/taxonomy.yaml` 和 `migrations.yaml`，版本号加一，同时补 `store-genres.yaml` 和 golden，
   交给榜单与内容规则线。货架大类怎么分，由那条线出草案，负责人确认。
