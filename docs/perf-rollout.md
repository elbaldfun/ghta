# 接口性能优化：上线与验证手册

面向 starrank.dev 的读接口性能优化上线。部署架构见 [DEPLOY.md](./DEPLOY.md);
本文只讲这批改动怎么安全上线、怎么验证"只快不变"。

> 原则:**性能优化 = 更快且结果不变**。每一步都用 [`scripts/perf-verify`](../scripts/perf-verify/README.md) 的差分回放确认行为未变,用 Mongo `explain` / `bench` 确认确实变快。

---

## 改动总览

| 档 | 改了什么 | 主要收益 | 需要迁移? |
|---|---|---|---|
| **P0** | 板块排序改 `_id` 收尾的复合索引 + 排序加 `_id` tiebreaker;List 匹配计数加 5min 缓存;去掉首页冗余全表 count | 筛选/搜索/翻页少读 4–5× 文档,搜索不再每次全表正则 count | 启动自动建索引 + drop 旧索引(幂等) |
| **P2** | 把 `sourceData.readme` 拆到独立集合 `item_content` | 热集合 `tracked_items` 每文档 ~22KB→365B,装进 0.5GB WiredTiger 缓存 | **需要**跑 backfill(见下) |
| **P1** | 板块缓存(ecosystem/developers/topics…)改 stale-while-revalidate | 去掉每 TTL 周期一次的数秒阻塞重算 | 无 |

合并顺序:**P0 → P2 → P1**(线性依赖,`store.go`/`trend.go` 层层叠加)。

---

## 上线步骤

### 0. 部署前:录基线

在**旧版仍在线**时,对生产录一份响应基线:

```bash
./scripts/perf-verify/verify.sh record https://api.starrank.dev
```

> 若生产就是要升级的目标,先录基线再部署;有 staging 则对 staging 走完整 diff 再上生产。

### 1. 部署 P0

正常 `docker compose` 部署(见 DEPLOY.md)。启动时 `EnsureSchema` 会:
- 创建 `board_*` 复合索引(67k 文档,**分钟级在线构建**,不阻塞读);
- 幂等 drop 被取代的旧单字段排序索引。

观察容器日志确认启动无误后:

```bash
# 行为未变(真实数据下,diff 只应在“排序键并列”的行上显示重排,成员集不变即放行)
./scripts/perf-verify/verify.sh diff  https://api.starrank.dev
# 变快了(取 p50/p95)
./scripts/perf-verify/verify.sh bench https://api.starrank.dev
```

`explain` 抽查(应为 `IXSCAN board_*`、**无 `SORT` 阶段**、`docsExamined ≈ nReturned`):

```javascript
db.tracked_items.find({source:"github",type:"cli"}).sort({"metrics.stars":-1,"_id":-1})
  .limit(24).explain("executionStats").executionStats
```

### 2. 部署 P2,然后跑 README backfill

部署后**写入侧立即生效**(新抓取的 README 直接进 `item_content`);读侧对未迁移项**回退读内嵌 blob**,所以**迁移期间不停服**。

排空存量(admin token 见 `deploy/.env` 的 `ADMIN_API_TOKEN`):

```bash
while true; do
  r=$(curl -fsS -X POST -H "Authorization: Bearer $ADMIN_API_TOKEN" \
        "https://api.starrank.dev/internal/split-content?limit=1000")
  echo "$r"
  echo "$r" | grep -q '"count":0' && break
done
```

确认迁移完成 + 热集合收缩:

```javascript
db.tracked_items.countDocuments({"sourceData.readme":{$exists:true}})   // 期望 0
db.tracked_items.stats().avgObjSize                                     // 应从 ~2万B 掉到几百B
```

可选:立即回收磁盘(否则随文档 churn 自然收缩):

```javascript
db.runCommand({compact:"tracked_items"})
```

验证详情页仍返回 README(从 `item_content` 回挂):

```bash
curl -fsS "https://api.starrank.dev/trending/item?source=github&externalId=<owner>/<repo>" \
  | jq '.data.item.sourceData.readme | length'   // >0
```

### 3. 部署 P1

无迁移。部署后板块接口(ecosystem/developers/topics…)在 TTL 过期时**立即返回旧值、后台刷新**,不再有请求为重算阻塞。观感:板块页 TTL 边界不再偶发卡顿。

---

## 回滚

| 档 | 回滚方式 |
|---|---|
| P0 | 部署旧镜像即可;索引可留(旧码也能用),或手动 `dropIndex('board_*')` 并重建旧单字段索引 |
| P2 | **先确保 backfill 已把 README 写进 `item_content`**,回退旧镜像后读侧仍能用回退逻辑吗?——不能,旧码只读内嵌 blob。**故 P2 上线后若要回滚,需先把 `item_content` 的 README 写回 `tracked_items.sourceData.readme`**(逆向脚本),否则已迁移项详情页 README 为空。建议 P2 观察一个周期确认无误再谈回滚 |
| P1 | 部署旧镜像即可,纯内存行为,无持久化改动 |

> P2 是唯一有数据形状变化的一档,回滚成本最高——上线时重点盯详情页 README 与 `item_content` 计数对账。

---

## 验证工具速查

- 差分回放:[`scripts/perf-verify/README.md`](../scripts/perf-verify/README.md)
  - `PERF_PATHS=<file>` 可只对某批路径跑,聚焦一次改动
- 回归测试(真 Mongo):
  ```bash
  docker run -d --name m -p 47017:27017 mongo:7
  MONGODB_URI="mongodb://localhost:47017" go test ./internal/service/ ./internal/repository/
  docker rm -f m
  ```
- `explain` 判据:`IXSCAN` + 无 `SORT` 阶段 + `docsExamined ≈ nReturned`。

---

## 已知无关项

`internal/job` 的 `TestCategorizerMarksFailed` / `TestResourceClassNoFail` 在 `main` 基线上**就失败**(pre-existing flaky),与本批性能改动无关,单独跟踪。
