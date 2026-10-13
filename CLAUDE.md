# StarRank（仓库 ghta）

开源项目热度榜 starrank.dev：Go 后端（`cmd/api`、`internal/`）、Next.js 前端（`web/`）、MongoDB；生产在 DO droplet（`docs/DEPLOY.md`）。回复和结论用中文。

## 工作规则

- 分支模型（与 OpticDrop 一致）：`develop` 是集成分支，`main` 是生产分支。改动走工作分支，开 PR 到 `develop`（`gh pr create --base develop`）；不直接改 `develop` 或 `main`。CI 的两个任务（`build-test`、`web`）通过后 PR 自动合并；想让 PR 先停着，开成草稿或加 `hold` 标签。改动 `.github/workflows` 的 PR 工作流合不了：负责人已授权 Claude 在 CI 通过后用 `gh pr merge --merge` 合并。
- `develop` 到 `main` 由 `.github/workflows/sync-main.yml` 同步：手动触发，或仓库变量 `AUTO_SYNC_MAIN=true` 时每次推送自动同步。重构切换之前这个变量不设，`develop` 上的重构代码不进 `main`。
- 生产热修是唯一直接开 PR 到 `main` 的情况；合并后要把 `main` 合回 `develop`（开一个 `main` → `develop` 的 PR）。
- 发版只用 `scripts/release.sh`（`api` / `web` / `all` / `rollback`），只发已并入 `origin/main` 的 HEAD，后端先于前端。负责人说"发布版本"就是让你自己发。
- 生产机只看 `docker logs --tail N`，绝不用 `--since`；查库用 `docker run --rm --network ghta_default -m 220m --env-file /opt/ghta/.env mongo:7 sh -c 'mongosh --host mongo …'` 从容器外连，不要 `docker exec` 进 mongo 容器。不要碰 `/opt/edge`（与 opticdrop.app 共用的 Caddy）以外的其他项目。
- LLM 分类任务同一时刻只能有一个 Run（中继并发上限很低），触发前先确认上一轮的 `categorizer done` 日志。
- 重构进行中（方案见 `docs/REBUILD.md`）：新代码只读写新集合（`items`、`item_blobs`、`boards` 等），不改旧集合 `tracked_items`，也不在新代码里写旧数据形态的兼容分支，规范化只放在导入器里。重构的 PR 进 `develop`，切换前 `develop` 不同步到 `main`。
- 分类树只改 `taxonomy/*.yaml`：改叶子要同时写 `migrations.yaml`，版本号加一；`go test ./internal/taxonomy ./internal/service` 会校验树、规则表、golden、货架枚举四者一致。

## 项目进度同步到 Obsidian

本仓库的待办、问题和决策统一记录在负责人的 Obsidian 库里。完成有意义的工作后按下面的约定写回；**做一件、更新一件**，不要攒到最后。

- 项目卡：`/Users/clawbot/Obsidian/projects/StarRank/StarRank.md`
- 项目链接名：`[[StarRank]]`
- 条目目录：`/Users/clawbot/Obsidian/TaskNotes/Tasks/`
- 条目标题统一以 `StarRank ` 开头
- 仓库里 `docs/REBUILD.md`、`docs/REDESIGN.md`、`docs/PRODUCT-REVIEW.md`、`docs/TAXONOMY-V3.md` 是方案与分析，不是待办表；待办以 Obsidian 为准

### 什么时候写回

- 修完一个 bug、做完一个功能、发了一个版本 → 关闭对应条目，项目卡"进展日志"加一行
- 发现新 bug、新需求、需要负责人拍板的问题、必须负责人亲手做的事（合并 PR、充值、商店后台操作）→ 新建条目
- 一个问题有了结论 → 条目改 done 并写清结论；方向性的决定在项目卡"决策日志"加一行
- 上线类任务（发版、迁移）每完成一步就勾掉条目里对应的 checkbox，并在正文记下日期和结果
- 小改动（重构、格式、补测试）不写回

### 条目格式（TaskNotes）

一条一个文件，文件名就是标题（不含 `/ : # ? *`）。

```yaml
---
status: open          # none=收集箱 · open=待办 · in-progress=进行中 · done=完成或决定不做
priority: normal      # high=P0 · normal=P1 · low=P2
dateCreated: 2026-10-13T14:00:00.000+08:00
dateModified: 2026-10-13T14:00:00.000+08:00
completedDate: 2026-10-13   # 仅 done 时填
scheduled: 2026-11-13       # 可选，定期要做的事
projects:
  - "[[StarRank]]"
tags:
  - task
  - 问题              # 五选一：功能 / 任务 / bug / 问题 / 需求
blockedBy:            # 可选
  - "[[StarRank 另一条的标题]]"
---
```

正文按类型写：

- **问题**：出处（文档章节）、现状、选项及代价、Claude 的建议、结论（先空着，等负责人定）
- **任务**：要做什么（步骤用 checkbox）、做完的标准、负责人要做的部分单独标出
- **bug**：现象、原因、已做的处理、还缺什么、需要负责人做什么
- **功能 / 需求**：为什么做、验收标准、不做什么

### 拆子任务

- 预计超过两天、或分几天做的事拆成子任务；子任务的 `projects` 同时写 `[[StarRank]]` 和父条目链接
- 一次能做完的步骤用正文 checkbox，不拆
- 父条目在所有子任务 done 后再关闭

### 项目卡里能改什么

- 可以改：`当前状态`、`下一步`（最多 3 条，链接到条目）、`进展日志`（倒序，一行一条，带日期）、`决策日志`（只追加），以及 frontmatter 的 `next_action` 和 `updated`
- 不要改：`目标与成功标准`，以及 frontmatter 的 `status`

### 规则

- 写之前先在条目目录里找同一件事，有就更新，不重复建
- 不删除任何文件。不做的事改成 `status: done`，正文写明"不做"及理由
- 只动本项目的条目和项目卡，不碰其他项目、`wiki/`、`journal/`、`.raw/`
- 不确定的事实标"待确认"，不要猜
- 写完在回复里告诉负责人改了哪几条
