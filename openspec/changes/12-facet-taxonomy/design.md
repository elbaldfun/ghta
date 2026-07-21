# Design: 12-facet-taxonomy

前提：本 change 建立在 change 9（固定分类树 + rule/embedding/LLM 三级流水线 + 建议队列）之上，
只改「分类的维度模型」与「各级分工」，不推翻流水线骨架。

## 维度模型：一主 + 多 facet

```
tracked_item
  ├─ categoryId[] / categoryPath   ← 领域主树 domain（必选，可多标签）  "干什么用的"
  ├─ type        string            ← 形态 facet     library/framework/app/cli/tool/
  │                                                  tutorial/awesome/dataset/plugin
  ├─ tech        []string          ← 技术栈 facet   语言 + 关键框架(react/pytorch/docker…)
  └─ sourceData.topicNames[]       ← 原始 GitHub topics（保留，最细检索兜底）
```

- **domain** 是浏览/排行榜主轴，语义判断，走三级流水线。
- **type/tech** 是正交 facet，确定性映射，只走规则表，不进 embedding/LLM。
- 检索是**组合筛选**：`domain=infra & type=cli & tech=rust` 精准命中「Rust K8s CLI」，
  不再逼它在一棵树里二选一。

`categoryId` 现已是数组，领域层多标签零结构改造；新增字段只有 `type`、`tech`。

## 领域树重构（taxonomy.yaml）

从树里移出非领域维度，重排失衡枝。骨架示意（**最终稿需人审**，此处仅定方向与粒度）：

```yaml
# 移出：lang → tech facet；learning → type facet
- ai:        [llm, agents, ml-framework, cv, media-gen, speech-audio, data-science]
- web:       [frontend, backend, css-ui, build-tools, static-site, api-graphql]
- devtools:  [cli, editors, vcs, testing, package-manager, ai-coding]
- infra:     [containers, ci-cd, iac, monitoring, networking, serverless]
- data:      [database, pipeline, search, analytics]        # cache 并入 database
- mobile:    [cross-platform, native]
- security:  [pentest, crypto-auth, supply-chain]
- systems:   [os, emulation, embedded]
- apps:      [productivity, media, selfhosted, browser-ext]
- gamedev:   [engines, games]
- blockchain: [protocols, contracts]                        # 降权，粒度不再扩
```

原则：每叶子条目数量级尽量均衡（现状 `ai` 6 叶偏细、`cache`/`blockchain` 偏粗）；desc 仍是
embedding 比对语料，一叶一句。变更方式不变——改 YAML + 提交 = 唯一途径。

## facet 资产（facets.yaml）

```yaml
type:                        # 形态枚举 + 判定线索（topics / 命名 / 兜底）
  - { key: tutorial,  topics: [tutorial, course, roadmap, book], name: 教程 }
  - { key: awesome,   topics: [awesome, awesome-list],           name: 资源清单 }
  - { key: cli,       topics: [cli, command-line, terminal],     name: 命令行工具 }
  - { key: library,   name: 库 }        # 默认兜底
  # framework / app / tool / dataset / plugin ...
tech:                        # 技术栈：语言直取 + 关键框架 topic 映射
  frameworks:
    react: react
    pytorch: pytorch
    docker: docker
    # ...
  # 主语言经归一化后直接进 tech（Go/Rust/TypeScript…）
```

- `type` 取单值：命中优先级（awesome > tutorial > cli > app > framework > library 兜底）。
- `tech` 取多值：主语言 + 命中的框架 topic，去重。
- 两者都是纯查表，确定性、免费；`learning` 原有的 tutorial/awesome/interview 全落到 `type`。

## 流水线：按维度分流

```
pending item
  ├─ facet 层（确定性，先跑）
  │    ├─ type ← facets.yaml 优先级匹配（单值）
  │    └─ tech ← 主语言 + 框架 topic（多值）
  └─ domain 层（语义，三级，前级命中即停）
       ├─ ① rule       topic-map(domain 段) 命中 → 多标签全收
       ├─ ② embedding  叶子向量 vs 仓库向量，余弦 ≥ 阈值的 **全收(top-K)**，向量落库
       └─ ③ LLM        仅兜底①②未达阈值的边界仓库；isNewCategory → 建议队列(同 change 9)
```

- **多标签放开**：①本就多标签；②③由 top1 改 top-K（②收所有过阈叶子，③允许返回多个 path）。
  上限设 `DOMAIN_MAX_LABELS`（默认 3）防止过度打标。
- **向量持久化**：仓库 embedding 从即算即弃改为写入 `tracked_items.embedding`（或旁路集合），
  ②复用；后续相似推荐/语义搜索直接读。叶子向量仍每次运行内存缓存。
- facet 层不依赖外部 API，永远可用；embedding 层缺后端时 domain 退化为 rule+LLM（同 change 9）。

## 评估（先建再动树）

- golden set：人工标注 200+ 条目存 `taxonomy/golden.yaml`，每条含 `domain[]`/`type`/`tech`。
- `go run ./cmd/eval` 输出：领域各级命中率与准确率、facet 准确率、多标签的 precision/recall。
- **纪律**：调 taxonomy/facets/阈值前后必须重跑 eval 对比，杜绝盲调。这是 change 9 tasks 3.3 的补课。

## 迁移

- 老 `lang/*`、`learning/*` 分类文档保留（createdBy 保持），仅从领域归类目标中移除。
- 重跑分类：领域收敛到新树，同时补 `type`/`tech`。分批重跑，先跑一批用 eval 校准再全量。
- 前端筛选器：领域树下钻 + `type`/`tech` 多选 chip；排行榜主轴仍是 domain。

## Trade-offs

- **改动面大**（domain model / 资产 / pipeline / API / 前端 + 一次全量重跑）→ 用 eval 兜底、
  分批重跑降低风险；facet 层独立可先行落地、单独验证。
- **多标签易膨胀** → `DOMAIN_MAX_LABELS` 上限 + eval 的 precision 监控。
- **向量落库增存储** → 用 `text-embedding-3-small`（1536 维）可控；先做仓库向量，叶子向量仍内存缓存。
- **type 单值可能不够**（既是 app 又是 framework）→ 先单值求简，eval 暴露痛点再考虑多值。
