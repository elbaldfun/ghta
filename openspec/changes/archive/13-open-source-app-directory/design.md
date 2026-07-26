# Design — 开源应用目录

## 1. 产品定义

**一句话**:一个「能下载、能按你的操作系统筛、按热度/新发行排序的开源应用目录」。

**什么算一个 App（语料边界）** — 已定
一个 tracked_item 进入 App 目录，当且仅当:

```
isApp = (type in {"app", "cli"})  OR  (len(platforms) > 0)
        AND type != "library"    # 库不是给终端用户跑的，唯一排除项
```

- `type=app` 是分类器已判定的「应用软件」形态（7,309 个）。
- `type=cli` **纳入**（决策）:命令行工具也是「能下载运行的软件」，其 `-linux-amd64`/`-darwin-arm64`/`.exe` 资产本就能解析出平台，自然融入目录。
- `platforms 非空` 覆盖没被判成 app/cli、但明显发行了平台构建的仓库。
- **唯一排除 library**:依赖库不是给终端用户直接运行的。

**副维度 `kind`（应用形态过滤）**:目录既含 GUI 应用又含 CLI，前端给一个 `kind` 副筛选让用户区分:
- `kind = app`（GUI/移动/Web 应用，`type=app` 或有 GUI 平台资产）
- `kind = cli`（`type=cli`）
`kind` 由 `type` 直接派生，不新增判定。

> 结果:一个 stars>1000、可下载运行的开源软件集合（应用 + CLI，预估上万，随抓取填充）。

## 2. 平台探测（本 change 的技术核心）

平台是这个功能的筛选主维度，探测分**主路 + 兜底**两层，主路权威。

### 2.1 主路:解析 release 资产文件名

现有 GraphQL 已查 releases（`adapter.go:174`），只需给**最新非 draft release**扩字段:

```graphql
releases(first: 5, orderBy: {field: CREATED_AT, direction: DESC}) {
  edges { node {
    name tagName isPrerelease isLatest isDraft publishedAt
    releaseAssets(first: 40) { nodes { name contentType downloadUrl size } }  # 新增
  } }
}
```

取「最新的 `isLatest && !isDraft`（无则退最近的稳定版）」的资产列表，逐个文件名匹配:

| 平台 | 匹配（大小写不敏感，扩展名或串内关键词） |
|---|---|
| **macos** | `.dmg` `.pkg` `-darwin` `-mac` `-macos` `-osx` `.app.zip` |
| **windows** | `.exe` `.msi` `.appx` `-windows` `-win32` `-win64` `-win.` |
| **linux** | `.appimage` `.deb` `.rpm` `.snap` `.flatpak` `.tar.gz`+`-linux` `-linux` |
| **android** | `.apk` `.aab` |
| **ios** | `.ipa` |

- 一个仓库可命中多平台(跨平台 App 常见)。
- 纯源码包(`Source code (zip/tar.gz)`、无平台关键词的 `.zip`)→ 不计平台。
- 结果去重排序存 `platforms`;同时保留命中资产的 `{name, platform, downloadUrl, size}` 存 `releaseAssets`（详情页下载区用）。

### 2.2 兜底:topic 推断（无资产时）

不用 GitHub release 发行的 App（Mac App Store / 自建站 / 商店）没有资产。用已落库的 topics 粗判:

| topic | 推断平台 |
|---|---|
| `macos` `windows` `linux` `android` `ios` | 对应单平台 |
| `electron` `tauri` `wails` `flutter`(desktop) | 桌面跨平台 = macos+windows+linux |
| `react-native` `flutter`(mobile) `ionic` | 移动 = android+ios |
| `pwa` `webapp` `web` | web |

兜底得到的平台标 `source: "topic"`（vs 资产的 `source: "asset"`），前端可标注「据 topic 推断」，避免把猜测当事实。

### 2.3 Web 平台（决策:v1 就做，含启发式）

Web/PWA 从 release 资产无从判断，综合两类信号:

```
web = topic ∈ {pwa, progressive-web-app, webapp, web-app, spa, website}
      OR ( type == "app"
           AND 有 homepage/demo URL
           AND 主语言 ∈ {JavaScript, TypeScript, Vue, Svelte, HTML}
           AND 无原生平台资产 )        # 没发 .dmg/.exe/.apk… 但是个 Web 前端栈应用
```

