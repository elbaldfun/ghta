# Design — 开源应用商店

## 1. 产品定义

**一句话**:把 `/apps` 从「带下载按钮的排行榜」变成「开源应用商店」——按用途货架逛、看截图、读一句话用途、按你的系统下载;想要什么直接搜(任何语言、按意思搜)。

用户找 app 的四条路径与承接(设计基准):

| 路径 | 用户心里想的 | 承接 |
|---|---|---|
| 功能明确 | 「我要个截图工具」 | 搜索:中文子串(v2a)→ 语义搜索(v2d) |
| 替代付费软件 | 「Notion 的免费替代」 | /alternatives(已有) |
| 模糊逛 | 「mac 上有什么好东西」 | 货架 × OS 筛选(v2b) |
| 听说过名字 | 「localsend 在哪」 | 搜索联想(已有) |

## 2. 货架分类树(已定稿)

11 大类 46 子类 + excluded。**slug 是受控枚举、永不改名**;显示名走前端 i18n(8 语言全量翻译,一次性 ~46 词条)。树用真实语料验证过(top 60 + 5k–20k 星随机 30,覆盖 ~95%+):

```
productivity  效率办公:notes 笔记 · todo 待办日历 · docs 文档/PDF · knowledge 知识管理
creative      创作设计:image 图像编辑 · video 视频剪辑 · audio 音频 · whiteboard 绘图白板 · cad CAD/3D · writing 写作
devtools      开发工具:editor 编辑器/IDE · terminal 终端 · api API工具 · database 数据库客户端 · git Git工具 · pkg 包管理构建 · devops 容器/DevOps · gamedev 游戏引擎
ai            AI 应用:assistant AI助手 · coding AI编码 · image-gen AI绘图 · media-gen AI音视频 · local-llm 本地推理 · platform AI平台/RAG
selfhosted    自托管:cloud 个人云/同步 · photos 照片/媒体库 · media-server 媒体服务器 · monitoring 监控面板 · tunnel 内网穿透/反代 · automation 自动化工作流
network       网络:browser 浏览器 · download 下载工具 · proxy 代理 · remote 远程桌面
media         影音:player 播放器 · music 音乐 · reader 阅读器
social        通讯社交:chat 聊天 · mail 邮件 · social-tools 社交工具
system        系统工具:screenshot 截图录屏 · launcher 启动器/剪贴板 · files 文件管理 · transfer 文件传输 · phone 手机管理/投屏 · backup 备份恢复
security      安全:passwords 密码管理 · privacy 加密隐私 · adblock 广告拦截 · pentest 安全工具/渗透
games         游戏娱乐:games 游戏 · emulator 模拟器 · smart-home 智能家居
excluded      非应用:框架/库/运行时/资料清单/纯在线服务 → 不上架
```

验证时撞出并已纳入的缺口:自托管(n8n/immich/uptime-kuma)、手机管理(scrcpy 147k)、PDF(Stirling-PDF)、包管理(uv/nvm)、游戏引擎(godot)、CAD(LibreCAD)、AI 平台(dify/ragflow)、AI 音视频(MoneyPrinterTurbo)、安全工具(sherlock)。

## 3. LLM 四合一批推断(storefinder,v2a 核心)

完全复刻 altFinder 的成熟模式(批 prompt / 顶层字段抗抓取覆盖 / 整批调用失败=留待重试 / run 守卫 / 按星标降序 / 每日 cron 补增量):

**输入**:externalId + name + description + topics + 领域分类 + platforms
**输出**(严格 JSON,每项):

```jsonc
{
  "category": "system/screenshot",   // 46 子类 slug 或 "excluded"
  "taglineZh": "跨平台截图与标注工具",  // ≤20 字,说人话,讲用途
  "taglineEn": "Cross-platform screenshot & annotation tool",
  "hasGui": true                      // 校正 type 误标(Windows Terminal 案例)
}
```

**落库**(TrackedItem 顶层):`appCategory`、`taglineZh`、`taglineEn`、`hasGui`、`storeStatus`(done 标记)。

**语料清洗语义**:`excluded` 不删数据,只使 app 语料过滤条件变为 `appCategory 存在且 ≠ excluded`(过渡期:未处理的沿用旧规则,处理完收紧)。

