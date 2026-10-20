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
- **信封**（统一出站出口 `pkg/resp`，所有 API 响应必经 `OK/Fail/Page`）：
  - 成功：`{"code": 0, "msg": "ok", "data": ...}`；无返回值动作为 `"data": null`。
  - 分页列表：`"data": {"list": [...], "total": n, "page": p, "size": s}`
    （`resp.Page[T]`，入参 `page`/`size`，size ≤ 100；未分页列表 `"data"` 直接是数组）。
  - 失败：`{"code": <五位错误码>, "msg": <中文文案>, "data": null}`；
    **HTTP 状态按码段映射**（`pkg/resp/resp.go` 的 `statusFor`）：1xxxx/3xxxx → 400、
    2xxxx → 401（`20003` 无权限 → 403）、5xxxx → 500。仓库无 404 惯例。
  - 五位错误码分段见 `pkg/errors/code.go`：1xxxx 参数、2xxxx 认证授权、3xxxx 业务规则、5xxxx 系统。
  - 豁免清单（勿改）：`/api/oauth2/*` 协议端点用 RFC 6749 信封 `{"error","error_description"}`、
    `/health*`、`/metrics`、`/debug/pprof/**`。
- **文件与导出**：文件上传/下载见 §3，Excel 导出见 §4；新建模块脚手架见 §5；scorecard 打牌记账运维见 §6。

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
| GET | `/api/admin/infra/codegen/tables` | 分页列表（`?page=&size=`，size ≤ **100**，信封 `data:{list,total,page,size}`；前端按页循环拉全） |
| GET | `/api/admin/infra/codegen/db-tables` | 库内可导入表名（导入弹窗下拉数据源，信封 `data:[...]`；元数据不可达 → 500） |
| POST | `/api/admin/infra/codegen/import` | 导入物理表为配置（`{"table_name":"..."}`） |
| GET | `/api/admin/infra/codegen/tables/:id` | 详情（信封 `data:{data, columns}`） |
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

# 6) 预览/下载生成结果：
curl -X POST $BASE/preview -H "$AUTH" -H "$CT" -d '{"table_ids":[1]}'   # → {data:[{path,content}]}
curl -X POST $BASE/download -H "$AUTH" -H "$CT" -d '{"table_ids":[1]}' -o codegen.zip   # → ZIP
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

校验失败与重复导入均返回 **400** 统一信封：重复导入 `{"code":30008,...}`、
字段校验 `{"code":10001,...}`（错误文本包 `ErrTableInvalid`/`ErrTableTaken` 哨兵，
文案在 `msg`）。

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

### 2.8 前端产物（vben5_antd 单表模板族）

同一渲染管线会同时输出**后端 7 + 前端 7 = 单表 14** 份产物（`make codegen-smoke`
断言三表共 42）；前端模板位于
`pkg/codegen/template/templates/front/vben5_antd/single/`，模板定界符为 `[[ ]]`
（Vue 占用 `{{ }}`），函数 `pascal/snake/plural/lower` 可用。每个已导入表生成：

| 模板 | 产物（`{module}` / `{business}` 为模块名与表的业务名） |
| --- | --- |
| `types.ts` | `apps/web-antd/src/api/{module}/{business}/types.ts` — 实体类型 |
| `api.ts` | `.../api.ts` — CRUD 封装 + `matchQuery` 客户端过滤 + `queryOps` |
| `index.ts` | `.../index.ts` — 桶导出 |
| `list.vue` | `apps/web-antd/src/views/{module}/{business}/list.vue` — vxe-grid 列表页 |
| `form.vue` | `.../form.vue` — 新建/编辑弹窗 |
| `drawer.vue` | `.../drawer.vue` — 详情抽屉 |
| `api.test.ts` | `.../__tests__/{business}.test.ts` — vitest 表无关断言 |

要点：

- **列 formatter** 由字段 `html_type` 驱动：`DatePicker` → `formatDateTime(cellValue)`，
  `Switch` → `启用/停用`；导入后在字段配置里改 `html_type` 再 `-force` 重新生成即可生效。
