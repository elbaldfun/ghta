# Design: 12-type-facet-multilabel

前提：建立在 change 9（固定分类树 + rule/embedding/LLM 三级 + 建议队列）之上，不推翻流水线骨架。
范围限定 GitHub 单源，不为后续源（App Store 等）预留分类结构。

## 维度模型：领域主树 + type facet

```
tracked_item
  ├─ categoryId[] / categoryPath   ← 领域树 domain（必选，可多标签）  "干什么用的"
  ├─ type        string            ← 形态 facet（单值）              "是什么东西"
  └─ sourceData.topicNames[]       ← 原始 GitHub topics（已有，本次暴露为筛选参数）
```

评审结论（记录砍掉的分支，防止翻烧饼）：

- **tech facet 不做**：语言筛选已有 `language` 参数；框架筛选用原始 topics 暴露即可覆盖。
  tech 的唯一增量是同义词归一化（react/reactjs），不值一套资产+字段+映射。真痛了再立项。
- **向量持久化不做**：与分类质量正交，属于「相似推荐/语义搜索」的地基，遵循 project.md
  「旁路、需要时加」，独立成章。

## type facet（facets.yaml）

```yaml
# 单值，按优先级取第一个命中；全部未命中 → software 兜底
type:
  - { key: awesome,   name: 资源清单,  topics: [awesome, awesome-list] }
  - { key: interview, name: 面试准备,  topics: [interview, interview-questions, leetcode] }
  - { key: tutorial,  name: 教程,      topics: [tutorial, course, roadmap, book, learning] }
  - { key: cli,       name: 命令行工具, topics: [cli, command-line, terminal] }
  - { key: app,       name: 应用,      topics: [app, desktop-app, self-hosted] }
  - { key: library,   name: 库/框架,   topics: [library, framework, sdk] }
  - { key: software,  name: 软件项目 }   # 兜底
```

- **单值 + 优先级**：资料类（awesome/interview/tutorial）优先于软件形态——产品上最重要的切分是
  「资料 vs 软件」，让 type=software/cli/app 的榜单干干净净。枚举与线索**人审定稿**。
- 纯查表、免费、不依赖外部 API；topics 为空时直接落兜底，不进 LLM。
- 单值是刻意求简：既是 app 又是 library 的场景先不管，eval 暴露痛点再考虑多值。

## 领域树微调（taxonomy.yaml，人审）

- **删 `learning`**：tutorials/awesome-lists/interview 全部下放为 type。资料类仓库的**领域**照常
  归类（React 教程 → domain=web/frontend + type=tutorial），领域树不再为形态维度留大类。
- **`lang` 语义收紧**为「语言基础设施」：`implementations`（编译器/解释器/运行时）+
  `utils`（通用工具库与算法实现）。评审曾提议删除 stdlib-utils，但完全删除会让 lodash 这类
  无明确领域的通用库无家可归，故保留为 `lang/utils`；关键改变是 **topic-map 不再按语言路由**——
  一个 Go 的 HTTP 库靠多标签进 `web/backend`，而不是被语言兜底拽进 `lang`。
- 粒度均衡：`ai` 拆 `media-gen`（图像/视频生成，从 cv 分离）；`data/cache` 并入 `data/database`、
  增 `data/analytics`；补 `devtools/ai-coding`（Copilot 类）。其余枝保持稳定，改动最小化。

### 树深度决策：维持两级，不做全树三级

评审结论（记录在案，防翻烧饼）：**不加第三级**。理由：当前数据量下三级叶子会稀疏到撑不起
榜单页；叶子数翻 3 倍会拖垮三级流水线的准确率（embedding 语料区分度下降、LLM 混淆、rule
表维护量爆炸）；且三级想表达的细分（`ai/llm/rag`、`web/frontend/react`）几乎全是 topic/
language/type 已覆盖的正交维度——硬塞进树就是把本 change 拆掉的维度混淆请回来。二级树 +
facet 组合的表达力等价于虚拟三级（`ai/llm + topic=rag` ≡ `ai/llm/rag`）。

实现层面加深是零代码改动（Load/Sync/buildTree 均递归、categorizer 对深度无假设），属可逆
决策。**局部**拆某叶子为三级的触发条件（三者齐备才拆，且只拆该叶子）：

1. 该叶子条目占全库比例失衡（> ~8-10%），浏览页失去筛选意义；
2. 建议队列反复出现该叶子下的细分 path；
3. eval 基线已建立，拆分后可量化确认准确率未回退。

## 流水线：type 前置，领域多标签

```
pending item
  ├─ type 层（确定性，先跑，永远可用）
  │    └─ facets.yaml 优先级匹配 → type（单值，兜底 software）
  └─ domain 层（语义，三级，前级命中即停）
       ├─ ① rule       topic-map 命中 → 多标签全收（现状保持）
       ├─ ② embedding  余弦 ≥ 阈值的叶子全收（top-K），上限 DOMAIN_MAX_LABELS
       ├─ ③ LLM        兜底长尾；prompt 允许返回多个 path（≤ DOMAIN_MAX_LABELS）
       │                isNewCategory → 建议队列（同 change 9，不变）
       └─ 全部失败 → 失败计数（同 change 9，不变）
```

- `DOMAIN_MAX_LABELS` 默认 3，走 config；防过度打标由 eval 的 precision 监控兜底。
- `classifiedBy` 语义不变（记录领域主类由哪级判定）。
- embedding 仍即算即用（不落库）；无 embedding 后端时领域退化为 ①+③，type 层不受影响。

## 评估（先建再动树）

- `taxonomy/golden.yaml`：人工标注 200+ 条目，每条 `domain[]` + `type`。
- `cmd/eval`：输出领域各级命中率、多标签 precision/recall、type 准确率。
- 纪律：改 taxonomy/facets/阈值前后必须重跑 eval 对比。基线数值在动树**之前**先记录。

## 迁移

- `learning/*` 分类文档保留（不删、不破坏历史 categoryPath），仅从归类目标移除。
- 分批重跑：先跑一批经 eval 校准，再全量；领域收敛到调整后的树，同时补全 type。
- 前端：type chip（软件/教程/清单…快切）+ topic 输入筛选；默认榜单可考虑排除资料类
  （产品决策，前端任务里定）。

## Trade-offs

- type 单值会错杀少数双形态仓库 → 求简优先，eval 数据说话。
- 保留 `lang/utils` 与「维度纯化」有张力 → 实用主义：无家可归比维度不纯更伤浏览体验。
- 教程类仓库领域标注可能变难（topics 常缺）→ 资料类的领域归类允许落空（只有 type），
  不计入失败升级；eval 单独统计资料类的领域覆盖率。
