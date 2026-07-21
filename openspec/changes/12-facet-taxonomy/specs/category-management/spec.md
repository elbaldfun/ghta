# category-management 变更

## MODIFIED Requirements

### Requirement: 分类维度模型

分类 SHALL 区分**领域主树**与**正交 facet** 两类维度：

- 领域主类以受控 `taxonomy.yaml` 为唯一事实源，含 name、description、parentId、level、path
  （物化路径，如 `data/database`）、createdBy（`taxonomy`）；一个条目可归入多个领域主类。
- facet 以受控 `facets.yaml` 为唯一事实源：`type`（形态，单值枚举，如 library/app/cli/tutorial/
  awesome）与 `tech`（技术栈，多值，如语言与关键框架）。facet 不进入领域树层级结构。

领域树与 facet 枚举的变更 SHALL 仅通过修改对应 git 资产实现。

#### Scenario: 领域主类为受控资产

- **WHEN** 系统启动同步分类
- **THEN** 领域主类按 `taxonomy.yaml` 的 path upsert，createdBy=taxonomy

#### Scenario: facet 与领域正交

- **WHEN** 一个「Rust 编写的 K8s CLI」被分类
- **THEN** 其领域主类为 `infra/containers`，`type=cli`，`tech` 含 `rust`，三者互不隶属

### Requirement: 分类树查询

系统 SHALL 提供领域树查询接口，将领域主类按 parentId 递归组装为树形（id/name/path/children）返回；
无子节点的分类不含 children。系统 SHALL 另提供 facet 枚举查询（`type` 列表、`tech` 列表）供前端筛选。

#### Scenario: 查询领域树

- **WHEN** 客户端请求分类树
- **THEN** 返回以根领域（无 parentId）为顶层的嵌套树，不含已下放的 `lang`/`learning`

#### Scenario: 查询 facet 枚举

- **WHEN** 客户端请求 facet 列表
- **THEN** 返回可用的 `type` 与 `tech` 取值，供筛选器渲染
