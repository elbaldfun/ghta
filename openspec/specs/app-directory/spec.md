# app-directory Specification

## Purpose

将已抓取的 GitHub 仓库中「可下载运行的开源软件」聚合成面向消费者的应用目录:按操作系统筛选、按热度/星标/最新发行排序,并推断每个应用是哪些商业付费软件的开源替代(AlternativeTo 式落地页)。区别于开发者视角的仓库排行,本能力以「应用」为主体、以「下载」与「找免费替代」为核心意图。

## Requirements

### Requirement: 平台探测

系统 SHALL 在抓取每个仓库时探测其支持的平台,归一化为 `{macos, windows, linux, android, ios, web}` 子集,存于 `sourceData.platforms`,并记录来源 `platformSource ∈ {asset, topic, heuristic}`。探测 SHALL 以最新非 draft release 的资产文件名为权威依据,topic 为兜底,Web 启发式为最后手段。附属文件(签名、校验和、更新清单)SHALL 被忽略;归档文件(.zip/.tar.*)SHALL 仅在含显式 OS 关键词时计入。

#### Scenario: 资产解析

- **WHEN** 一个 release 含 `App.dmg`、`App-setup.exe`、`App.AppImage`
- **THEN** platforms = [macos, windows, linux],platformSource = asset

#### Scenario: 无资产走 topic 兜底

- **WHEN** 仓库无 release 资产但带 `electron` topic
- **THEN** platforms = [macos, windows, linux],platformSource = topic

#### Scenario: Web 启发式否决

- **WHEN** 仓库无资产、无平台 topic、名称含 "desktop"
- **THEN** 不触发 Web 启发式(避免把桌面应用误判为 Web)

### Requirement: 应用目录查询

系统 SHALL 提供 `GET /apps`,接受可选 `os`、`kind(app|cli)`、`category`、`sort(hot|popular|new)`、`limit`、`page`,返回符合「app/cli 形态 或 有平台,且非 library」的条目。`os`/`kind` SHALL 白名单校验;结果 SHALL 缓存(compute 出锁、singleflight、键有界)。每条 SHALL 附带每平台主下载链接、license、homepage、平台集合与(若有)开源替代。

#### Scenario: 按系统筛选

- **WHEN** 客户端请求 `/apps?os=macos`
- **THEN** 仅返回 platforms 含 macos 的应用

#### Scenario: 最新发行排序

- **WHEN** 客户端请求 `/apps?sort=new`
- **THEN** 按 `sourceData.latestRelease.publishedAt` 倒序返回

### Requirement: 开源替代推断

系统 SHALL 通过 LLM 推断每个应用是哪些商业/付费产品的开源替代,存于**顶层** `alternativeTo: [{name, slug, kind}]`(顶层以免被抓取的 sourceData 覆盖),并用 `altStatus` 标记已处理。推断 SHALL 严格:仅当是同类可替代关系时给出规范产品名,否则为空;整批 LLM 调用失败 SHALL 视为「稍后重试」而非「无替代」。任务 SHALL 有并发守卫、按星标降序、可恢复。

#### Scenario: 严格推断

- **WHEN** 一个库/框架不是任何知名付费产品的替代
- **THEN** alternativeTo 为空,altStatus = done(不重复推断)

#### Scenario: 批调用失败

- **WHEN** LLM 整批调用/解析失败
- **THEN** 该批不写 altStatus,留待后续运行重试

### Requirement: 开源替代查询

系统 SHALL 提供 `GET /alternatives`(付费产品按其开源替代数量降序)与 `GET /alternatives/:slug`(替代某产品的应用,按星标降序,附产品显示名)。两者 SHALL 缓存;SHALL 有 `alternativeTo.slug` 索引支撑反向查询。

#### Scenario: 反向查询

- **WHEN** 客户端请求 `/alternatives/cursor`
- **THEN** 返回 alternativeTo.slug 含 cursor 的应用及显示名 "Cursor"

### Requirement: 商店语义层(change 15)

系统 SHALL 由 LLM 批推断为每个 app 候选产出:货架分类(53 受控 slug 或 excluded)、中/英用途一句话、hasGui 判定,存顶层 `store` 子文档(版本门控可重判;人工 categoryOverride 永远优先)。excluded SHALL 以单一 `$ne` 子句从 app 语料剔除(标记不删除)。推断 SHALL 有毒批守卫(item 级失败计数、阈值停牌)且整批调用失败视为重试而非判决。

#### Scenario: 语料清洗

- **WHEN** awesome 清单/框架经 topic 兜底混入 app 候选
- **THEN** 判为 excluded,不再出现于 /apps 及其派生页

#### Scenario: 人工纠错

- **WHEN** 管理端设置 store.categoryOverride
- **THEN** 货架筛选与展示以 override 为准,后续 LLM 重判不覆盖它

### Requirement: 货架浏览与中文搜索

`GET /apps` SHALL 支持 `shelf=`(完整 slug 或大类前缀,白名单)按生效货架筛选;卡片 SHALL 以用途 tagline(zh 语言用中文)为主描述。CJK 搜索查询 SHALL 子串匹配 store.taglineZh+名称+描述(Mongo $text 无 CJK 分词),按星标排序。

#### Scenario: 中文功能词搜索

- **WHEN** 用户搜索「下载」
- **THEN** 结果含 yt-dlp 等 taglineZh 命中的下载工具

### Requirement: 质量分合集(试水)

系统 SHALL 提供 `GET /apps/best/{shelf}`(白名单试水货架):按可解释质量分(ln 星标 + 2·ln 日增 + 90 天内发行加成 + 有安装包加成)排序,top-12 封顶。合集页 SHALL 含手写导语与方法论说明;扩展受 90 天 GSC 数据门控。

#### Scenario: 质量分非纯星标

- **WHEN** 一个低星但高增速、近期发行、有安装包的应用与高星但停滞的应用同货架
- **THEN** 质量分可使前者排位高于纯星标排序

### Requirement: 截图管线(展示层关闭)

系统 SHALL 持续提取应用截图(README 首图启发式 + 尺寸校验 + og:image 兜底,标注来源),API 输出经 CN 可达镜像改写。**展示层当前关闭**(审计精确率 80% 未达上墙线且展示形态未获认可);数据留存待展示方案定稿。

#### Scenario: 来源门控

- **WHEN** 截图来源为 og(营销卡概率高)
- **THEN** 该图不具备卡片墙资格,仅可用于未来的详情页兜底