Web 是所有平台里**最模糊**的一档（homepage 可能只是文档站、前端栈也可能是桌面壳）。降级策略:标 `platformSource=heuristic`，前端标注「Web 应用（推断）」，并 `log` 出 web 命中量与占比，上线后盯误报、必要时收紧规则。**做，但诚实标注不确定性。**

## 3. 数据模型

`TrackedItem.sourceData`（Mongo 文档，无需改 Go struct 顶层）新增:

```jsonc
"sourceData": {
  // ... 现有 topicNames / releases / readme ...
  "platforms": ["macos", "windows", "linux"],          // 归一化去重
  "platformSource": "asset",                            // asset | topic | mixed
  "releaseAssets": [                                     // 仅命中平台的资产，供详情页
    {"name": "App-1.2.0-arm64.dmg", "platform": "macos", "url": "...", "size": 84213760},
    {"name": "App-1.2.0-setup.exe", "platform": "windows", "url": "...", "size": 72351744}
  ],
  "latestRelease": {"tag": "v1.2.0", "publishedAt": "2026-07-20T..."}  // 排序/展示
}
```

- `platforms` 建索引(`sourceData.platforms`),支撑 `os=` 过滤。
- 探测在**抓取适配器内联完成**(`adapter.go` 组装 sourceData 时顺手算),不新起 job、不进 LLM。纯函数 `platforms.go` 可单测。

## 4. 后端 API

```
GET /apps?os=&category=&sort=hot|popular|new&q=&page=&limit=
```

