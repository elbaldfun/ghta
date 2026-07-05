# Tasks: 8-add-source-microsoft-store

## 0. 合规与通道评估（先行）

- [ ] 0.1 评估官方后端接口（StoreEdgeFD/DisplayCatalog）可行性、稳定性、使用条款
- [ ] 0.2 调研第三方数据源；评估受控自抓取的 ToS/法律与技术可行性
- [ ] 0.3 定默认通道并记录结论到 design.md

## 1. 适配器

- [ ] 1.1 通道接口抽象（A/B/C 可切换）+ 共用归一化
- [ ] 1.2 通道实现：拉取产品列表/详情（Product Store ID、name、rating、ratingCount、category、榜位?）
- [ ] 1.3 归一化 TrackedItem（source=msstore、externalId=Product Store ID、primaryMetric、metrics/sourceData）
- [ ] 1.4 实现 Fetcher 契约并注册；限速/缓存/降级；config

## 2. 复用与前端

- [ ] 2.1 MetricSnapshot 记录指标；rising 反映增长/榜位（榜位用 change 6 的 direction）
- [ ] 2.2 AI 分类对软件条目走通
- [ ] 2.3 前端 source=msstore 展示

## 3. 验证

- [ ] 3.1 单测：通道解析、归一化映射、增长/榜位方向
- [ ] 3.2 端到端：拉取一批产品入库，排行/详情正确；通道失效降级不崩