**成本**:17,907 条 / 批 12 ≈ 1,500 次调用一次性;每日增量个位数批次。

## 4. 截图(shotfetcher,v2b)

- **主路**:README 提第一张「真截图」——从 markdown/HTML 提 img,过滤徽章与装饰:域名黑名单(shields.io/badge*)、扩展名 svg 排除、URL 含 badge/logo/icon 排除,取剩余第一张;GitHub 相对路径补全为 raw URL;
- **兜底**:官网 `og:image`(iconfetcher 已有抓官网管线,顺手);
- **存储**:热链 URL(`screenshotUrl` 顶层),不建图床;前端 onError 隐藏容器(永不摆破图);
- 复刻 iconfetcher:限速/可恢复/守卫/每日 cron;`log` 覆盖率。

## 5. 商店前端(v2b)

- **`/apps` 商店首页**:热门/新发行横条 → 分类货架(每大类横滑 8 张卡,「查看全部」进子类列表)→ 保留列表模式入口;OS 筛选贯穿(对 Flathub/AlternativeTo 的差异化);
- **卡片**:截图缩略(16:10,无图回退纯 icon 布局)+ icon + 应用名 + taglineZh(locale=zh)/taglineEn + 按 OS 下载 + star/增速小字;
- **CLI 按用途混排进货架**(数据验证后的修正决策:yt-dlp 181k 属于「下载工具」货架、claude-code 属于「AI编码」)——挂 `CLI` 徽标,保留应用/命令行筛选;CLI 的 README 终端动图同样可做截图;
- **详情页「应用」区块**:截图轮播 + 同类应用(同 appCategory 按质量分 top 6)+ 已有的下载区/替代关系;
- 46 货架词条 × 8 语言进 messages。

## 6. 自动合集(v2c)

- `/apps/best/<subcategory>`:「最佳开源笔记应用」——**质量分** = star 分位 + 增速分位 + 近 90 天有 release + 有可下载资产,权重固化可解释;
- 每子类一页,自动生成,SEO 长尾("best open source note taking app");与 /alternatives 互链;
- sitemap 收录;内容随每日数据自更新。

## 7. 搜索:分层策略

| 层 | 覆盖 | 实现 | 期 |
|---|---|---|---|
| 英文关键词 | en | 现有 $text(词干/权重) | ✅ 已有 |
| 中文功能词 | zh | **taglineZh 子串匹配**(Mongo $text 无 CJK 分词,子串扫 1.8 万短字段=毫秒级,带语料过滤) | v2a |
| 其余 6 语 | ja/ko/de/fr/es/pt | 兜底=本地化货架浏览(分类名 8 语全量)+ 英文搜索 | v2b 隐含 |
| **全语言语义** | 全部 8 语 + 自然语言 | embedding(§8) | v2d |

验收样例:搜「截图」→ flameshot;搜「视频下载」→ yt-dlp。

## 8. 多语言 embedding 语义搜索(v2d,受 key 门槛)

**原理**:embedding 把文字变成「意思坐标」——意思相近的文字坐标相邻,跨语言同义(截图/screenshot/スクリーンショット 落同一区域)。搜索从「认字」升级为「认意思」,一套索引通吃 8 语言 + 自然语言查询(「能自动整理照片的工具」→ immich)。

**架构(零新基建,规模小到不需要向量库)**:

```
嵌入文本:taglineZh + taglineEn + 应用名 + 货架名 拼接
模型:text-embedding-3-small,dimensions=256
库:1.8万 × 256 × 4B ≈ 18MB → 存 Mongo,API 启动载入内存
查询:query → 1 次 embedding 调用 → Go 进程内暴力余弦 top-K(亚毫秒)
```

**调用形态**:
- 建库:一次性 ~10 个批请求(2048 条/请求);每日增量 1–2 请求;
- 查询:每次语义搜索 1 次调用(~100–300ms)+ LRU 查询缓存(热词零调用)+ 失败自动回退关键词搜索 + 联想下拉短路(不走语义);
- 成本:首次 ≈ $0.01,之后 < $0.01/月。

