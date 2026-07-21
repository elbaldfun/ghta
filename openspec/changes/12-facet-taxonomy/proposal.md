# Change: 多维分类（领域主树 + 正交 facet + 领域层多标签）

## Why

change 9 把分类树冻结成受控资产、用三级流水线归类，解决了语义漂移和成本结构。但它沿用了
**单一分类树**这个前提，而这棵树里混了三种性质不同的维度，被硬塞进同一层级：

- **领域**（干什么用的）：`ai`/`web`/`data`/`infra`… —— 这才是真正的分类主轴；
- **技术栈**（用什么造的）：`lang`（编程语言本身）—— 语言是横切属性，不该与领域平级；
- **形态**（是什么东西）：`learning`（教程/清单/面试）—— 一个 React 教程既是 `web` 又是
  `learning`，被迫二选一。

由此产生两个结构性问题：

1. **多领域仓库被压碎**：一个「Rust 写的 K8s CLI」跨 `lang`/`devtools`/`infra`。规则层还能多标签，
   但 embedding/LLM 层只取 top1，信息直接丢失。
2. **维度混淆**：浏览/筛选时无法回答「所有 CLI 工具」「所有 Rust 项目」这类正交问题——它们散落在
   各领域子树里。

另有可量化但当前无数据支撑的问题：change 9 的 golden set / eval（tasks 3.3）一直是 TODO，
任何调树/调阈值都是盲调。

## What Changes

一个仓库产出 **一个必选领域主类（可多标签）+ 若干正交 facet**：

- **领域主树（domain）**：重构后的受控 2 级树，作为排行榜与浏览的主轴。**移出两个非领域大类**——
  `lang` 降为 `tech` facet、`learning` 降为 `type` facet；重排失衡的枝（`ai` 拆 `media-gen`、
  `data` 吸收 `cache` 并增 `analytics`、补 `devtools/ai-coding` 等），目标叶子条目数量级均衡。
- **type facet**（形态）：`library`/`framework`/`app`/`cli`/`tool`/`tutorial`/`awesome`/
  `dataset`/`plugin`…，单值或少量，主要由 topics/命名规则确定。
- **tech facet**（技术栈）：主语言 + 关键框架（react/pytorch/docker…），多值，由主语言与 topics
  直接映射，**免费、确定性、天然多标签**。
- **领域层放开多标签**：embedding/LLM 从 top1 改为 **top-K（所有 ≥ 阈值/被判定命中的都收）**。
- **facet 走独立规则表**，不进 embedding/LLM，成本几乎为零。
- **embedding 向量持久化**（现为即算即弃），为相似仓库推荐 / 语义搜索铺路（对齐 project.md
  「分析/搜索扩展走旁路」的既定方向）。
- **先建 eval 再动树**：补 change 9 欠下的 golden set + eval 脚本，调树/阈值前后必须重跑对比。
- 查询 API 增 facet 组合筛选（`type`、`tech`），前端筛选从「单树下钻」改为「领域树 + type/tech 多选」。

## Impact

- Affected specs: `ai-categorization`（流水线按维度分流、领域层多标签、向量持久化）、
  `category-management`（领域树资产 + facet 资产模型）、`trending-query`（facet 组合筛选）
- Affected code (Go)：
  - `taxonomy/`：`taxonomy.yaml` 重构为领域树；新增 `facets.yaml`（type/tech 枚举与映射）；
    `topic-map.yaml` 拆为领域映射与 facet 映射两段
  - `internal/domain`：`TrackedItem` 增 `type string`、`tech []string`；`categoryId/categoryPath`
    语义收窄为「领域主类」
  - `internal/taxonomy`：加载/同步 facet 资产；facet 规则映射
  - `internal/job/categorizer.go`：facet 规则层前置；领域层 top-K 多标签；向量持久化
  - `internal/service`：embedding 向量落库；查询服务支持 facet 筛选
  - `internal/handler` + `api/openapi.yaml`：`/trending` 增 `type`/`tech` 参数
  - 新增 `cmd/eval`：golden set 命中率/准确率评估
  - 前端：组合筛选 UI
- 迁移：老的 `lang/*`、`learning/*` categoryPath 保留不删；重跑一次分类自然收敛到新树并补 facet
  （沿用 change 9「存量不删、重跑收敛」）
- **BREAKING**：`categoryPath` 语义变化（仅表示领域主类）；查询返回体新增 `type`/`tech` 字段
