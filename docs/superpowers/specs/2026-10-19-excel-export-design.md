# Excel 导出（子项目 C / P1-2）设计

> 日期：2026-10-19
> 状态：已与用户逐节确认（方案 A + 四决策 + 三节设计）
> 范围：`pkg/excel` 薄共享层、system 用户 / member 用户 / order 订单三域列表导出、前端两颗导出按钮
> 不在范围：任何导入（留小票）、独立 `:export` 权限码、筛选 query 参数、member/order 前端页面、xlsx 样式/多 sheet、字典翻译列

## 背景与目标

现状盘点（2026-10-19）：仓库零 excelize 依赖、零 Excel 代码。下载先例两处——codegen 流式 zip（信封豁免）与 P1-1 file 受控下载。三域列表端点齐全但**均无筛选**（system 用户全量无分页、member 仅 page/size、order 全量无分页）；member 前端无页面无 API。

目标（roadmap P1-2）：三域列表可导出 xlsx。roadmap 标题「导入导出」，内容栏只定义导出——**本期只做导出**，导入留后续小票（用户明确选择）。

## 决策记录（用户逐项确认）

| 决策点 | 选择 |
|---|---|
| 范围 | 只做导出；导入留小票 |
| 传输形态 | 流式直出（excelize 写 `http.ResponseWriter`），不走 P1-1 落盘中转——导出是一次性消费，落盘是给「反复取」的场景（YAGNI） |
| 导出口径 | 全量匹配当前筛选（本期筛选为空 → 实质为租户内全量）+ 行数保险丝 5 万；将来列表加搜索，导出端点同步收同形 query 参数 |
| 前端挂载 | 只挂现有页面（system 用户 + 订单）；member 导出**后端端点照做**（curl 可验收），前端按钮留 member 页面立项时接 |
| 共享层切法 | 方案 A：`pkg/excel` 薄层（写流 + 上限），列映射各域私有 |

## 1. 架构与组件

```
前端导出按钮（system 用户页、订单页）
      │ GET /admin/{system/users|order/orders}/export
      ▼
各域 controller：ExportUsers / ExportMemberUsers / ExportOrders
  · svc.Export(ctx) 取全量行（租户插件照常隔离）
  · 行 → 列映射（各域私有）
  · pkg/excel.Write 写响应流（信封豁免，attachment）
      ▼
pkg/excel（新，薄层 ~150 行）
  · Write(ctx, w, spec)：表头 + 数据行 → io.Writer
  · 上限：len(Rows) > MaxExportRows → errs.New(CodeExportTooManyRows)（写任何字节之前）
  · Content-Disposition RFC5987（照 file_controller 先例，mime.FormatMediaType）
```

文件面：

| 层 | 文件 |
|---|---|
| pkg | `pkg/excel/excel.go` + `excel_test.go`（新）；`pkg/errors/code.go` 加 `CodeExportTooManyRows=30013`（3xxxx → HTTP 400，`statusFor` 自动映射） |
| system | `controller/admin/user.go` 加 `ExportUsers`；`module.go` 注册 `GET /users/export` |
| member | `controller/admin/member_user.go` 加 `ExportMemberUsers`；`module.go` 注册 |
| order | `controller/admin/order.go` 加 `ExportOrders`；`module.go` 注册 |
| 前端 | `admin.ts` 加 `exportUsers()/exportOrders()`（`requestClient.download`，照 `downloadCodegen` 先例）；两页面加按钮 |
| 依赖 | `github.com/xuri/excelize/v2`（唯一新依赖） |

**权限零改动**：`permissionRoutes` 既有键 `system:user` / `member:user` / `order:order`，模板 `/users/*`、`/member/users/*`、`/orders/*` 天然覆盖 `.../export` 子路径。不做独立 `:export` 按钮权限。

