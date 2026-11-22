# 分支与发版规范

StarRank 是网站：部署就是发布，一天可能发两次，所以用日期版本，不用语义版本和 rc 候选。

**`main` 就是线上。** `main` 只通过 `develop → main` 的发版 PR 更新；每次发版健康检查通过后，在发出去的 `main` 提交上打一个日期标签，并建一个 GitHub Release。

本文定规则。操作命令和服务器细节见 [DEPLOY.md](DEPLOY.md)，脚本是 `scripts/release.sh`。

## 1. 分支模型

| 分支 | 作用 | 谁能写入 | 合并方式 |
|---|---|---|---|
| `develop` | 集成分支，所有工作 PR 的目标 | 只经 PR | `build-test`、`web` 通过后由 CI 的 `auto-merge` 作业用 merge commit 合并 |
| `main` | 线上代码；`release.sh` 只发它包含的提交 | 只经 `develop → main` 发版 PR | CI 通过后由 master 用 merge commit 合并（**不要 squash**）；合并就是决定发版 |
| 工作分支 | 单一主题的改动 | 作者 | PR 到 `develop`，合并后删除 |

- 工作分支命名：`<类型>/<简短英文描述>`，类型见第 2 节。Claude 会话自动建的 `claude/<主题>-<后缀>` 分支保留原名。
- **不直接 push `develop` / `main`。** 两个分支都开了 GitHub 分支保护：必须走 PR、必须通过 `build-test` 和 `web`、禁止强推和删除（2026-11-22 起）。
- **跟上 `develop` 用 `git merge origin/develop`，不要 rebase。** rebase 之后要强推，强推是被禁止的。
- 指向 `main` 的 PR 只能来自本仓库的 `develop`，CI 的 `release-source` 作业会让其他来源直接失败。
- **热修也走 `develop`：** `fix/` 分支 → PR 到 `develop`（自动合并）→ 发版 PR → 发版。不在 `main` 上改，也就不用再把 `main` 合回 `develop`。代价最多是多等一轮 CI。
- 发版 PR 的 merge commit 只存在于 `main` 上，GitHub 会显示 `develop`"落后 main N 个提交"。这是正常的，不需要回合并：只要没人直接改 `main`，两边的代码就是一样的。
- 以前用的 `sync-main.yml`（develop 自动同步到 main）和仓库变量 `AUTO_SYNC_MAIN` 已被发版 PR 取代：工作流已删除，变量在 2026-11-22 设成了 `false`。

## 2. 提交与 PR 标题

PR 标题决定它在发布说明里归到哪一类：

| 标题前缀 | 发布说明分组 |
|---|---|
| `feat:`、`feat(范围):` | 新增 |
| `fix:`、`fix(范围):`、`hotfix:` | 修复 |
| 其他（`chore:`、`docs:`、`perf:`、`refactor:`、`web:`、`fetch:` 这类只写范围的，或者没有前缀的中文句子） | 改进 |

- 想让一个 PR 出现在「新增」或「修复」里，标题就以 `feat:` / `fix:` 开头，例如 `fix(reconcile): treat a change of case as a rename`。只写范围的标题（`web: …`）一律归到「改进」。
- 一个 PR 只做一件事。描述写清楚改了什么、怎么验证的、还剩什么没做。
- 不想自动合并：开成草稿，或加 `hold` 标签。

## 3. CI 门禁

`.github/workflows/ci.yml`：

| 触发 | 作业 |
|---|---|
| PR → `develop` | `build-test`（gofmt · `bash -n scripts/release.sh` · vet · build · test）+ `web`（typecheck · layout guard · unit tests）→ `auto-merge` |
| PR → `main` | `release-source`（来源必须是 `develop`）+ `build-test` + `web`，**不自动合并** |
| push `develop` / `main` | `build-test` + `web` 兜底 |

- `auto-merge` 用 `gh pr merge --merge --match-head-commit`，合并的一定是通过检查的那个提交。
- 改了 `.github/workflows` 的 PR，工作流 token 合不了，由 master 在 CI 通过后用 `gh pr merge --merge` 手动合并。

## 4. 版本号

- 格式是 `vYYYY.MM.DD.N`，例如 `v2026.11.22.1`。
  - 日期取**打标签那一刻的 UTC 日期**，和 `releases.log`、流水线时间窗用同一个时区。北京时间早上 8 点前发版，日期还是前一天。
  - `N` 从 1 开始，同一天每发一个新提交加一。跳过的号（例如被删掉的标签）不回填。
- 标签是注释标签，打在发出去的那个 `main` 提交上，说明写 `StarRank vYYYY.MM.DD.N (api,worker,web)`，括号里是这次发了哪些服务。
- **一个提交只打一个标签。** 同一个提交再发一次（例如先发了 api，后来又发 web），不再打新标签，只在已有的 GitHub Release 说明末尾追加一行"再次发布：web"。
- **已有标签不移动、不删除。**
- 只有 `origin/main` 里的提交会被打标签。用 `RELEASE_ALLOW_UNMERGED=1` 应急发出去的未合并代码不打标签，`releases.log` 那一行的标签记成 `-`。
- 仓库里的旧标签 `v1-final`（重构切换前最后一版）不是日期版本，算版本号时会被忽略。

