# Tasks: 6-add-source-appstore

## 1. 适配器

- [ ] 1.1 iTunes RSS 榜单客户端：按 (country, chartType, genre, limit) 拉取有序应用列表
- [ ] 1.2 iTunes Lookup 客户端：批量补全元信息（rating/ratingCount/genre/developer/artwork/version）
- [ ] 1.3 归一化为 TrackedItem（source=appstore、externalId=trackId、primaryMetric=rankPosition、metrics/sourceData）
- [ ] 1.4 实现 Fetcher 契约并注册进 registry；配置国家/品类/榜型/TopN/频率

## 2. 指标方向支持

- [ ] 2.1 TrackedItem.primaryMetric 增加 direction（asc-better/desc-better）
- [ ] 2.2 change 2 指标计算据 direction 正确判定"增长/上升"（榜位减小为上升）

## 3. 复用与前端

- [ ] 3.1 确认 MetricSnapshot 记录榜位/评分；rising 排行正确反映榜位上升
- [ ] 3.2 AI 分类（change 3）对 App Store 条目走通（用 app 描述/品类）
- [ ] 3.3 前端 source=appstore 展示（列表/排行/详情），source 切换 tab

## 4. 验证

- [ ] 4.1 单测：feed 解析、Lookup 合并、归一化映射、榜位方向增量
- [ ] 4.2 端到端：拉取一组国家/品类榜入库，rising 与详情页正确
