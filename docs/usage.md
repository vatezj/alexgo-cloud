# alexGo-cloud 使用文档

> 本文依据当前代码整理，是新功能的操作入口参考。与实现不一致时，以代码为准。
> 架构与装配见 [architecture.md](architecture.md)；OAuth2 统一登录见 [oauth2.md](oauth2.md)。

## 0. 通用约定

- **Base URL**：所有接口挂在 `/api` 组下（`alexgo-server/server/http.go`）。
- **认证**：请求头 `Authorization: Bearer <token>`；登录经 OAuth2 统一登录签发**不透明令牌**
  （SSO 授权码流或 `POST /api/oauth2/token`，见 [oauth2.md](oauth2.md)）。
  注意 `/api/oauth2/*` 协议端点用 RFC 6749 信封 `{"error","error_description"}`，与本节 admin 信封不同。
- **权限（Casbin）**：每个接口前缀注册为权限码，需角色持有对应权限才可访问，否则 403。
  权限码注册见 `modules/system/service/permission_routes.go`。
- **信封**（infra 模块惯例，`codegen_controller.go` 头注释）：
  - 列表：`{"data": [...], "total": n}`
  - 详情：`{"data": {...}, "columns": [...]}`
  - 变更成功：`{"status": "ok"}`
  - 错误：`{"error": "..."}`，状态码只用 **400**（业务拒绝/参数错）与 **500**（内部错），仓库无 404 惯例。

---

## 1. 菜单管理（系统管理 → 菜单）

### 1.1 界面操作

入口：管理后台 **系统管理 → 菜单管理**（`apps/web-antd/src/views/system/SystemMenusPage.vue`）。

- 表格为**树形表**（按 `parent_id` 转树，默认全部展开，点名称列箭头折叠），一次性加载全量菜单，不分页。
- **新建**：右上「新建」打开弹窗，填写名称/类型/路径/组件/权限码/图标/排序/状态。
- **编辑**：行操作「编辑」打开同一弹窗，提交走 `PUT`（编辑态由 `editingId` 区分）。
- **删除**：行操作「删除」，二次确认后调用 `DELETE`；删除后后端**全量重建 Casbin 策略**，被删菜单的权限立即失效。

**菜单类型**（`type`）：

| 值 | 含义 | 关键字段 |
| --- | --- | --- |
| `dir` | 目录（UI 分组，无路由） | `name`, `icon`, `sort` |
| `menu` | 页面 | `path`（路由路径）、`component`（前端组件路径） |
| `button` | 按钮级权限 | `permission`（权限码） |

**编辑限制**：不允许把节点挂到自己或自己的后代之下（防树成环，含深度上限 64 的脏数据环防御）；修改 permission/路由后策略自动重建，无需手动操作。

### 1.2 API

