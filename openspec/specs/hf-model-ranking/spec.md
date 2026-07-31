# hf-model-ranking Specification

## Purpose

将 HuggingFace 模型作为第二数据源纳入多源趋势平台:有界语料抓取、快照增量、按任务分组的排行(热度/下载/点赞/最新),与 GitHub 语料严格隔离。

## Requirements

### Requirement: 有界语料抓取

系统 SHALL 每日抓取三榜并集(downloads top-20k ∪ likes top-10k ∪ trendingScore top-5k),游标分页,归一化为 TrackedItem(source=huggingface)。私有/禁用模型 SHALL 排除;gated 模型 SHALL 保留并打标。入库项 SHALL 持续快照(掉榜不删除)。

#### Scenario: 三分片抓取

- **WHEN** 每日抓取运行
- **THEN** 三个分片(downloads/likes/trending)各自分页拉满上限并 upsert,断点按 fetch_runs 恢复

### Requirement: 指标语义

`likes` 为主指标(累计值),其快照增量 SHALL 驱动 hot 榜;`downloads30d` 为近 30 天滚动 gauge,SHALL 仅直排采用榜、不作计数器。增量数据不足时 hot 榜 SHALL 以 trendingScore 自举并在真增速可用后自动切换。

#### Scenario: hot 榜自举退役

- **WHEN** likes 增量(dailyIncrease>0)存在
- **THEN** hot 榜按 likes 日增排序,不再使用 trendingScore

### Requirement: 任务分组与源隔离

分类 SHALL 复用官方 pipeline_tag 静态映射为 ~10 任务组(categoryPath="hf/<组>"),不进 LLM 管线(upsert 即标 analysisDone)。GitHub 语料的榜单/热力图/联想 SHALL 默认排除 HF items;HF SHALL 通过 `GET /models?task=&sort=hot|downloads|likes|new` 独立查询(白名单键,ttlCache)。

#### Scenario: 隔离

- **WHEN** 请求 /trending 或 /category/heat 未显式指定 source
- **THEN** 结果仅含 github 语料,"hf/*" 不出现在领域热力图

### Requirement: 富元数据

抓取 SHALL 通过 expand 顺带取得并入库:参数量(safetensors.total)、累计下载(downloadsAllTime)、原始 tags 整包、base_model 谱系(剥离 finetune/quantized 限定词)、语言/数据集/arxiv 关系。前端 SHALL 展示参数量(7B/70B 规模标)。

#### Scenario: 谱系提取

- **WHEN** 模型 tags 含 "base_model:finetune:Qwen/Qwen3-32B"
- **THEN** baseModels 含 "Qwen/Qwen3-32B"(限定词已剥离)
