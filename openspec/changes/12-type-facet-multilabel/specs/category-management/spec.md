# category-management 变更

## MODIFIED Requirements

### Requirement: 分类维度模型

分类 SHALL 区分**领域主树**与 **type 形态 facet** 两个正交维度：

- 领域主类以受控 `taxonomy.yaml` 为唯一事实源，含 name、description、parentId、level、path
  （物化路径）、createdBy（`taxonomy`）；一个条目可归入多个领域主类；树中 SHALL NOT 包含
  形态性大类（如原 `learning`）。
- type 以受控 `facets.yaml` 为唯一事实源：单值枚举（如 tutorial/awesome/interview/cli/app/
  library 及兜底值），不参与领域树层级。

两者的变更 SHALL 仅通过修改对应 git 资产实现。

#### Scenario: 领域主类为受控资产

- **WHEN** 系统启动同步分类
- **THEN** 领域主类按 `taxonomy.yaml` 的 path upsert，createdBy=taxonomy

#### Scenario: type 与领域正交

- **WHEN** 一个 React 教程仓库被分类
- **THEN** 其领域主类为 `web/frontend`，type 为 `tutorial`，二者互不隶属

### Requirement: 分类树查询

系统 SHALL 提供领域树查询接口，将领域主类按 parentId 递归组装为树形（id/name/path/children）
返回；无子节点的分类不含 children。系统 SHALL 另提供 type 枚举查询供前端筛选器渲染。

#### Scenario: 查询领域树

- **WHEN** 客户端请求分类树
- **THEN** 返回以根领域为顶层的嵌套树，不含已下放的 `learning`

#### Scenario: 查询 type 枚举

- **WHEN** 客户端请求 type 列表
- **THEN** 返回 facets.yaml 中定义的全部 type 取值及显示名