权限码：`system:menu` → 前缀 `/api/admin/system/menus`。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/admin/system/menus` | 菜单列表（扁平数组，前端转树） |
| POST | `/api/admin/system/menus` | 新建 |
| PUT | `/api/admin/system/menus/:id` | 编辑 |
| DELETE | `/api/admin/system/menus/:id` | 删除 |

编辑/新建载荷（`MenuPayload`，见 `apps/web-antd/src/api/admin.ts`）：

```json
{
  "parent_id": 0,
  "type": "menu",
  "name": "代码生成配置",
  "path": "/codegen/config",
  "component": "views/infra/codegen/index",
  "icon": "lucide:file-code",
  "permission": "infra:codegen",
  "sort": 1,
  "status": 1
}
```

编辑示例：

```bash
curl -X PUT http://localhost:8080/api/admin/system/menus/12 \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"parent_id":1,"type":"dir","name":"代码生成","path":"/codegen","component":"","icon":"lucide:code","permission":"infra","sort":90,"status":1}'
```

> `status` 显式传 `0` 即停用——编辑接口不会把 0 当作"未传"套默认值。

---

## 2. 代码生成配置

代码生成分阶段交付：**M2** 配置持久化与同步 + 管理页面；**M3** 在线链路已接通——预览 / ZIP 下载 / CLI 读配置表（当前）；M4–M7 依次续交付（见 spec §8）。
设计规格见 [codegen-redesign-design](superpowers/specs/2026-10-10-codegen-redesign-design.md)。

### 2.1 前置

- **权限码**：`infra:codegen`（页面）、`infra:codegen:import` / `infra:codegen:sync` / `infra:codegen:delete`（按钮），
  路由前缀 `/api/admin/infra/codegen` 已在 `permission_routes.go` 注册（通配覆盖 `db-tables` 等子资源）。
  预览/下载不引入新按钮码，复用页面权限 `infra:codegen`。
- **菜单入口**：**系统管理 → 代码生成 → 代码生成配置**（`/codegen/config`，
  组件 `views/infra/codegen/index`，迁移 `20261011000002_codegen_menus` 种入）。
- **元数据库**：导入/Sync/表下拉读取的是配置的元数据库（DSN 见 `configs/config.yaml` 的 `dsn`）。

### 2.2 界面操作（`/codegen/config`）

| 操作 | 权限 | 行为 |
| --- | --- | --- |
| **导入表**（工具栏） | `infra:codegen:import` | 弹窗下拉搜库内表（`GET db-tables`）→ `POST import`；重复导入 400 |
| **字段配置**（行） | `infra:codegen` | 全量编辑**用户配置列**（控件/列表/表单/查询开关/查询方式/字典/排序），保存为整体替换 `PUT columns` |
| **编辑**（行） | `infra:codegen` | 改 module / business_name / class_name / 备注（白名单合并，见 2.5） |
| **同步**（行） | `infra:codegen:sync` | 重读元数据算差异并落库，toast 显示 added/updated/deprecated/unchanged 计数 |
| **删除**（行） | `infra:codegen:delete` | 二次确认后删配置（级联删列，**不动数据库物理表**） |
| **生成代码**（工具栏） | `infra:codegen` | 预览**全部**已导入表：弹窗文件树（后端/前端分组）+ 代码 + 底部「下载 ZIP」 |
| **预览**（行） | `infra:codegen` | 同上弹窗，但只渲染该表 |
| **下载**（行） | `infra:codegen` | 该表直接下载 ZIP（文件名 `codegen_<表名>.zip`） |

**快照字段 vs 配置字段**（spec §9.4，理解字段配置弹窗的关键）：

- **快照字段**（`type`/`comment`/`go_type`/`json_name`/主键空性等）：导入时从元数据落库，**同步时刷新**，
  `SaveColumns` 不写它们 → 界面上「注释」列**只读**，改了保存也不会生效。
- **配置字段**（`html_type`/`list_enable`/`form_enable`/`query_enable`/`query_operation`/
  `list_required`/`form_required`/`dict_type`/`sort_order`）：用户可编辑，保存走
  `Select(configFields)` 白名单，**同步永远不覆盖**。

### 2.3 接口一览

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/admin/infra/codegen/tables` | 分页列表（`?page=&size=`，size ≤ 200） |
| GET | `/api/admin/infra/codegen/db-tables` | 库内可导入表名（导入弹窗下拉数据源，`{data:[...]}`；元数据不可达 → 500） |
| POST | `/api/admin/infra/codegen/import` | 导入物理表为配置（`{"table_name":"..."}`） |
| GET | `/api/admin/infra/codegen/tables/:id` | 详情（`{data, columns}`） |
| PUT | `/api/admin/infra/codegen/tables/:id` | 改配置（白名单字段，见 2.5） |
| DELETE | `/api/admin/infra/codegen/tables/:id` | 删除配置（级联删列） |
| GET | `/api/admin/infra/codegen/tables/:id/columns` | 列配置列表 |
| PUT | `/api/admin/infra/codegen/tables/:id/columns` | 批量保存列配置（整体替换，非空数组；只落配置字段） |
| POST | `/api/admin/infra/codegen/tables/:id/sync` | 从元数据同步差异（spec §9.4） |
| POST | `/api/admin/infra/codegen/preview` | 预览渲染（body `{"table_ids":[1,2]}` → `{data:[{path,content}]}`；纯内存，失败无副作用） |
| POST | `/api/admin/infra/codegen/download` | 同 body → `application/zip` 流（`Content-Disposition: attachment; filename="codegen.zip"`） |

### 2.4 典型工作流（curl）