- **权限**：列表页按钮挂 `v-access:code="['{module}:{business}:*']"`。需要在菜单管理建菜单
  （权限串 `{module}:{business}:*`）并绑定角色；`permissionRoutes` 需有对应前缀 → 路由条目，
  否则接口 403。前端 `accessCodes` 是登录时缓存的，改权限后要**重新登录**按钮才会出现。
- **运行测试**：`pnpm --filter @vben/web-antd test`（vitest，node 环境，跑 `src/**/*.test.ts`）。
- **本地验收链路**：生成 → 接线 module.go 路由与 `permissionRoutes` → 建菜单绑角色 → 重登 →
  页面 CRUD 全链路（E2E 用例即按此执行）。

#### 查询语义（`api.ts` 的 `matchQuery` 客户端过滤）

controller 一次性返回全量行，查询表单提交值经 `matchQuery` 客户端过滤（`queryOps`
记录每列的 `query_operation` 配置）：

- **空值跳过**：`undefined / null / ''` 的条件不参与过滤，返回全部行；
- **`eq` 精确匹配**：两侧都 `String(...)` 后全等（数字、字符串统一转字符串比较）；
- **`like` / `between` 子串匹配**：`toLowerCase().includes()`，**不区分大小写**。
  ⚠️ 生成页的 `between` 是**子串近似**而非区间过滤——与服务端 `between`（日期/数值
  区间）语义**不同**；配置 `query_operation='between'` 前先确认你要的是客户端模糊搜
  还是服务端区间（后者需走 API 参数，当前模板不生成）。

#### 列类型映射（Go ↔ TS ↔ 组件）

| Go 类型（`bind.tsType`） | TS 类型 | 表单组件 / 转换 |
| --- | --- | --- |
| `string` / `string*` | `string` | `Input`；`decimal string`（GoType 为 string 的 `decimal`）→ `InputNumber` |
| `bool` | `boolean` | `Switch`：提交 `Number(v ?? 0)`（→0/1），回显 `Boolean(row.v)`，列表 `启用/停用` |
| `int` / `uint*` / `float*` | `number` | `InputNumber`；主键 id 亦为 `number`（>2^53 的雪花 ID 会精度丢失，见局限票） |
| `time.Time` / `*time.Time` | `string` | `DatePicker`：回显 `dayjs(row.v)`，提交 `payload.v.toJSON()`（RFC3339）→ Go `time.Time`；列表 `formatDateTime` |
| `json.RawMessage` | `any` | 不生成表单列（`JSONName="-"` 同） |

- **非 0/1 的 tinyint(1)** 经 `Switch` 提交会收成 0/1（`Number(Boolean(v))` 链路），
  存量脏值以编辑保存为准归一。

### 2.9 树表模板族（template_type=2，M5）

- **固定约定**（spec §10.1）：表必须有 `parent_id`（整型自引用，根=0）与 `name`（varchar 显示名），
  缺列在生成入口报 `ErrColumnInvalid`（CLI 与在线预览同一路径）；导入不加配置列、无迁移。
- **后端语义**（spec §10.2）：repository 增 `HasChildren`；service `Create/Update` 前
  `validateParent`（parent_id≠0 时父存在且非自身），`Delete` 前有子拒绝；
  List 平铺返回、不加 Children 字段与 GetTree，构树在前端。
- **模板组织**（spec §10.3）：树表族 = 单表全集 14 产物，其中 5 个被 tree overlay 按输出模式覆盖——
  server `repository/service/service_test`、front `list.vue/form.vue`；其余 9 个复用单表模板。
- **前端**（spec §10.4）：`list.vue` 用 vxe `treeConfig`（transform + parentField/rowField + 全展开）、
  关分页、无搜索表单；`form.vue` 的 `parent_id` 按列名硬认 `TreeSelect`
  （打开时拉平铺 list 本地构树，虚拟顶级 value=0 可选）。
