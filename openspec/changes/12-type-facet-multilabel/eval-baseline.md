# Eval 基线记录（tasks 1.3）

- **golden**: v1.0（2026-07-21 冻结，284 条，Q1-Q32 裁决见文件头）
- **被评估对象**: 现行流水线（change 9 的树 + topic-map + top-1 embedding/LLM），
  预测经别名表换算到新树 path 后与 golden 对比；`learning/*` 预测单独记 legacy 桶
- **环境**: 元数据来自生产 API；embedding=nomic-embed-text-v1.5(768d)@LM Studio 本机；
  LLM=qwen3.6-35b-a3b(reasoning)@LM Studio 本机；EMBED_SIM_THRESHOLD=0.35
- **复跑**: `go run ./cmd/eval [-skip-embed] [-skip-llm]`（provider env 同服务端）

## 运行 A：rule only（`-skip-embed -skip-llm`）

| 指标 | 数值 |
|---|---|
| rule 解决率 | 64%（183/284），hit@any **79%** |
| legacy-learning 桶 | 26 条（9%） |
| unresolved | 75 条（26%） |
| 全体 hit@any（n=247 有 golden 领域） | 55.1% |
| precision / recall（micro 多标签） | 0.45 / 0.51 |

## 运行 B：rule + embedding@0.35（LLM 未触发）

| 指标 | 数值 |
|---|---|
| rule | 183（64%），hit@any 79% |
| embedding | 71（25%），hit@any **31%** ⚠️ |
| legacy-learning | 30（11%） |
| unresolved | **0**（embedding 全收） |
| 全体 hit@any | 63.6%；P 0.43 / R 0.58 |
| 资料类（n=77）领域覆盖 | 65%，其中 27 条被路由进 legacy-learning |

## 运行 C：rule + LLM（`-skip-embed`，qwen3.6-35b-a3b）

| 指标 | 数值 |
|---|---|
| rule | 183（64%），hit@any 79% |
| llm | 70（25%），hit@any **71%** |
| legacy-learning | 31（11%），unresolved 0 |
| 全体 hit@any | **74.5%**；P 0.50 / R 0.66 |
| 资料类（n=77）领域覆盖 | 62%，29 条路由进 legacy-learning |

### B vs C：同一批 75 条长尾，embedding@0.35 对比本地 LLM

| 兜底层 | 该层准确率 | 全体 hit@any |
|---|---|---|
| embedding@0.35（运行 B） | 31% | 63.6% |
| qwen3.6-35b（运行 C） | **71%** | **74.5%（+10.9pp）** |

耗时参考：LLM 75 条 ≈ 20 分钟（本地 reasoning 模型，batch=8）；embedding 全程 <1 分钟。

## 基线结论（改造动作的依据）

1. **embedding@0.35 阈值形同虚设，且拖累整体 -10.9pp**：同一批 75 条长尾，
   embedding 31% 正确 vs 本地 LLM 71%；embedding 全收导致 LLM 兜底从未触发。
   生产存量约 1/4 条目的分类质量 ≈31%。
   → 动作（按数据强度排序）：a) 大幅上调阈值，只放高置信过、其余交 LLM；
   b) 若调不出「高置信区间准确率 ≳ LLM」的阈值，直接停用该层（长尾量 75/284，
   LLM 全吃的成本本地可接受）。改法由阈值扫描定，不拍脑袋。
2. **rule 层 79% 准确率有清理空间**：约 1/5 规则命中是 topic 撞名误判
   （golden 里 10 条“假命中案例”即样本）。→ 动作：topic-map 清理歧义词。
3. **recall 0.51-0.58 印证多标签改造收益**：golden 多标签占 19%，现行 top-1
   天然漏掉全部副标签。
4. **legacy-learning 26-30 条（~10%）**：资料类污染的量化实锤，type facet
   上线即消化。
5. **provider bug（已修）**：LM Studio 拒绝 response_format=json_object，
   deepseek 模式下 LLM 层此前 100% 失败；已加降级重试（provider.go）。

## 运行 D：embedding 阈值扫描（`-sweep`，rule-unresolved 集 n=67）

| 阈值 | 解决 | 命中 | 准确率 |
|---|---|---|---|
| 0.30-0.50 | 67 | 21 | 31% |
| 0.55 | 65 | 21 | 32% |
| 0.60 | 51 | 19 | 37% |
| 0.65 | 16 | 6 | 38% |
| 0.70 | 1 | 1 | 100%（无意义）|

**判决：停用 embedding 分类层。** 无任何阈值同时满足「准确率 ≳71%」且「解决量可观」；
nomic-embed 对「中文分类 desc vs 英文仓库文本」区分度不足。流水线降为 **规则 + LLM 两级**，
embedding 保留给未来语义搜索（独立 change），不进分类。符合 change 9「embedding 可缺省」原意。

## 运行 E：type facet 预验证（facets.yaml v1 草稿，topics + 命名规则，n=284）

| 口径 | 准确率 |
|---|---|
| 8 类全判 | 50% |
| **二值：资料(awesome/interview/tutorial/skill) vs 软件** | **95%** |

- 错分 ~80% 集中在软件内部细分（app/library/cli → software）：这些形态天生不在 topics 里
  （transformers/opencv 的 topics 无 "library"），白名单救不了。
