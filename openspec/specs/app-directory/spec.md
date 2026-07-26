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