**门槛(已实测,当前全不可用)**:本地/prod OPENAI_API_KEY 均为空或占位;grok 中继仅 chat(grok-4.5);LAN LM Studio prod 不可达。**需用户提供 OpenAI key 或任一 OpenAI 兼容 embedding 端点(如 Jina 免费档)**。key 到位后 v2d 约 2 天上线;晚到不阻塞 v2a–v2c。

## 9. 决策记录

| 决策 | 结论 | 依据 |
|---|---|---|
| 货架树 | 11 大类 46 子类 + excluded,slug 冻结 | 真实语料 90 条抽样验证;slug 一旦跑批不可改 |
| 分类多语言 | slug + 前端 i18n,8 语全量 | 枚举翻译一次性,与 HF 任务组同模式 |
| tagline 语言 | 仅中/英,其余回退英文 | 逐条自由文本 ×8 语成本高;搜索由语义层(v2d)补 |
| excluded 清洗 | LLM 判定,标记不删除 | topic 兜底混入清单/框架的实证;货架不摆面试题 |
| CLI | **进货架混排 + 徽标**(推翻初稿的排除方案) | yt-dlp/claude-code/uv 数据说话;排除=下架最亮的商品 |
| hasGui | LLM 顺带产出 | Windows Terminal/Tabby 被 type 误标 cli 的实证 |
| 截图 | README 首图启发式 + og:image,热链不建图床 | 覆盖率优先、成本近零;前端破图回退 |
| 中文搜索 | taglineZh 子串,不走 $text | Mongo 无 CJK 分词($text 对中文整句成单 token) |
| 语义搜索 | 进程内暴力余弦,256 维 | 1.8 万规模下向量库是过度工程 |
| 语义搜索依赖 | OpenAI 兼容 embedding 端点,用户提供 key | 已实测现有全部端点不可用 |
| Meilisearch | 本轮不上 | 语义层直接越过分词问题;Meili 留作纯关键词体验升级的备选 |

## 10. 边界与降级(诚实清单)

- **LLM 分类会有错**:货架错放/误 excluded 存在;缓解=按星标降序跑(头部先对)+ 后续可加人工覆盖表;不承诺 100%;
- **截图覆盖率未知**:README 无图/全 badge 的 app 存在,预期覆盖 60–80%;无图卡片有降级布局,不摆破图;
- **热链风险**:GitHub raw/官网图可能 404 或防盗链;前端 onError 隐藏;
- **语义搜索质量**:256 维 + 小模型对长尾细分需求(「支持 markdown 的番茄钟」)召回有限;关键词层永远兜底;
- **excluded 边界模糊**:godot(引擎但可下载的 GUI 编辑器)这类灰区判给货架(游戏引擎),规则=「用户能下载运行的就是应用」;
- **8 语 tagline 债**:ja/ko/de… 自由文本搜索仍弱,触发条件=GA 非中英流量成气候,方案=top-2000 补翻或全量语义层已覆盖大部分。

## 11. 分期与验收

| 期 | 内容 | 验收 |
|---|---|---|
| **v2a** | 货架枚举 + storefinder 四合一批推断 + taglineZh 子串搜索 | 搜「截图」出 flameshot;/apps?category=system/screenshot 返回正确;excluded 生效(awesome-mac 不在语料) |
| **v2b** | 截图 job + 商店首页/货架/卡片/详情区块 + 46 词条 ×8 语 | /apps 呈货架形态;截图覆盖率 log 输出;CLI 徽标混排 |
| **v2c** | /apps/best/<sub> 自动合集 + sitemap | 46 个合集页可访问、进 sitemap |
| **v2d** | embedding 语义搜索(**门槛:用户提供 key**) | 搜「能自动整理照片的」出 immich;德语/日语查询可命中;API 断连时回退关键词 |

## 12. 开放项

1. ~~分类树全面性~~ → ✅ 已用语料验证并修订定稿
2. ~~tagline 语言~~ → ✅ 中/英
3. ~~截图热链~~ → ✅ 接受,前端回退
4. ~~CLI 是否进货架~~ → ✅ 进,混排 + 徽标
5. **embedding key** → ⏳ 待用户提供(OpenAI 或兼容端点);不阻塞 v2a–v2c
6. **节奏确认** → v2a → v2b → v2c 顺序,v2d 随 key 插入 —— 待最终点头
