# Tasks — 开源应用目录

状态:**已交付并上线**(2026-07-26)。

## 平台探测(v1 §1–2)

- [x] `internal/source/github/platforms.go`:资产文件名 → 平台(权威)+ topic 兜底 + Web 启发式(名字含 desktop 否决);忽略 `.sig/.blockmap/校验和`;归档需显式 OS 关键词
- [x] 单元测试(真实 App 资产名样本)+ `PLATFORMS_LIVE=1` 实时 demo
- [x] GraphQL 抓 `releaseAssets`;`adapter.go` 内联探测,落 `sourceData.platforms/platformSource/releaseAssets/latestRelease`
- [x] `store.go` 索引 `sourceData.platforms`、`sourceData.latestRelease.publishedAt`
- [x] 一轮回填(高星段已回填;全库随每日抓取铺满)

## 目录 API + 前端(v1 §4–5)

- [x] `service/app.go` + `handler/apps.go`:`GET /apps`(os/kind/category/sort/q/page),复用 `ttlCache`,os/kind 白名单
- [x] 每平台主下载挑选(按扩展名优先级)+ homepage + license
- [x] `/apps` App 卡片页:图标(GitHub 头像)、应用名清洗、用途、按系统下载按钮、star 降级为可信标
- [x] 分类下拉(领域树,按已知域校验)
- [x] 详情页下载区(按平台列资产,直链)+ 平台徽标
- [x] 导航「应用」入口 + 8 语言

## 开源替代(v2 → 提前并入)

- [x] `service/alternatives.go`:严格批量 prompt + slug 化;整批失败留待重试
- [x] `job/altfinder.go`:app 语料按星标降序推断,顶层存 `alternativeTo`/`altStatus`(抗抓取覆盖),run 守卫
- [x] `domain.Alternative` + `TrackedItem.AlternativeTo/AltStatus`;`main.go` `POST /internal/alt-find`
- [x] `GET /alternatives`(产品按替代数排)+ `GET /alternatives/:slug`;索引 `alternativeTo.slug`
- [x] `/alternatives` 索引页 + `/alternatives/[slug]` 落地页(SEO 入口)
- [x] 卡片 + 详情页「↔ 开源替代 X」交叉链接;导航「替代」入口

## 后续(未做,留待独立)

- [ ] 全库平台数据铺满后按 Web 启发式误报率收紧规则
- [ ] alt-find 定时 cron(当前手动/admin 触发);全库推断铺满
- [ ] 消费者用途分类(区别于 dev 领域树)
- [ ] App 截图(README/OpenGraph 提取)
- [ ] 接商店 API 补非 GitHub 发行的下载渠道

## 决策偏差(与原 design 的出入)

- **v2 的"开源替代"提前到本轮做了**——用户要求先做效果,质量验证后一并上线,故 v1/v2 边界模糊化,统一归档。
- **详情页下载区从 v2 提前到 v1**(用户确认)。
