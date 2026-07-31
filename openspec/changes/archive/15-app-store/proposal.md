# Change: 开源应用商店(App Store 化)

> **状态:按三方评审修订版(15′)交付并上线,归档于 2026-07-31。** v2a 数据层 / v2b 货架前端 / v2c 五页合集试水完成;v2d 语义搜索移出;截图管线在跑但展示层经用户裁决关闭。能力规范见 `openspec/specs/app-directory/spec.md`(商店语义层等新增节);完成清单与偏差见本目录 `tasks.md`。

## Why

change 13 交付的 `/apps` 是「带下载按钮的排行榜」,不是应用商店。用真实语料(app 语料 17,907 条)对照诊断,五个差距按重要性:

1. **分类是开发者视角**(ai/agents、devtools 领域树),不是消费者货架(笔记/浏览器/截图录屏/密码管理);
2. **没有截图**——应用商店的灵魂是看到界面;
3. **描述是 GitHub 仓库描述**(英文 dev 一句话),不是「这个应用帮你做什么」;
4. **发现路径单一**——没有货架浏览、没有合集;
5. **搜索断层**——语料全英文,中文用户搜「截图」找不到 flameshot;其余 6 语同理。

另有一个语料质量问题(top 60 抽样实证):**topic 兜底把非应用混进了语料**——`awesome-mac`(清单,因 macos topic 得到平台)、`react-native`/`electron`/`node`(框架)、`system-design-primer`(面试资料)都在 app 语料里。货架上不能摆面试题清单。

## What Changes

- **消费者货架分类** `appCategory`:11 大类 46 子类 + `excluded`(非应用垃圾桶),受控 slug 枚举,显示名走前端 i18n(**8 语言全量**);树已用 top-60 + 中部随机 30 的真实语料逐条验证覆盖(~95%+)。
- **LLM 批推断一次产出四样**(复刻 altFinder 成熟模式:批 prompt/顶层字段抗覆盖/整批失败重试/每日 cron):
  `appCategory`(货架)+ `tagline`(中/英用途一句话)+ `hasGui`(校正 type 误标,如 Windows Terminal 被标 cli)+ `excluded`(清洗语料)。
- **截图**:README 第一张「真截图」(启发式滤 badge)→ 官网 og:image 兜底;热链存 URL 不建图床;复刻 iconfetcher 模式。
- **商店化前端**:`/apps` 重构为商店首页(热门横条 + 分类货架横滑 + 全量列表模式);卡片加截图缩略 + tagline;仓库详情页加「应用」区块(截图轮播 + 同类推荐);**CLI 按用途混排进货架**(yt-dlp 属于下载工具货架,挂 CLI 徽标)——不排除。
- **中文搜索接通**:tagline_zh 子串匹配(app 语料 1.8 万条、短字段,毫秒级;Mongo $text 无 CJK 分词,故不走 $text)。
- **自动合集**:`/apps/best/<category>` 按质量分(star+增速+近期 release+有下载资产)自动生成——SEO 长尾入口,与 /alternatives 互为犄角。
- **多语言 embedding 语义搜索**(受 key 门槛,见 design §8):256 维向量进程内暴力余弦,零新基建;8 语言查询一套索引通吃;关键词搜索兜底。

## Impact

- Affected specs: `app-directory`(扩展为商店能力);新增搜索语义层描述。
- Affected code:
  - `internal/domain`:TrackedItem 增 `appCategory`、`taglineZh/taglineEn`、`hasGui`、`screenshotUrl`、`embedding`(均顶层,抗 sourceData 覆盖)
  - 新增 `internal/job/storefinder.go`(LLM 四合一批推断)、`internal/job/shotfetcher.go`(截图)、`internal/job/embedder.go`(向量,门槛后)
  - `internal/service/app.go`:货架/合集/语义查询;`internal/handler`:对应端点
  - `taxonomy/appshelves.yaml`(或 Go 常量):46 子类受控枚举
  - 前端:商店首页、货架页、合集页、卡片/详情升级、8 语言词条
- 语料清洗是**数据修正**:excluded 项从 app 语料剔除(不删数据,只标记),/apps 计数会下降——这是变准,不是变少。
- LLM 成本:~1.8 万 app / 批 12 ≈ 1,500 次调用,一次性;每日增量个位数。
