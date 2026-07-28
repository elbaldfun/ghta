# Change: HuggingFace 模型源(AI 模型排行)

## Why

站点定位"聚焦 AI",但目前只覆盖 GitHub(代码/工具)。AI 生态的另一半主体是**模型**——它们活在 HuggingFace 上。给 HF 模型做「谁在涨、按任务筛、速度优先」的排行,把情报面从"AI 工具"扩到"AI 模型",是主线上的自然扩张,也是 project.md 多源架构(TrackedItem + Source 适配器)设计之初就预期的第二个源。

**数据可行性已实测验证**(2026-07,官方公开 API,无需鉴权):

- `GET /api/models?sort=downloads|likes|trendingScore&direction=-1&limit=N`,Link header 游标分页;
- 每条含:`id, downloads(近30天滚动), likes(累计), trendingScore(官方热度), pipeline_tag(任务), library_name, tags(含语言/数据集/量化格式), createdAt, lastModified, gated`;
- 对比 project.md 里搁置的 Chrome/微软商店(要爬、有合规风险),HF 是**官方、免费、字段齐全**的干净通道,难度远低。

对比 HF 官方榜的差异化:HF 只展示**绝对下载数**与不透明的 trending;我们的价值是**速度视角**(likes 增速、下载动量、历史曲线)+ 任务维度榜 + 与 GitHub 生态的同站交叉。

## What Changes

- **新增 HF 源适配器** `internal/source/huggingface/`:游标分页抓取,归一化为 TrackedItem(`source=huggingface`,externalId=模型 id)。语料按阈值有界(见 design §2)。
- **指标语义扩展**:HF 的 `downloads` 是**近 30 天滚动窗口**(非累计,与 star 语义不同)——作为 gauge 存储与快照;`likes` 是累计值,其日增量与 star 增速同语义,作为「热度速度」主轴(见 design §3,这是本 change 最重要的设计决策)。
- **分类零 LLM**:任务分类直接复用 HF `pipeline_tag`(~40 个官方任务标签 → 收敛到 ~10 个任务组);不进现有 LLM 分类管线。
- **抓取/快照/趋势复用**:每日抓取 + metric_snapshots 时序 + 日/周增量,全部走现有管线,仅按源区分。
- **查询与前端**:`GET /models`(task/sort/q 过滤,复用 ttlCache 模式);新前端 `/models` 榜单页 + 导航入口(AI ▾ 下拉内);模型行点击外链 HF(v1 不做站内模型详情页)。

## Impact

- Affected specs: 新增 capability `hf-model-ranking`;`github-trend-fetching` 不动(HF 独立适配器)。
- Affected code:
  - 新增 `internal/source/huggingface/`(client + adapter + 单测)
  - `internal/job/fetcher.go`:源注册表已支持多源,注册 HF 适配器与其分片策略
  - `internal/service/models.go` + `internal/handler/models.go`:`GET /models`
  - `internal/repository/store.go`:HF 查询索引
  - 前端:`/models` 页、`ModelRow`、`getModels`、导航、8 语言
- **数据规模控制**:阈值语料(设计 §2),预估 2–4 万模型;快照量与现有 GitHub 源同量级,timeseries 承受得住。
- 无破坏性迁移;GitHub 源行为零变化。