- **界面**：编辑弹窗「模板类型」下拉可切单表/树表（主子表 M6 置灰）。
- **验收**（spec §10.5）：`CGO_ENABLED=0 go test ./pkg/codegen -run TreeGolden`（14 产物）
  + `make codegen-smoke`（3 表 × 14 = 42 产物）。

---

## 3. 文件上传与附件（infra 文件存储）

权限码：`infra:file` → 前缀 `/api/admin/infra/file`（`modules/system/service/permission_routes.go`）。
存储驱动 `pkg/storage`（本地盘，纯 IO 三方法 `Save/Open/Delete`，可换 S3/OSS 实现同一 `Driver`），
策略（白名单/限流/随机名/落库）在 `modules/system/infra/service/file_service.go`。

### 3.1 API

| 方法与路径 | 说明 |
| --- | --- |
| `POST /api/admin/infra/file/upload` | 上传（multipart：字段 `file`；可选表单域 `public=true`、`biz_type=<归类>`） |
| `GET /api/admin/infra/file/:id` | 受控下载（`Content-Disposition: attachment`，需鉴权 + `infra:file`） |
| `DELETE /api/admin/infra/file/:id` | 删除（盘上文件 + 落库记录一并清） |
| `GET /api/public/infra/file/:key` | 公开读（`public_key` 随机键，**免鉴权**，`inline`；private 行与不存在统一 30010） |

上传成功返回：

```json
{"code":0,"msg":"ok","data":{"id":1,"name":"a.png","size":123,"mime":"image/png",
 "public_url":"/api/public/infra/file/<key>"}}
```

### 3.2 上传校验（双保险）

- **扩展名白名单**：内置 `jpg,jpeg,png,gif,webp,pdf,zip,xlsx`（刻意不含 svg/html，堵存储型 XSS）；
  `config.yaml` 的 `storage.allowed_exts` 可整体覆盖，省略则用内置表。不符 → `30011`。
- **大小**：`storage.max_size_mb`（默认 10）。上传头预检 + 落盘字节流限流双保险
  （`FileHeader.Size` 客户端可伪造，最终以流为准），超限删已写文件 → `30012`。
- **双 MIME 校验**：头 512 字节 `DetectContentType` 结果须落在该扩展名的允许集合
  （`extMimeOK`；zip/xlsx 同为 `application/zip`），不符 → `30011`。
- **路径**：随机名生成，驱动侧 `relPath` 不含任何用户输入（`pkg/storage` 包注释约定）。

### 3.3 公告附件闭环（前端示例）

`apps/web-antd/src/utils/notice-attachments.ts`：上传恒 `public:true + bizType:'notice'`，
把返回的 `{id,name,size,url}` 序列化成 JSON 数组存进 `system_notices.attachments`
（varchar 4096，后端原样存取不解析）；展示时按 `url` 直接走公开读端点。

```bash
# curl 上传
curl -X POST http://localhost:8080/api/admin/infra/file/upload \
  -H "Authorization: Bearer <token>" \
  -F "file=@./a.png" -F "public=true" -F "biz_type=notice"
```

---

## 4. Excel 导出

`pkg/excel`：声明式列（`Column[T]{Title,Cell}`）+ `Spec[T]{Sheet,FileName,Columns,Rows}`，
`Build` 一次产出 xlsx 字节；行数保险丝 `MaxExportRows = 50000`，超限 → `30013`（400 信封，
**先 Build 后写头**——Build 报错时一个字节未出，信封安全）。

### 4.1 端点（三域，GET，当前租户全量）

| 端点 | 权限码 | Sheet / 文件名 |
| --- | --- | --- |
| `GET /api/admin/system/users/export` | `system:user` | 用户 / `users_<yyyyMMdd_HHmmss>.xlsx` |
| `GET /api/admin/member/users/export` | `member:user` | 会员 / `member_users_<时间戳>.xlsx` |
| `GET /api/admin/order/orders/export` | `order:order` | 订单 / `orders_<时间戳>.xlsx` |

