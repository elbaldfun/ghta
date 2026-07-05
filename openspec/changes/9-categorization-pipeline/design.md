# Design: 9-categorization-pipeline

## 分类树的"固定"机制

- 唯一事实源是 git 里的 `taxonomy/taxonomy.yaml`；改树 = 提交 = 可 review 可回滚。
- 启动时按 path upsert 进 categories（createdBy=taxonomy）；代码里不存在任何"AI 建分类"路径。
- 初稿生成：LLM 基于库内高频 topics + GitHub Explore/App Store 品类起草，人审定（合并重复/删偏门/
  拆过大/每叶一句说明——说明文本同时是 embedding 层的比对语料）。
- 维护信号：category_suggestions 中同类建议堆积（缺分类）；单分类条目数膨胀（该拆）。

## 流水线

```
pending item
  ├─ ① topic-map.yaml 命中？ ──是──► 归类 classifiedBy=rule
  ├─ ② embedding 余弦 ≥ 阈值？ ─是─► 归类 classifiedBy=embedding
  ├─ ③ LLM 批量（change 3 管道）──► 归类 classifiedBy=llm
  │      └─ isNewCategory → 写 category_suggestions + analysisFailCount+1
  └─ 全部失败 → 失败计数（3 次进 failed，同 change 3）
```

- ②的分类向量每次运行计算一次（叶子 ~100 个，可内存缓存）；条目向量即算即用（后续可持久化复用，
  为相似仓库推荐/语义搜索铺路）。阈值走 config（EMBED_SIM_THRESHOLD，默认 0.35，凭 golden set 调）。
- Embedder 复用 openai 客户端（text-embedding-3-small）；LM Studio 亦兼容 /v1/embeddings。
  无可用 embedding 后端时②整层跳过，退化为 ①+③，管道不失败。
- 多标签保留：①可命中多个 topic → 多个分类；②③取 top1（后续可扩展）。

## 评估（本 change 内建立，最小可用）

- golden set：人工标注 200+ 条目存 `taxonomy/golden.yaml`；提供 `go run ./cmd/eval` 输出各级命中率与准确率。
- 每次调整 taxonomy/阈值/模型后重跑评估，防盲调。

## Trade-offs

- 冻结树牺牲了"自动发现新领域"的灵活性 → 用建议队列补偿，人审频率约每月一次。
- ②依赖外部 embedding API → 设计为可缺省层，不成为硬依赖。
- 存量 ai 建的分类不删（避免破坏已有 categoryPath），仅从归类目标中移除，重跑分类自然收敛到新树。