```bash
BASE=http://localhost:8080/api/admin/infra/codegen
AUTH="Authorization: Bearer $TOKEN"
CT='Content-Type: application/json'

# 1) 导入物理表（表名只允许小写 snake；重复导入返回 400）
curl -X POST $BASE/import -H "$AUTH" -H "$CT" -d '{"table_name":"order_items"}'

# 2) 查详情，拿 id 与列配置
curl -H "$AUTH" $BASE/tables/1

# 3) 改配置（只传要改的字段；见 2.5 校验规则）
curl -X PUT $BASE/tables/1 -H "$AUTH" -H "$CT" \
  -d '{"module":"order","business_name":"order_item","class_name":"OrderItem"}'

# 4) 调列配置：开关展示/表单/查询字段、必填、字典等（整体替换）
curl -X PUT $BASE/tables/1/columns -H "$AUTH" -H "$CT" \
  -d '[{"id":10,"name":"status","list_enable":true,"form_enable":true,
        "query_enable":true,"query_operation":"eq","sort_order":3}]'

# 5) 表结构变更后同步：新增列默认全关、消失列标 deprecated、快照刷新不碰配置
curl -X POST $BASE/tables/1/sync -H "$AUTH"
# → {"data":{"added":1,"updated":0,"deprecated":0,"unchanged":7}}
```

### 2.5 配置字段与校验（PUT `/tables/:id`）

可编辑字段（**白名单合并**：只覆盖传入的非零字段，`table_name`/`created_at` 等快照与系统字段不受 body 影响）：

| 字段 | 校验 |
| --- | --- |
| `module` | 小写 snake `^[a-z][a-z0-9_]*$`，≤128（拼进生成物路径，拒绝 `..`/`/`） |
| `business_name` | 同上 |
| `class_name` | Go 导出标识符 `^[A-Z][A-Za-z0-9_]*$`，≤128；**同 module+class_name 跨行冲突拒绝** |
| `template_type` | 1=单表, 2=树表, 3=主子表（0 视为未传） |
| `front_type` | 1=vben5-antd（0 视为未传） |
| `parent_table_id` | 主子表父表 id（0 视为未传） |
| `remark` | 自由文本 |

校验失败与重复导入均返回 **400** `{"error": "...table/class_name..."}`（错误文本包 `ErrTableInvalid` 哨兵）。

### 2.6 同步语义（spec §9.4）

`POST /tables/:id/sync` 重读元数据并分类：

| 分类 | 行为 |
| --- | --- |
| added（新列） | 插入行，所有开关默认关，sort 接在既有最大之后 |
| updated（快照变） | 刷新 Type/Comment/GoType/主键等快照，**配置字段不动**；deprecated 列重现即复活 |
| deprecated（列消失） | 标 `deprecated=1`（只标未标过的行），不物理删除 |
| unchanged | 不写库 |

返回计数 `{added, updated, deprecated, unchanged}`。

### 2.7 CLI（可选）

`tools/codegen` 提供同引擎的命令行入口，**默认读配置表**（与后台预览同一渲染路径 `service.Preview`）。
写盘规则：已存在不覆盖，`-force` 才覆盖；全部渲染成功才写盘。常用 flag：

```bash
# 日常主用法：从配置表生成（已导入的表；module/tables 均为过滤器，空=全部）
go run ./tools/codegen -module=order -tables=order_item
go run ./tools/codegen -module=order                # 模块下全部已导入表
go run ./tools/codegen                              # 全部已导入表

# 现场导入：读元数据写入配置表后再生成（已导入的表跳过，幂等）
# 注意：-import 必须配 -module（导入行要写目标模块）
go run ./tools/codegen -module=order -tables=order_item -import
go run ./tools/codegen -module=order -import        # -tables 留空 = schema 全部表

# 通用：-dsn（默认 config database.dsn / DB_DSN）、-schema（仅 -import，默从 dsn 解析）
#       -out（默认 .）、-dry-run、-force、-no-tests（覆盖 config codegen.unit_test_enable）
```

> 与旧 draft 模式（直读元数据、无需配置表）的差异：不带 `-import` 时**必须先在后台导入**，
> 否则报「没有匹配的已导入表」。`make generate-db-crud` 已自动带 `-import`，旧用法不受影响。
>
> **已裁定偏离（spec §5，M3）**：
> 1. 「后台三页面」→ 沿用 M2 modal 结构（导入/字段配置/编辑），M3 只新增预览 Modal；
> 2. 「代码高亮」→ 纯 `<pre>` 等宽展示（仓库无高亮依赖，YAGNI，M4 可加）；
> 3. CLI 示例 `--dsn=... --tables=t1 --import` 未带 `--module` → 实现**要求 `-import` 必须配 `-module`**，
>    错误信息明确提示；
> 4. 伪代码 body `tableIds[]` → 实际 `{"table_ids":[...]}`（仓库 snake_case JSON 惯例）。