响应为**下载流**（豁免信封）：`Content-Type: application/vnd.openxmlformats-officedocument.spreadsheetml.sheet`
+ `Content-Disposition: attachment`（RFC5987 文件名）。前端走 `requestClient.download`
（`exportUsers` / `exportOrders`，见 `apps/web-antd/src/api/admin.ts`），
Blob 错误体会按信封解 `msg` 弹 toast（`apps/web-antd/src/api/blob-error.ts`）；
member 导出端点暂无前端按钮（后端 curl 可用）。

```bash
curl -OJ -H "Authorization: Bearer <token>" \
  http://localhost:8080/api/admin/system/users/export
```

> 导入目前留小票（roadmap P1-2 尾注），本节只覆盖导出。

---

## 5. 新建模块脚手架（newmodule）

一条命令生成新业务模块的全部骨架，并原子地改挂 4 个既有文件；任一步失败**全有或全无**
（已写文件删除 + 既有文件从写前备份回滚）。设计与触点细节见
[`superpowers/specs/2026-10-19-module-template-design.md`](superpowers/specs/2026-10-19-module-template-design.md)，
本文只记用法。

### 5.1 用法

```bash
make new-module NAME=demo ENTITIES=item                    # 主入口
make new-module NAME=demo ENTITIES=item,category ARGS="-dry-run"   # 只看计划不落盘
go run ./tools/newmodule -name=demo -entities=item -skip-frontend  # 直调 CLI
```

| 参数 | 必填 | 说明 |
| --- | --- | --- |
| `-name` | ✅ | 模块名（小写 snake_case） |
| `-entities` | ✅ | 逗号分隔初始实体（单数小写下蛇），每个生成最小 CRUD 骨架 |
| `-menu-prefix` | ❌ | 菜单/路由前缀，默认 = `-name`；**自定义值会被显式拒绝**（模板写死 `/<module>`，设置与产物不一致属陷阱） |
| `-dry-run` | ❌ | 只打印 ChangeSet 计划，不落盘（同时跳过 build 验证） |
| `-skip-frontend` | ❌ | 只生成后端 |
| `-skip-build` | ❌ | 跳过 `go build ./...` 验证 |

### 5.2 触点（流水线 1 校验 → 2 内存构建 → 3 落盘 → 4 验证 → 5 报告）

| # | 触点 | 处理 |
| --- | --- | --- |
| 1 | `modules/<name>/` 全套目录（module/controller/service/repository/model + migration_source） | 新建 |
| 2 | `alexgo-server/cmd/main.go` | AST 插 import + FxModule 装配 |
| 3 | `alexgo-server/configs/config.yaml` | `yaml.Node` 插 `modules.<name>: true`（保注释保排版） |
| 4 | `modules/system/service/permission_routes.go` | map 字面量按字母序插入权限码→路由条目 |
| 5 | 菜单种子 | system 模块 migrations 新建 SQL 对（幂等 NOT EXISTS：1 目录 + N 页面；**不含角色绑定**，启动 seed 全量 `SetRoleMenus` 兜底） |
| 6 | 前端 api + views | 新建（示例实体复用 codegen 引擎产物） |
| 7 | `docs/modules.md` | **不自动改**，收尾报告输出追加内容，留人工 |

两条免改结论：前端 `accessMode: 'backend'`（路由来自 `/menu/all`，无静态路由文件）；
`seed.go` 每次启动全量绑 admin 种子角色 + 重建策略（菜单落库即授权，无需手工绑）。

### 5.3 收尾报告

stdout 输出（`tools/newmodule/report.go`）：`✔ 新建模块 <name>（实体: ...）` + 新建文件清单 +
修改文件清单 + **手工项**（`docs/modules.md` 补条目、菜单名是英文占位可后台改中文、
如需独立部署参照 `modules/member/cmd/main.go`、回滚步骤）+ 验证结果
（`go build ./... 通过` / `跳过` / dry-run 标注）。失败时 `✘ <原因>` 走 stderr、退出码 1，
已按全有或全无回滚。

### 5.4 验收