**service 层**：各域 service 加 `Export(ctx)`——与 `List` 同查询、不截断。member 的 `List(ctx,page,size)` 内部变体或 repo 层 `ListAll`，实现时定；spec 只钉行为等价。

**明确不做**：导入、独立权限码、筛选参数、member/order 页面、样式、多 sheet、字典翻译。

## 2. 数据流与列映射

1. 前端 → `GET .../export`（blob 下载）
2. controller：`svc.Export(ctx)`（tenant 插件隔离）
3. controller 组 `excel.Spec{Sheet名, 文件名, []Column{表头+取值闭包}, Rows}`
4. `pkg/excel.Write`：超限 → 信封 30013（**先查后写**）；否则 excelize 写流
5. 信封豁免与 codegen download 同款：写字节开始后错误只能中断连接

列映射（各域私有；状态导原始 int 不译字典；时间 RFC3339；Avatar/PasswordHash 不导出）：

| 域 | 列 |
|---|---|
| system 用户 | ID、用户名、昵称、邮箱、手机、性别、状态、部门ID、备注、登录IP、登录时间、创建时间 |
| member 用户 | ID、昵称、手机、状态、注册IP、登录IP、登录时间、创建时间 |
| order 订单 | ID、订单号、用户ID、产品ID、金额(分)、状态、创建时间、更新时间 |

上限 `MaxExportRows = 50000`（`pkg/excel` 包常量，不进 config——无调参需求）。文件名 `users_<RFC3339紧凑>.xlsx` / `member_users_...` / `orders_...`。

## 3. 错误处理

| 场景 | 处理 |
|---|---|
| 行数超限 | `errs.New(CodeExportTooManyRows)`（30013）→ `resp.Fail` → 信封 400 |
| 取行 DB 裸错 | 50000 信封（写流前，安全） |
| 写流中途 excelize 失败 | 无信封可发；zap warn + 中止连接（codegen zip 同款局限，注释注明）；不测中途 IO 断 |
| 未登录/无权限 | 既有 JWT 中间件（export 在 `/admin/**` 内） |
| 空表 | 合法：仅表头 xlsx，200 |

## 4. 测试与验收

| 层 | 覆盖 |
|---|---|
| `pkg/excel`（xlsx 读回） | 表头+行往返；空 Rows 仅表头；超限 `errs.Is(30013)`；列数与表头数不一致 → 返回 error（不 panic） |
| 各域 controller（httptest，照各自既有模式） | Content-Type/Disposition 断言；xlsx 读回断言行数与首行关键字段；member 租户隔离（照 `TestList_FiltersByTenant` 模式，**真注册 tenant 插件**——B 子项目 C2 的教训：裸 sqlite 钉不住插件层） |
| 前端 vitest | `exportUsers/exportOrders` 断言走 `requestClient.download`（mock） |
| 权限 | export 路径无 token → 401 case |

门禁：`CGO_ENABLED=0 go build/vet/test ./modules/... ./pkg/...` + `pnpm typecheck && pnpm test`。无新表，无迁移集成测。冒烟：curl 下载 xlsx 读回 + 前端两按钮（浏览器项留票，照 B 惯例）。

**Review Focus**（task 级盲区预警）：
1. export 路由确实落进 `permissionRoutes` 模板——用既有权限组测试钉住，不信「模板应该匹配」
2. 上限检查在写字节**之前**（先写后查会发出半个 200）
3. member 导出真走 tenant 插件（别在裸 sqlite 自证）
4. `request.ts:114` 的 Blob 错误特判——export 400（30013）应弹出文案而非乱码

## 非目标（一期明确不做，记为局限）

- Excel 导入（任何域）——后续小票
- 独立 `:export` 权限按钮（yudao 形态）——需要时加法
- 导出筛选参数——列表页现无筛选；将来加搜索时导出端口同步
- member 管理前端页面——独立小票
- 超大导出（>5 万行）、异步导出任务队列、xlsx 样式/多 sheet/公式