## 5. 发版流程

```
工作分支 ──PR──▶ develop ──promote──▶ 发版 PR「Release vYYYY.MM.DD.N」──master 合并──▶ main
                                                                                      │
                                                     scripts/release.sh api|web|worker|all
                                                                                      │
                                                          健康检查通过 → 打标签 + GitHub Release
```

1. **开发版 PR**：`scripts/release.sh promote`
   - 已经有打开的 `develop → main` PR 时，只刷新它的标题和正文，不会开第二个。
   - 标题是 `Release vYYYY.MM.DD.N`，用的是预计版本号；实际版本号在打标签时才确定，如果当天别人先发了一版，号会顺延。
   - 正文按「新增 / 修复 / 改进」分组，列出 `origin/main..origin/develop` 之间合并的 PR。
   - `develop` 没有 `main` 缺少的提交时，脚本会拒绝执行。
   - 加 `--dry-run` 只打印将要执行的 `gh` 命令和正文，不改 GitHub 上的任何东西。
2. **合并**：`release-source`、`build-test`、`web` 都通过、而且没加 `hold` 标签时，master 用 merge commit 合并。负责人想叫停，就给发版 PR 加 `hold`。
3. **发版**：在已经并入 `origin/main` 的 HEAD 上运行 `scripts/release.sh api` / `web` / `worker` / `all`。后端先于前端发。
4. **打标签**：所有服务的健康检查都通过后，脚本会：
   1. 算出版本号；
   2. 打注释标签并推送；
   3. 用 `gh release create --verify-tag --latest` 建一个正式的 GitHub Release（不是草稿，也不是预发布）。

   如果打标签这一步失败了（例如网络断了），服务其实已经发出去了。修好以后用 `scripts/release.sh tag <服务列表>` 补打，例如 `tag api,worker,web`。

### 流水线时间窗

worker 每天 UTC 03:30（北京时间 11:30）开始跑抓取和指标计算，08:30 前跑完。这段时间里重启 worker，会把当天剩下的抓取杀掉。

所以 `scripts/release.sh worker` 和 `all` 在 UTC 03:30–08:30 一律拒绝执行，并说明原因，**没有绕过开关**。

- 只发 `api` 或 `web` 不受限：它们不重启 worker。
- `rollback api` 会把 worker 一起换回上一个镜像。它是应急操作，不受时间窗限制，但在窗口里执行同样会打断当天的抓取。

### 回滚

- `scripts/release.sh rollback api|web` 换回服务器上的 `:prev` 镜像，行为和以前一样。
- 回滚**不动标签**：标签记录的是"这个提交曾经发出去过"，不是"线上现在是什么"。线上现在是什么，看 `scripts/release.sh status` 和 `releases.log`。
- 回滚后要修复，就走正常流程：修复 PR 进 `develop` → 发版 PR → 发版，发出来是一个新版本号。

### releases.log

服务器上的 `/opt/ghta/releases.log` 每次发版追加一行：

```
<UTC 时间> <服务> <标签> <短提交号> <提交标题>
2026-11-23T09:12:40Z api v2026.11.23.1 5e693b1 Merge pull request #80 from elbaldfun/develop
2026-11-23T09:30:02Z rollback-api -
```

没有标签的那一行（应急发未合并代码、`compose`、`site`、回滚）标签记成 `-`。

## 6. 发布说明

GitHub Release 的说明由脚本自动生成：

```
StarRank v2026.11.23.1：发布 api、worker、web。

## 新增

- rank change arrows (#72)

## 修复

- reconcile: treat a change of case as a rename (#74)

## 改进

- web: GitHub 榜单页迁到新设计 (#76)

上一个版本：v2026.11.22.1
```

- 第一行说明这一版是什么、发了哪些服务。
- PR 按标题前缀分到「新增 / 修复 / 改进」（规则见第 2 节），每行末尾写 PR 号。没有 PR 的分组不显示。归到「新增」「修复」的，`feat:` / `fix:` 前缀会去掉，范围保留。
- 列哪些 PR：上一个日期标签到这次的提交之间合并的 PR，不含发版 PR 自己，也不含 `main → develop` 的回合并。第一个日期标签没有"上一个"，只列最近 30 个 PR。
- 生成逻辑在 `internal/release`，有单测；`cmd/release` 是给脚本调用的命令行入口。

## 7. 已知限制

- `promote` 标题里的版本号是预计值。合并发版 PR 和实际发版之间如果别人又发了一版，打出来的号会比标题大，以标签为准。
- 分支保护里 `enforce_admins` 是关着的，负责人留着应急用：管理员仍然可以直接推。规则靠分支保护、CI 和这份规范一起约束。
- `release-source` 目前不是分支保护的必需检查。它失败时 PR 会显示红叉，但不会拦住合并；要拦住，需要把它加进 `main` 的必需检查。