- **产品核心诉求（把资料类挡出软件榜）= 95%**，规则+命名规则足够、免费。
- → 设计修正：type 拆两层——**资料判定纯规则(95%)** + **软件细分搭 LLM 便车**（LLM 判 domain 时
  同一响应返回 type，近零成本）。运行 F 实测 LLM 判 type 的准确率后定夺 cli/app/library 是否值得做筛选。

## 运行 F：LLM type 准确率（rule-unresolved 硬例集 n=75，LLM 同响应返回 type）

| 口径 | 规则+命名 | LLM |
|---|---|---|
| 8 类全判 | 50% | **80%** |
| 资料/软件 二值 | 95% | 93% |

错分零星：software→library 5、skill→library 2。这 75 条是缺 topic 信号的最难批次，LLM 仍达 80%。

**判决：type 采两层混合。**
- **资料判定**（awesome/interview/tutorial/skill）：规则+命名，95%，免费，覆盖全量。
- **软件细分**（cli/app/library/software）：规则给 50% 不足以做筛选；**搭 LLM 便车**（LLM 判 domain
  时同一响应返回 type，近零成本），硬例 80%。→ cli/app/library 可作为可信筛选器，符合「用户按分类快速找工具」。
- 成本注记：让 LLM type 覆盖 rule-domain-resolved 的那 64%，需对其额外发 LLM（这些本不进 LLM）。
  本地模型成本=时间不=钱，全量重跑可离线过夜；是否对这 64% 也 LLM-type 由 migration 策略定（见下）。

## 运行 G：后端头对头（本地 qwen3.6-35b vs grok-4.5，同 golden 284）

| 指标 | 本地 qwen | grok-4.5（中转 dragoncode.codes）|
|---|---|---|
| domain hit@any | 74.5% | 74.1% |
| LLM type exact | 80% | **93%** |
| type 资料/软件二值 | 93% | **99%** |
| 吞吐 | 3.8 条/分 | **42 条/分**（~11×，可并发）|
| 稳定性 | — | 75 条 0 报错 |

**洞察：domain 准确率与模型无关，type 与模型强相关。**
- domain 两者持平（74%）：因 64% 由规则层决定、recall 被单标签压住。**提升 domain 靠 topic-map
  清理 + 多标签 top-K（change 12 本职），非换模型。**
- type 差距明显（93 vs 80）：软件细分（cli/app/library）对模型能力敏感。若要 type 筛选可信，
  强模型有实际价值。

**后端选型结论（供全量重跑 tasks 6.3）：**
- grok-4.5 中转：质量最高 + 快 11×，全量 67k ≈ 27h（并发可压到几小时）；成本=中转额度（不可见，
  用户持 key）、稳定性待长跑验证；provider 已适配（`baseURLKey`，OPENAI_API_KEY 走 base-url 分支）。
- 本地 qwen：domain 等效、type 略低、免费但慢（12.5 天）。
- 决策取向：**domain 用规则+多标签把地基打好（模型无关）；type 若追求筛选可信用 grok，否则 qwen 也够**。

## 运行 H：改造后（新树 v2 + 清理 topic-map，grok-4.5）—— eval-first 闭环验证

| 指标 | 改造前(旧树+旧map) | **改造后 v2** | 变化 |
|---|---|---|---|
| **domain precision** | 0.50 | **0.65** | **+0.15**（核心目标：错标签少一多半）|
| domain recall | 0.66 | 0.70 | +0.04 |
| domain hit@any | 74.1% | 79.4% | +5.3pp |
| 规则层 hit@any | 79% | **87%** | +8pp（歧义规则清理见效）|
| 资料类领域覆盖 | 58% | **92%** | +34pp（删 learning，教程归真实领域）|
| legacy-learning 污染 | 32 | **0** | learning 大类移除 |
| LLM type exact | 80-93% | 84% | 稳定（99% 资料/软件二值）|

改造动作：删 learning 大类、lang/utils 改名、新增 media-gen/analytics/ai-coding 叶子；topic-map
删 23 条歧义规则（平台词/用了≠是/过宽AI词）+ 重映射 media-gen；均 grok 跑。**precision 0.50→0.65
是最硬的收益**——domain 筛选变干净，直接兑现「用户按分类快速找对仓库」。改造后 domain 上限已不
在模型（grok/qwen 同分），而在规则质量 + 多标签，本轮两者都动了。

**观测：grok 批次漏项，且强相关于批大小。** 本地 40 条真实试跑实测：batch=8 漏 43%（6/14），
batch=5 漏 5%（1/20）。已加 prompt 硬约束「为每个 id 返回一条、不得省略」。
**全量迁移（6.3）必须用 batch ≤ 5**；漏掉的非资料类项计一次失败、后续运行重试（3 次上限）。

**本地端到端试跑（trial，已清理）暴露并修复 2 个 bug：**
1. 父分类可被分配：LLM 返回 "ai"/"lang" 等父路径被 resolveIDs 接受 → 改 loadTaxonomy 只索引叶子
   （`TestParentPathDropped` 锁住）。
2. grok 漏项 43% → prompt 强约束 + 小批次降到 5%。

**已修生产 bug（本轮暴露）：** `internal/service/ai.go` + `cmd/eval` 的 JSON 解析只认
`{"results":[...]}`，grok 偶发裸数组 `[...]` 会整批丢弃 → 已加裸数组兜底（build+test 通过）。

> 纪律：此后任何 taxonomy/topic-map/阈值/模型改动，改前改后各跑一次 eval，
> 数字对比贴进对应 change/PR。
