# Tasks — 开源应用商店(修订版 15′)

状态:**已交付并上线**(2026-07-31 归档)。本 change 经三方第三方评审(产品/技术/红队)后按修订版执行:立项名义"应用语料质量 + SEO 合集",v2d 语义搜索移出。

## v2a 数据层(已完成)

- [x] `domain.StoreInfo` 子文档(category/双语 tagline/hasGui/status/failCount/version + 人工 categoryOverride)
- [x] `service/storefinder`:53 货架受控枚举 + 四合一批推断(README 摘要入 prompt);slug 白名单(非法=单项失败不猜);tagline 程序化限长
- [x] `job/storefinder`:批 8、原子 $inc failCount、阈值 5 停牌(毒批守卫;中继故障=计数非判决)、版本门控重判、每日 cron 05:30 + admin drain
- [x] 语料过滤收敛 `repository.App{Candidate,Corpus}Filter`(三处漂移→一处);excluded 单 $ne 子句;store.category 索引
- [x] provider MaxTokens=4096(截断毒批杀手)
- [x] 中文搜索:CJK 查询走 taglineZh+名称+描述子串(Mongo $text 无 CJK 分词)
- [x] admin:store-find(drain/limit)+ store-override 纠错通道
- [x] **200 条试点审计闸门**:excluded 全对、货架无错放、tagline 达标 → 用户批准全量
- [x] 全量 drain:**17,923 判定 / 6 失败 / 踢出 3,050 非应用(17%)**,中继零雪崩

## v2b 前端(已完成)

- [x] /apps 货架导航(11 大类横滑 + 子货架行,slug 端到端白名单;缓存 cap 256→1024)
- [x] 后端 ?shelf= 生效货架筛选(override 永远赢);AppItem 携带 shelf/双语 tagline/hasGui
- [x] 卡片:tagline 打头(zh 语言→中文)、货架 chip、CLI 徽标改 hasGui 裁决(修 Windows Terminal 误标)
- [x] 64 货架词条 × 8 语言

## v2c 合集试水(已完成)

- [x] 5 个手工加厚合集页(笔记/下载/截图/密码/本地推理),手写中英导语 + 可解释质量分(ln stars + 2·ln growth + 发行/资产加成,常量集中一处)+ top-12 封顶
- [x] 与 /alternatives 交叉链接;**扩不扩由 90 天 GSC 数据决定**(观察期进行中)

## 截图(shotfetcher)——做了,但展示层最终关闭

- [x] `internal/shot` 提取器:README 首图(噪声域名/词表过滤、raw@HEAD 解析、256KB 尺寸校验拒横幅小图)+ og:image 兜底(SSRF 防护、混合内容丢弃)+ 单测
- [x] job:限速/断点/守卫/每日 cron 07:00;jsdelivr CN 出口改写
- [x] 两轮审计:R1 覆盖 66% 精确 67% → 收紧词表 + og 降级(shotSource)→ R2 精确 80%(og 兜底 416→74)
- [x] 80% < 85% 上墙线 → 仅接详情页(低风险位)
- [x] **最终:用户裁决展示效果不行,详情页也撤下** → 现状:数据管线继续攒(每日 1,500),全站无截图展示;组件保留待更好的展示方案
- 上墙路径(未做):头部 top 300 人工复核,或精确率改善

## 评审条件兑现记录

三方评审(有条件批准×2 + 大幅缩水×1)的放行条件全部执行:回填机制写明(H1)、批 8+failCount+slug 校验+MaxTokens(H2)、CN 截图代理(H3)、缓存扩容+索引(H4)、过滤收敛(M3)、override+storeVersion 随 v2a(M5)、hasGui 展示裁决(M2)、200 条抽检闸门、46 页合集缩为 5 页试水、v2d 移出。

## 偏差记录

- SHOT_CRON 默认值脚本补丁静默失败 → 空 cron 表达式 → **prod 崩溃循环 ~4 分钟**;修复并吸取教训:关键赋值补丁必须带断言。
- altfinder 在 drain 期间被用户暂停(ALTFIND_CRON 设为永不触发),尚未恢复。

## 未做(留待,含移出项)

- [ ] v2d 多语言 embedding 语义搜索(评审移出;触发=站内搜索量数据;需 embedding key)
- [ ] 合集页扩展(90 天 GSC 达标后)
- [ ] 截图展示方案重新设计(数据已就绪)
- [ ] 8 语 tagline 补翻(触发=非中英流量成气候)
