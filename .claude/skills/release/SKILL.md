---
name: release
description: 把 starrank.dev 发布到生产 droplet（构建镜像、传到服务器、重启、验证）。用户说“发布版本”“发版”“上线”“部署到生产”“release”“deploy”时使用。
---

# 发布 starrank.dev

用户说“发布版本”就是授权你**把已合并到 main 的代码**发到生产：自己构建镜像、
自己传到服务器、自己重启并验证，不需要逐步确认。全部通过 `scripts/release.sh`
完成，不要手写 `docker save | ssh` 之类的命令。完整背景见 `docs/DEPLOY.md`。

## 流程

1. **看线上现在是什么版本**

   ```bash
   scripts/release.sh status
   ```

   输出里有 api / web 各自镜像对应的 commit（`revision`）和最近的发布记录。

2. **判断要发什么**：对比线上 revision 与 `origin/main`：

   ```bash
   git fetch -q origin main && git diff --stat <线上 revision>..origin/main
   ```

   | 改动落在 | 发布 |
   |---|---|
   | `cmd/` `internal/` `pkg/` `taxonomy/` `go.mod` `Dockerfile` | `api` |
   | `web/` | `web` |
   | 两边都有 | `all`（脚本保证后端先行） |
   | `deploy/docker-compose.yml` | 先 `compose`，再发镜像 |
   | `deploy/starrank.caddy` | `site` |
   | 只有 `docs/` `openspec/` 等 | 不用发，直接告诉用户 |

   revision 显示 `unknown`（脚本启用前构建的旧镜像）时，无法对比，按用户说的范围发；
   用户没说就发 `all`。

3. **发布**

   ```bash
   scripts/release.sh api     # 或 web / all / compose / site
   ```

   脚本会自己做：检查工作区干净且 HEAD 已并入 `origin/main` → `go vet` + `go test`
   （仅 api）→ 交叉编译 amd64 镜像 → 把线上镜像留作 `:prev` → 传到服务器 →
   `docker compose up -d` → 等健康检查 200 并确认容器跑的是新镜像 → 写发布记录。

   构建和传输要几分钟，用足够长的超时（10 分钟）运行。

4. **汇报**：发了哪个服务、哪个 commit、健康检查结果。失败时贴出脚本的报错。

## 约束

- **只发已合并的代码。** 脚本拒绝发布未并入 `origin/main` 的 HEAD。待发的改动还在
  PR 里时，先请用户合并（合并 main 需要用户明确同意），不要用
  `RELEASE_ALLOW_UNMERGED=1` 绕过，除非用户明确要求发某个未合并的分支。
- **健康检查失败就回滚**，然后再排查：

  ```bash
  scripts/release.sh rollback api   # 或 web
  ```

- **不要碰别的项目。** 这台 droplet 还跑着 opticdrop.app，共用 `/opt/edge` 的 Caddy。
  发布只动 `/opt/ghta` 下的 api / web 容器；不要往 `/opt/ghta/` 放 `Caddyfile`，
  不要改 `/opt/edge` 里除 `starrank.caddy` 以外的任何东西。
- **数据库迁移不在发布范围内。** 无 schema 变更时 API 启动的 `EnsureSchema` 会自动
  就绪；需要手动迁移数据的改动，先和用户确认再发。
- 本机 Docker 是 OrbStack，没启动时脚本会自己拉起。
