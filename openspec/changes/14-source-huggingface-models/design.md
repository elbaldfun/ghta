# Design — HuggingFace 模型排行

## 1. 产品定义

**一句话**:「AI 模型的 StarRank」——哪个模型此刻在被大量采用、哪个在快速升温,按任务(文生文/文生图/语音/嵌入…)筛,速度优先。

面向的问题:HF 官方只给绝对下载数和不透明 trending;用户想知道的是**动量**("过去一周谁真正起飞了")和**历史曲线**——正是我们已有的快照体系擅长的。

## 2. 语料边界(有界抓取)

HF 有 200 万+ 模型,绝大多数无人问津。语料取**三个榜的并集**,每日刷新:

```
corpus = top-N by downloads  (N=20,000)
       ∪ top-N by likes      (N=10,000)
       ∪ top-N by trendingScore (N=5,000)     # 捕捉「新爆但还没量」的
```

- 实测 top-1000 by downloads 的门槛已是 36 万+/月,top-20k 门槛预计在数千/月——足以覆盖一切有意义的模型;
- **进得来、出不去**:一旦入语料就持续快照(掉出榜单的模型停止更新指标,但历史保留)——与 GitHub 源"star 掉回 1000 以下仍保留"同策略;
- 预估稳态语料 **2–4 万条**,快照量约为 GitHub 源(6.8 万)的一半。

私有(`private`)与禁用(`disabled`)模型排除;`gated`(需申请)模型**保留并打标**——它们(Llama 系)是重要玩家,前端标注「需授权」。

## 3. 指标语义(本设计最关键的决策)

HF 与 GitHub 的指标语义不同,**不能照搬 star 逻辑**:

| 指标 | 语义 | 怎么用 |
|---|---|---|
| `likes` | **累计值**(同 star) | 快照 → 日/周增量 = **热度速度**,与 star 增速完全同构,复用现有 daily/weeklyIncrease 机制。**排行主轴。** |
| `downloads` | **近 30 天滚动窗口**(非累计!) | 作为 gauge 存快照。它本身就是"采用度的移动平均"——**直接按它排 = 「当前采用榜」**;其日变化 = 采用加速度(噪声大,v1 只展示不排序)。 |
| `trendingScore` | HF 官方热度(算法不透明) | 抓取入语料的信号之一;前端不展示(不可解释)。 |

**为什么 likes 增速做速度主轴而不是 downloads**:downloads 被 CI/自动化灌水严重且是滚动窗,日环差噪声大;likes 是人的主动行为,干净、可比、和 star 完全同构。**这正好也是与 HF 官方榜的差异化**(HF 不提供 likes 历史/速度)。

映射到 TrackedItem:

```
source          = "huggingface"
externalId      = "deepseek-ai/DeepSeek-R1"
primaryMetric   = "downloads30d"
metrics         = { downloads30d, likes }
dailyIncrease 等 = likes 的增量(由现有 metrics job 从快照算)
sourceData      = { author, pipelineTag, library, tags, languages,
                    gated, createdAt, lastModified, license(从tags解析),
                    quantFormats(gguf/awq/mlx… 从tags解析) }
```

## 4. 任务分类:复用 pipeline_tag,零 LLM

HF 的 `pipeline_tag` 是官方任务标签(~40 个),直接收敛成 ~10 个任务组,一个静态映射表搞定:

| 组 | 收纳的 pipeline_tag(举例) |
|---|---|
| 文本生成 | text-generation, text2text-generation |
| 多模态 | image-text-to-text, visual-question-answering, any-to-any |
| 图像生成 | text-to-image, image-to-image, unconditional-image-generation |
| 视频 | text-to-video, image-to-video |
| 语音 | automatic-speech-recognition, text-to-speech, audio-* |
| 嵌入/检索 | feature-extraction, sentence-similarity |
| 视觉理解 | image-classification, object-detection, image-segmentation… |
| 传统 NLP | text-classification, token-classification, translation, summarization, QA |
| 强化学习/机器人 | reinforcement-learning, robotics |
| 其他 | 剩余长尾 |

- **不进 LLM 分类管线**(analysisStatus 对 HF 源直接标 done):官方标签比 LLM 推断更权威,零成本零错误;
- 现有 `categoryPath` 存任务组(如 `hf/text-gen`),与 GitHub 领域树**平行不混合**——两源的分类语义不同,不强行合树。

## 5. 抓取

```
internal/source/huggingface/
  client.go    # GET /api/models,游标分页(Link header),限速 ~4 req/s,UA 标识
  adapter.go   # 实现 source.Fetcher:Shards() 返回三个榜的分片;Fetch() 拉一片并归一化
```

