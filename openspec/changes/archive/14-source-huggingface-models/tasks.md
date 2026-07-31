# Tasks — HuggingFace 模型源

状态:**已交付并上线**(2026-07-29 上线,07-31 归档)。

## 已完成

- [x] `internal/source/huggingface/`:游标分页 client(expand 字段)+ 三榜分片适配器(downloads 20k / likes 10k / trending 5k)+ 单测
- [x] 指标语义:likes=主指标(累计,复用增量管线→hot 榜);downloads30d=滚动 gauge(采用榜);语料 27,110 条入库
- [x] pipeline_tag → 10 任务组静态映射,零 LLM;`UpsertItems` 支持源自带分类(hf/<task> + 标 done)
- [x] 隔离:heatmap/trending/suggest 默认 GitHub 语料;HF 走独立 `GET /models` + 复合索引
- [x] hot 榜 trendingScore 自举(增速数据满 24h 后自动切真 likes 日增)
- [x] 前端 `/models`(任务 tab + 4 排序 + gated/量化 chip)+ AI ▾ 导航 + 8 语言
- [x] 增强批:params 参数量(safetensors)、downloadsAll 累计、原始 tags、baseModels 谱系、languages/arxiv/datasets 关系(全 expand 顺带,零额外请求);前端参数量 chip
- [x] 每日抓取与 GitHub 同轮;metrics 同管线

## 实际结果

- 语料 27,110(预估 2–4 万内);任务组分布:text-gen 8.7k 最大
- params 覆盖 13,660、baseModels 12,624、languages 13,108
- likes 日增速已接管 hot 榜(自举如期退役)

## 偏差记录

- 部署时文本索引 `language_override` 冲突导致 API 中断 ~8 分钟(`language` 字段值被误当搜索语言);修复:override 指向不存在字段。教训:字段名与 Mongo 保留语义的冲突要在本地预演。
- 与并行 session 两次 rebase 冲突(reconciler/PageShell),均正确合并;layout-guard 曾拦下 /models 页(本地 `npx next build` 绕过了 npm 钩子)——此后本地验证一律 `npm run build`。

## 未做(留待)

- [ ] 站内模型详情页(likes/下载历史曲线,快照攒够后)
- [ ] Datasets / Spaces 榜(验证模型榜使用后)
- [ ] GitHub 仓库 ↔ HF 模型交叉链接
