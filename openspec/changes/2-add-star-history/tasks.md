# Tasks: 2-add-star-history

## 1. 数据层

- [ ] 1.1 新增 RepoSnapshot schema + Time Series collection 初始化（启动时 ensure）
- [ ] 1.2 `parseTrendingReposIntoDB` 在 upsert 主文档的同时追加当日快照（同日已有则跳过）
- [ ] 1.3 GithubTrend schema 新增 starIncreaseDaily/Weekly/Monthly 字段
- [ ] 1.4 快照集合 TTL/保留策略（400 天）

## 2. 指标计算

- [ ] 2.1 新增 TrendMetricsService：聚合快照计算日/周/月增量，bulkWrite 回填主文档
- [ ] 2.2 抓取任务完成后触发指标计算（链式调用，失败独立记日志）
- [ ] 2.3 窗口起点无快照时指标置 null

## 3. API

- [ ] 3.1 新增 `GET /trending/rising?window=daily|weekly|monthly&language=&limit=`
- [ ] 3.2 Swagger 文档 + DTO 校验
- [ ] 3.3 starIncrease* 字段加入现有 /trending 返回

## 4. 验证

- [ ] 4.1 单测：指标计算（正常、缺口、新仓库 null）
- [ ] 4.2 手动跑两天抓取（或 mock 两天快照）验证 rising 排行正确
