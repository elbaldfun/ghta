# Tasks: 7-add-source-chrome-webstore

## 0. 合规与通道评估（先行）

- [ ] 0.1 评估第三方数据 API（chrome-stats 等）成本、字段、授权条款
- [ ] 0.2 评估自抓取的 ToS/法律边界与技术可行性（robots、反爬、稳定性）
- [ ] 0.3 定默认通道（A 第三方 / B 自抓取），记录结论到 design.md

## 1. 适配器

- [ ] 1.1 通道接口抽象（A/B 可切换）+ 共用归一化逻辑
- [ ] 1.2 通道实现：拉取扩展列表/详情（id、name、users、rating、ratingCount、category、开发者、图标）
- [ ] 1.3 归一化 TrackedItem（source=chrome、externalId=扩展ID、primaryMetric=users、metrics/sourceData）
- [ ] 1.4 实现 Fetcher 契约并注册；限速/缓存/失败降级；config 通道与范围

## 2. 复用与前端

- [ ] 2.1 MetricSnapshot 记录 users/rating；rising 反映用户数增长
- [ ] 2.2 AI 分类对扩展条目走通
- [ ] 2.3 前端 source=chrome 展示

## 3. 验证

- [ ] 3.1 单测：通道解析、归一化映射、users 量级增长
- [ ] 3.2 端到端：拉取一批扩展入库，排行/详情正确；通道失效时降级不崩