| 参数 | 说明 |
|---|---|
| `os` | `macos`\|`windows`\|`linux`\|`android`\|`ios`\|`web`;空=全部。过滤 `sourceData.platforms ∋ os` |
| `category` | 复用领域树前缀过滤(`^domain/`,与 trend.go 一致) |
| `sort` | `hot`(日增,默认)\|`popular`(总星)\|`new`(最新 release 时间倒序——独有角度) |
| `q` | 搜索(externalId/name/description,与 `/trending` 一致,不缓存) |
| `page`/`limit` | 分页,limit≤100,page 设上限(避免 review #11 整型溢出) |

- 返回:密集 App 行 `{externalId, name, description, language, stars, growth, platforms, platformSource, latestRelease, type, categoryPath}`。
- **实现复用**:`internal/service/app.go` 用 `ttlCache`(本轮 review 刚建的),board 键 = `os|category|sort`(全部白名单/有界值),搜索走非缓存路径。**沿用刚修好的缓存并发模式,不重蹈锁跨聚合的覆辙。**
- 可选 `GET /apps/facets` 返回各平台计数(给筛选 tab 显示数量)。

## 5. 前端 `/apps`

**筛选栏**(密集、套现有设计语言):
- OS tab:`全部 · ﻿macOS · Windows · Linux · Android · iOS · Web`,每个带平台图标 + 计数;选中态 `border-accent bg-accent/10 text-accent`(与站点一致)。
- 分类下拉(领域树)+ 排序 toggle(热度/星标/最新)+ 搜索框。

**列表行**(复用 RankTable/RepoRow 密度):
- rank · 名称/owner · 描述 · **平台徽标组**(OS 图标,资产命中实心、topic 推断描边)· 语言 chip · 分类 · stars · 日增(热度色)· `v1.2.0 · 3天前`(最新发行)· 「下载 ↓」(→ GitHub releases)/「详情」。
- 移动端:OS tab 横滑,行密度不变。

**详情页**:增「下载」区——按平台分组列 `releaseAssets`(文件名 + 大小 + 直链到 GitHub 资产),没有资产则显示「前往 Releases / 官网」。

**导航**:Header 加「应用 / Apps」入口(这是新目的地,加导航符合预期;不隐藏现有项)。

## 6. 排序与「新发行」

- `hot` = `dailyIncrease` 倒序(复用现成速度语言,默认)。
- `popular` = `metrics.stars` 倒序。
- `new` = `sourceData.latestRelease.publishedAt` 倒序 → **「最近发布新版本的开源 App」**。这是目录独有的、别处没有的发现角度(比「新建仓库」更贴近「有活跃维护、刚出新版能下载」的真实需求)。

## 7. 决策记录

| 决策 | 结论 | 依据 |
|---|---|---|
| 是否新数据源 | **否**,GitHub 语料的新视图 | 数据已在库,只缺资产字段;与 `6-add-source-appstore`(抓 Apple 榜单)正交 |
| 平台探测 | **资产解析为主 + topic 兜底** | 资产文件名是可验证的硬证据;topic 补商店/自建站发行的盲区 |
| 平台集合 | macos/windows/linux/android/ios(+web 保守) | 五大系统覆盖绝大多数;Web 难判,v1 仅 topic 命中才标 |
| App 边界 | type∈{app,cli} ∪ platforms 非空,**仅排除 library** | CLI 也纳入(决策);库不是终端直接运行 |
| 平台集合 | macos/windows/linux/android/ios/**web(含启发式)** | 五大系统 + Web 都做(决策);Web 最模糊,标注推断 |
| kind 副维度 | app / cli,由 type 派生 | 目录含 GUI 与 CLI,给用户区分,不新增判定 |
| 抓取方式 | 内联进现有 GraphQL 抓取 | 已在查 releases,加 releaseAssets 是扩字段,零新 job |
| 存储 | `sourceData.platforms`/`releaseAssets` | 源专属字段惯例落 sourceData;建 platforms 索引 |
| 缓存 | 复用 `ttlCache`(compute 出锁 + singleflight + 键上限) | 直接用本轮 review 修好的模式,不重犯锁跨聚合 |
| 详情页下载区 | **放 v1**(决策) | 「能下载」是功能核心,资产数据本就要存,不放会显半成品 |
| 不做 | v1 不抓截图/不接商店 API | 控制范围,先把「按系统筛的可下载开源软件」跑通 |

## 8. 边界与降级

- **无 GitHub release 的 App**(商店/自建站发行):无资产 → 平台靠 topic 粗判或标「来源未知」,不进 OS 精确筛选,仅在「全部」出现并提示。**诚实,不假装有数据。**
- **资产名歧义**:无平台关键词的 `.zip`/`.tar.gz` → 不计平台(宁缺毋滥)。
- **prerelease/nightly**:优先取 `isLatest` 稳定版;只有 prerelease 时取最近的,并标 pre。
- **monorepo/多产物**:`releaseAssets(first:40)` 截断;若单 release 资产超 40,可能漏平台(概率低,先接受,`log` 记录)。
- **数据陈旧**:release 会更新,平台随每轮抓取自然刷新;不单独建实时通道。
- **覆盖率诚实**:上线后 `log` 出「App 语料中有平台数据的占比」,不把「无平台」当「不支持」。

## 9. 分期

**v1(MVP,首发一次做到能用)**:
1. GraphQL 加 releaseAssets + `platforms.go` 探测(资产 + topic + Web 启发式) → 落 `sourceData.platforms`/`releaseAssets` + 索引
2. `GET /apps`(os/kind/category/sort/q/page,复用 ttlCache)
3. `/apps` 页:OS tab(6 平台,含 Web)+ kind(应用/CLI)+ 分类 + 排序(热度/星标/最新)+ 搜索 + 平台徽标行 + 下载/详情 + 导航入口 + 8 语言
4. **详情页下载区**(按平台列 releaseAssets,可直链下载)
5. 一轮回填,`log` 覆盖率与 Web 命中占比

**v2(上线后按反馈)**:
- 「本周新发行」feed / 按平台的 trending 榜
- Web/PWA 探测按误报率收紧或加强
- App 截图(README/OpenGraph 提取)
- 接商店 API 补非 GitHub 发行的下载渠道

## 10. 开放项 — 已全部确认

1. ✅ **平台集合**:五大系统 + Web(含启发式,标注推断)全做。
2. ✅ **CLI**:纳入目录,加 `kind`(应用/CLI)副维度区分。
3. ✅ **导航命名**:中文「应用」/ 英文「Apps」。
4. ✅ **详情页下载区**:放 v1。

> 设计定稿。下一步:拆 `tasks.md` + spec deltas,即可开工。
