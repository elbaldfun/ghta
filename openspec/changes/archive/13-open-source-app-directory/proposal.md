# Change: 开源应用目录（Open-Source App Directory）

> **状态:已交付并上线,归档于 2026-07-26。** 能力规范见 `openspec/specs/app-directory/spec.md`;完成清单见本目录 `tasks.md`。v2 的「开源替代」在本轮一并落地。

## Why

站点已积累大量「能装能跑」的开源应用仓库，但它们散落在通用排行榜里，用户**无法按自己的操作系统筛选、也无法一眼看出哪个仓库真的发行了可下载的构建**。现状盘点（生产库实测）：

- `type=app`（分类器判定的「应用软件」形态）：**7,309** 个仓库
- 带桌面/移动 topic（electron/tauri/flutter/react-native/desktop-app/android/ios/…）：**5,490** 个
- 两者并集且 stars>1000：**11,241** 个——语料充足，样本对路（vscode、scrcpy、clash-verge-rev、opencode…）

关键缺口:**我们已经在抓 release 元数据，但没抓 release 资产文件名**（`internal/source/github/adapter.go:174` 只取了 release 的名字/tag/时间，没取 `releaseAssets`）。资产文件名（`.dmg`/`.exe`/`.AppImage`/`.apk`/`.ipa`…）是判断「发行了 App + 支持哪些系统」**最硬的证据**——不是猜。

这个交叉（**能下载 · 按系统筛 · 按热度排 · 开源**）没有现成对标物同时覆盖:AlternativeTo 不限开源也不讲增速；awesome-list 静态无排序；Flathub 只有 Linux；star-history 只画单仓库曲线。它正好扬站点「速度/排行」的长处。

> 范围决策（已拍板）:**做全量开源 App，不限 AI**。这与 `project.md` 的多平台情报站定位一致，AI App 作为其中一个分类子集自然包含。

## What Changes

- **抓取扩展**:GitHub GraphQL 查询给最新 release 增 `releaseAssets(first:N){ nodes{ name contentType downloadUrl size } }`。**复用现有主抓取，不新起 job、不新增数据源**（本 change 不是 `6-add-source-appstore` 那种新源，而是对 GitHub 语料的新视图）。
- **平台探测**:资产文件名 → 平台集合（主路，权威）+ topic 兜底（无 GitHub release 但走商店/自建站发行的 App）。归一化为 `platforms: []string`（macos/windows/linux/android/ios/web），落 `sourceData`。
- **App 语料判定**:`isApp = type∈{app,cli} OR platforms 非空`，仅排除 `type=library`。CLI 也纳入（命令行工具同样是可下载运行的软件），前端用 `kind`（应用/CLI）副维度区分。
- **查询 API**:新增 `GET /apps`——按 `os` / `category`（复用领域树）/ `sort`（hot/popular/new）/ `q`（搜索）过滤分页，返回带平台徽标与最新发行信息的密集行。复用现有缓存榜单模式。
- **前端**:新增 `/apps` 页——OS 筛选 tab（含平台图标)+ 分类 + 排序 + 搜索 + 平台徽标行 + 下载/详情入口；导航加「应用」入口；详情页增下载区（按平台列资产）。
- **排序新增「new」轴**:按「最新 release 时间」排——「本周新发行的开源 App」是这个功能独有的、别处没有的角度。

## Impact

- **Affected specs**:
  - `github-trend-fetching`（抓取增 releaseAssets）
  - `trending-query`（新增 apps 查询能力 + platforms 过滤维度）—— 或新建独立 capability `app-directory`
- **Affected code (Go)**:
  - `internal/source/github/client.go` + `adapter.go`:GraphQL 增 releaseAssets;解析资产→平台;落 `sourceData.platforms` / `sourceData.releaseAssets`
  - `internal/domain`:`TrackedItem.sourceData` 增 `platforms`、`releaseAssets` 字段约定
  - 新增平台探测器 `internal/source/github/platforms.go`（资产名/topic → 平台，纯函数、可单测）
  - 新增 `internal/service/app.go` + `internal/handler/apps.go`:`/apps` 榜单/搜索,复用 `ttlCache`
  - `internal/repository/store.go`:`sourceData.platforms` 索引
  - `api/openapi.yaml`:`/apps` 端点
- **Affected code (Frontend)**:
  - `web/src/lib/data.ts`:`getApps`、`AppItem` 类型（含 platforms、latestRelease）
  - 新增 `web/src/app/[locale]/(rank)/apps/page.tsx` + `web/src/components/rank/AppsBoard.tsx`、平台徽标组件
  - `Header.tsx` 加导航;详情页下载区;`messages/*.json` 8 语言键
- **数据回填**:上线后需一轮全量重抓（或增量）以填充 `platforms`;历史仓库在下一个抓取周期自然补齐。无破坏性迁移。
- **诚实边界**:不走 GitHub release 发行的 App（商店/自建站）无资产 → 平台靠 topic 粗判或标「来源未知」,不计入 OS 精确筛选。这一降级在 design.md 记录。