```bash
CGO_ENABLED=0 go test ./tools/newmodule/...   # 单测 + golden 路径清单钉触点
make new-module NAME=demo ENTITIES=item ARGS="-dry-run"   # 干跑，确认计划后再实跑
```

---

## 6. scorecard 打牌记账（运维要点）

模块功能、端点、数据模型见 [modules.md §11](modules.md#11-scorecard--打牌记账模块)；
本节只记落地开关、日常操作与本地库维护。

### 6.1 开关与鉴权

- `alexgo-server/configs/config.yaml` → `modules.scorecard: true`；置 `false` 时
  `RegisterRoutes` 直接返回，两组路由整体消失（其余模块不受影响）。
- app 端 `/api/app/scorecard/**`（18 条）：**member** Bearer token，handler 内自鉴权、
  不走 casbin；缺/坏 token 401、非 member 身份 403。
- admin 端 `/api/admin/scorecard/**`（5 条）：casbin 三前缀
  `scorecard:room` / `scorecard:record` / `scorecard:friend`，授权走 §1 的
  菜单管理 → 角色授权（菜单行落库后由启动 seed 全量绑 admin 并重建策略）。

### 6.2 监管操作（前端 3 页 + 详情子页）

- `/scorecard/rooms`：状态下拉（全部/进行中/已结算/已关闭）+ 房号输入精确筛（`GET /api/admin/scorecard/rooms?status=&room_code=`，空值不传参）→ 「详情」跳 `/scorecard/rooms/detail?id=`
  （房行 / 成员 / 流水三区）→ 「强制关房」二次确认
  （`POST /api/admin/scorecard/rooms/:id/close`，**唯一运维写动作**；进行中与已结算
  房均可关，已关再关报 `30103`）。
- `/scorecard/records`、`/scorecard/friends`：只读分页（战绩可按 `user_id`/`room_id` 筛）。
- 详情页流水最多 **200 条**（服务端 cap）；要全量流水走 app 端
  `GET /api/app/scorecard/rooms/:id/logs` 分页。

### 6.3 超时关房（后台 ticker）

- 判定：`status=0 且 expire_time < now`，**每分钟**扫一轮 → 置 `status=2` 并写
  type=8 流水（`extra.timeout_close`）。
- 续期口径：只有**开房 / 进房 / 退房 / 配置变更 / 每笔支出**刷 `expire_time = now+2h`；
  **看快照不算操作、不续期**。
- 日志关键字 `scorecard sweep failed`（warn）= 该轮扫描失败，下轮自动重试，不影响请求链路。

### 6.4 菜单与本地库维护

- scorecard 菜单来自 system 迁移 `20261020000001_scorecard_menus.up.sql`
  （1 目录 + 3 页，**英文占位名**）——改中文在后台「菜单管理」改，不回改 SQL；
  房间详情页 `/scorecard/rooms/detail` **不进菜单**（前端 externalRoutes 静态注册）。
- 迁移器只跑 `Up()`（`pkg/migrate/runner.go`，`migrate.auto` 可关），按模块表
  `schema_migrations_<module>` 记版本、**无 checksum**：已记账的 up 永不重放，
  重复 `make migrate` 是空跑（`ErrNoChange`），不会重写既有数据。
- 本地库若残留脚手架版 6 页菜单（废弃 path `/scorecard/room_users|room_logs|friend_blacks`），
  洗净二选一：
  1. **手工删行**（最快，不动版本账）：
     `DELETE FROM menus WHERE tenant_id=0 AND type='menu' AND path IN ('/scorecard/room_users','/scorecard/room_logs','/scorecard/friend_blacks');`
  2. **down→up 一轮**：执行 `modules/system/migrations/20261020000001_scorecard_menus.down.sql`
     （超集幂等 DELETE，覆盖 7 个 path）→ 把 `schema_migrations_system` 的 `version`
     回退到 `20261016000002` 且 `dirty=0`（以实际库为准）→ `make migrate` 重跑 up，
     按 `NOT EXISTS` 重插 3 页形态；重启进程让 seed 重建策略并绑 admin。
