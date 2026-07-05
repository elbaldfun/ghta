# Tasks: 9-categorization-pipeline

## 1. 分类树资产

- [ ] 1.1 生成 `taxonomy/taxonomy.yaml` 初稿（两级、~100 叶、每叶一句说明）—— 待人工审定
- [ ] 1.2 生成 `taxonomy/topic-map.yaml` 初稿（高频 topic/language → 分类 path）—— 待人工审定
- [ ] 1.3 internal/taxonomy：YAML 加载 + 启动同步进 categories（upsert by path，createdBy=taxonomy）

## 2. 流水线

- [ ] 2.1 ①规则层：topics/language 查表归类（多命中→多标签），classifiedBy=rule
- [ ] 2.2 ②embedding 层：provider 增 Embed()；分类向量缓存；余弦 ≥ EMBED_SIM_THRESHOLD 归类；
      无 embedding 后端时整层跳过
- [ ] 2.3 ③LLM 层：复用 AnalyzeBatch；移除 ensureCategory 自动建分类，isNewCategory → 写
      category_suggestions（去重计数）+ 元素失败
- [ ] 2.4 categorizer 串联三级；TrackedItem 增 classifiedBy 字段

## 3. 验证

- [ ] 3.1 单测：规则映射、余弦/阈值、建议去重、流水线逐级回退
- [ ] 3.2 集成：mongo + fake embedder/LLM 跑通三级命中与失败路径
- [ ] 3.3 golden set 骨架（taxonomy/golden.yaml + eval 脚本）—— 标注量后续补
- [ ] 3.4 人工审定 1.1/1.2 初稿后重跑存量分类
