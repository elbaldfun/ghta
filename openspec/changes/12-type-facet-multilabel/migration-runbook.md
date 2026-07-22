# 生产迁移 Runbook（change 12，tasks 6.3/6.4）

把存量 67k 文档从旧树迁到新树（多标签 categoryPath + type + generatedTopics）。
所有代码已就位并本地验证；本文件是**生产执行步骤**，由维护者按序操作。

## 前置：为什么可以零停机

- 模型 `PathList` 兼容解码：新二进制能读旧 string `categoryPath`（实测不再 500）。
- reset 只改 analysisStatus，`categoryPath/type` 保留到被重跑覆盖——迁移期站点照常服务（显示旧分类）。
- Sync 启动时删除不在 taxonomy.yaml 的旧分类（learning/*、*-framework 等），`GET /category` 自动干净。

## 步骤

### 1. 部署新二进制（含 taxonomy.yaml / facets.yaml / topic-map.yaml 资产）
- 交叉编译 + compose 部署到 droplet（注意 [[ghta-prod-droplet]] 的交叉编译与 timeseries 坑）。
- 启动时 Sync 会：同步新 44 叶、**删除旧 learning/*/framework 分类**、加 categoryPath/type 索引。
- 部署后验证：`GET /category` 顶层 13 域无 learning；`GET /trending` 返回 200（旧文档兼容读）。

### 2. 生产 .env 必改（否则 grok 掉档）
```
AI_PROVIDER=deepseek
LMSTUDIO_BASE_URL=https://dragoncode.codes/v1   # 或你的 grok 中转
LMSTUDIO_LOCAL_MODULE_NAME=grok-4.5
OPENAI_API_KEY=<relay/xai key>
CATEGORIZE_BATCH_SIZE=5      # 关键：grok batch>5 漏 40%+
DOMAIN_MAX_LABELS=3
EMBED_MODEL=                 # 分类不再用 embedding
```

### 3. 分批重跑（先小批校准，再全量）
```bash
TOKEN=<ADMIN_API_TOKEN>; API=https://api.starrank.dev
# 小批校准：重置 500 → 分类 → 抽查结果对不对
curl -sX POST -H "Authorization: Bearer $TOKEN" "$API/internal/reset-analysis?limit=500"
curl -sX POST -H "Authorization: Bearer $TOKEN" "$API/internal/categorize"   # 处理 ≤1000 pending/次
# 看日志 categorizer done items=.. rule=.. llm=..，抽查 /trending 分类质量
```
校准 OK 后全量：
```bash
curl -sX POST -H "Authorization: Bearer $TOKEN" "$API/internal/reset-analysis?limit=0"   # 全部 → pending
# 循环触发直到排空（categorize 每次 ≤1000；grok ~42 条/分，全量约 27h，可挂着定时触发）
while :; do
  curl -sX POST -H "Authorization: Bearer $TOKEN" "$API/internal/categorize"
  sleep 1800   # 半小时一批；或用 CATEGORIZE_CRON 让它自己跑
done
```
> 提示：也可直接靠 `CATEGORIZE_CRON` 每日自动消化，只是全量收敛慢。手动循环更快。

### 4. 迁移后校验（6.4）
```bash
# learning 不再是归类目标（应为 0）
mongosh ghta --eval 'db.tracked_items.countDocuments({categoryPath:{$regex:"^learning/"}})'
# 仍有 string categoryPath 的（未重跑完，应递减到 0）
mongosh ghta --eval 'db.tracked_items.countDocuments({categoryPath:{$type:"string"}})'
# type 分布合理、generatedTopics 有值
mongosh ghta --eval 'db.tracked_items.aggregate([{$group:{_id:"$type",n:{$sum:1}}}])'
```
- 重跑后拿 `go run ./cmd/eval`（连 grok）对现网跑一次，和 eval-baseline 的改造后数字（precision 0.65）对比，确认线上与离线一致。

## 回滚

- 分类结果是幂等 `$set`，重跑不破坏其它字段；如需回滚仅需部署旧二进制（PathList 兼容，旧二进制读新 array
  会失败——所以**回滚需先把 categoryPath 转回 string**，代价高）。→ 结论：正向兼容已保证零停机，**不建议回滚**，有问题就修正 taxonomy/topic-map 再重跑。

## 状态

- [x] 6.1/6.2 单测+集成（categorizer_test：type 优先级/多标签/资料类不失败/父类丢弃/失败升级）
- [x] 兼容解码 PathList（坑 1）、Sync prune 旧分类（6.4 代码）、reset-analysis 端点（6.3 工具）——本地实测
- [ ] 6.3 生产执行：部署 + .env + 分批重跑（**待维护者按本 runbook 操作，不可逆**）
- [ ] 6.4 生产校验：learning 归零、string 归零、eval 线上对比