- **分片策略**:`downloads:p1..p200`、`likes:p1..p100`、`trending:p1..p50`(每页 100 条)——复用 fetch_runs 的分片断点续跑;
- 全量一轮 ≈ 350 请求,几分钟级,对 HF 无压力;无需 token(公开 API),预留 `HF_TOKEN` 配置以防未来限流;
- **每日 cron** 与 GitHub 抓取错峰(如 04:30 UTC),抓完触发 metrics job(现有机制,按源跑);
- 列表 API 的字段已够用,**不逐模型打详情接口**(v1 不需要 cardData/siblings)。

## 6. 查询 API

```
GET /models?task=&sort=hot|downloads|likes|new&q=&page=&limit=
```

| 参数 | 说明 |
|---|---|
| `task` | 任务组白名单(§4 的 ~10 组);空=全部 |
| `sort` | `hot`=likes 日增速(默认,速度主角)· `downloads`=近30天下载(当前采用)· `likes`=累计点赞 · `new`=createdAt 倒序(新发布) |
| `q` | 文本搜索(HF item 同样进 §搜索 的 text 索引,零额外工作) |

实现:`internal/service/models.go`,复用 `ttlCache`(compute 出锁 + singleflight + 键白名单),board 上限 300 分页——与 apps/ecosystem 完全同模式。

索引:`{source:1, categoryPath:1, dailyIncrease:-1}` 与 `{source:1, "metrics.downloads30d":-1}`(现有 source 索引 + 新增复合)。

## 7. 前端

- **`/models` 榜单页**:任务组 tab(横滑)+ 排序切换(热度/下载/点赞/最新)+ 密集行;
- **ModelRow**:排名 · 模型名(author/model)· 任务组 chip · 量化格式 chip(GGUF/AWQ,本地部署者最关心)· `gated` 锁标 · **likes 日增(热度色)** · 近30天下载 · likes;点击**外链 HF 模型页**(v1 不做站内详情——模型详情的核心是 model card,HF 已做得最好,别重复);
- **导航**:放 **AI ▾ 下拉**内(AI 生态 / AI 热点 / **模型**)——不新增顶级项,守住 6 项终态;
- **首页不动**:GitHub 榜仍是主页;模型是 AI 专区的一块。
- 8 语言文案。

## 8. 决策记录

| 决策 | 结论 | 依据 |
|---|---|---|
| 速度主轴 | **likes 增速**(非 downloads 环差) | downloads 是滚动窗+可灌水;likes 干净且与 star 同构,直接复用增量机制 |
| downloads 用法 | gauge 直排「当前采用榜」 | 30 天滚动窗本身就是移动平均,直接排序即有意义 |
| 分类 | pipeline_tag 静态映射,零 LLM | 官方标签更权威;省钱省错;与 GitHub 领域树平行不混 |
| 语料 | 三榜并集 + 进得来出不去 | 有界(2–4 万)、不漏新爆款、历史连续 |
| 站内详情页 | **v1 不做**,外链 HF | model card 是 HF 的主场;我们的价值在榜与趋势 |
| 导航 | AI 下拉内,不加顶级项 | 守住 6 项终态 |
| gated 模型 | 保留 + 打标 | Llama 系是关键玩家,排除会失真 |
| 数据集/Spaces | **v1 不做** | 同 API 形态,验证模型榜有人用再扩,避免一次铺太大 |

## 9. 边界与降级(诚实清单)

- **downloads 可灌水**(CI/自动化):前端下载数标注"近30天";不用它做速度主轴已规避大半;刷量检测是未来「刷 star 检测」能力的同族问题,本 change 不解决;
- **likes 稀疏**:多数模型 likes 很低,增速榜长尾会平;板凳深度靠 downloads 榜补;
- **历史自增长**:与 star 一样,曲线从上线日开始积累,无回填来源——如实展示短历史;
- **trendingScore 不可解释**:只用于入语料,不对用户展示;
- **HF API 无 SLA**:失败走 fetch_runs 断点重试(现有机制);连续失败告警日志。

## 10. 分期

**v1(本 change)**:
1. HF client + adapter + 单测(真实 API fixture)
2. 语料抓取 + 快照 + likes 增量(复用管线)+ 每日 cron
3. `GET /models` + `/models` 页 + AI 下拉导航 + 8 语言
4. HF item 进搜索 text 索引(顺带)

**v2(验证后)**:
- 站内模型详情页(likes/下载历史曲线——攒了快照才有东西可画)
- 数据集榜 / Spaces 榜(同 API 形态)
- GitHub 仓库 ↔ HF 模型交叉链接(arxiv/github tag 解析)
- 模型作者榜(并入开发者体系)

## 11. 开放项 — 已全部确认(2026-07-29)

1. ✅ **语料阈值**:三榜 20k/10k/5k,稳态 2–4 万条。
2. ✅ **导航**:AI ▾ 下拉内(AI 生态 / AI 热点 / 模型),不新增顶级项。
3. ✅ **默认排序**:`hot`(likes 增速)——与全站「速度是主角」一致。
4. ✅ **数据集/Spaces**:v1 不带,验证模型榜后再扩。

> 设计定稿,开工。
