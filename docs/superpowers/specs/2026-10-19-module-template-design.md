# 新建模块统一脚手架（newmodule）设计

日期：2026-10-19
状态：已确认（四节设计逐节获批）

## 背景与目标

alexGo-cloud 是模块化单体（fx 装配 + gin）。目前新建一个模块需要手工触碰 7 个位置（新目录、main.go 装配、config 开关、权限码、菜单种子、前端 api/views、文档），易漏易错。本设计提供统一脚手架命令，以后所有新模块按它创建。

**成功标准**：`make new-module NAME=x ENTITIES=y` 一条命令后，`go build` 通过、启动即有菜单和可 CRUD 的页面；漏改任何触点视为失败。

## 已澄清的需求

| 问题 | 结论 |
|---|---|
| 覆盖范围 | 全链路（后端骨架 + 权限码 + 前端 api/views + 菜单；文档更新以报告手工项输出） |
| 载体形态 | 脚手架命令（Go 实现） |
| 基准模块 | member（admin/app 双前缀）；但 micro 独立 cmd 入口与跨模块接口映射**不生成**（见 §2.4） |
| 既有文件修改 | 自动改 + 报告；失败回滚（全有或全无） |

## 1. 总体架构与命令接口

### 1.1 工具形态

```
tools/newmodule/
├── main.go          # CLI 入口：解析 flags → 调 scaffold → 打印报告
├── scaffold.go      # 编排器：校验 → 内存构建 → 落盘 → 报告
├── editor.go        # 三个既有文件的专用结构化编辑器
├── report.go        # 结果报告（新建/修改/手工项）
└── templates/       # go:embed 模板目录（壳层文件；实体层复用 codegen）
```

- 包名 `newmodule`，位于 `tools/` 下，与 `tools/codegen` 平级、互不依赖；
- 模板用 `text/template` + `go:embed`，变量为模块名/业务名/实体名。

### 1.2 CLI 接口

```bash
make new-module NAME=product ENTITIES=product          # 主入口
go run ./tools/newmodule -name=product -entities=product -dry-run
```

| 参数 | 必填 | 说明 |
|---|---|---|
| `-name` | ✅ | 模块名（小写 snake_case，如 `product`） |
| `-entities` | ✅ | 逗号分隔的初始业务实体（如 `product,category`），生成最小 CRUD 骨架 |
| `-menu-prefix` | ❌ | 菜单/路由前缀，默认 = `-name` |
| `-dry-run` | ❌ | 只打印计划（ChangeSet 清单），不落盘 |
| `-skip-frontend` | ❌ | 只生成后端（调试用） |

### 1.3 执行流水线（全有或全无）

```
1. 校验      name 合法性、目标模块不存在、实体名不与保留字冲突
2. 内存构建  渲染全部新文件 → map[path][]byte
            读取 3 个既有文件 → 编辑器产出新内容（内存）
3. 落盘      任一步失败：已写文件删除 + 既有文件回滚（写前备份）
4. 验证      go build ./...（失败则回滚并报错）
5. 报告      stdout：新建/修改文件清单 + 剩余手工项
```

### 1.4 触点清单

| # | 触点 | 处理方式 |
|---|---|---|
| 1 | `modules/<name>/` 全套目录 | 新建 |
| 2 | `alexgo-server/cmd/main.go` | 插 import + FxModule 装配（自动，AST） |
| 3 | `alexgo-server/configs/config.yaml` | 插 `modules.<name>: true`（自动，yaml.Node） |
| 4 | `modules/system/service/permission_routes.go` | 插权限码→路由条目（自动，AST） |
| 5 | 菜单种子 | 在 system 模块 migrations **新建**迁移 SQL 对（自动） |
| 6 | 前端 api + views | 新建（自动生成） |
| 7 | `docs/modules.md` | 报告输出追加内容（**不自动改**，留给人工） |

**两条免改结论（已核实）**：

- 前端 `accessMode: 'backend'`（`preferences.ts`），路由来自后端 `/menu/all`，`access.ts` 用 `import.meta.glob('../views/**/*.vue')` 解析组件——**无静态路由文件要改**；
- `modules/system/service/seed.go` 每次启动 `SetRoleMenus` 全量绑 admin 种子角色 + `RebuildPolicies`——**菜单落库后下次启动自动授权，无需手工绑角色**。

## 2. 生成物清单与示例实体

### 2.1 新建目录结构

以 `name=product`、`entities=product,category` 为例：

```
modules/product/
├── module.go                    # FxModule + RegisterRoutes（admin/app 双前缀）
├── migration_source.go          # 迁移源（GetMigrationPath / GetModuleName）
├── configs/config.yaml          # 模块私有配置（空骨架）
├── migrations/
│   ├── 20261019000001_products.up.sql / .down.sql      # 每实体一对
│   └── 20261019000002_categories.up.sql / .down.sql
├── model/product.go             # 每实体一文件
├── repository/product.go
├── service/product.go + product_test.go
└── controller/
    ├── admin/product.go         # List/Create/Update/Delete
    └── app/product.go           # app 端最小只读（member 基准的双前缀）
```

前端：

```
apps/web-antd/src/
├── api/product/                 # 按 codegen 风格分目录
│   ├── product/types.ts + api.ts + index.ts
│   └── category/types.ts + api.ts + index.ts
└── views/product/
    ├── ProductListPage.vue      # 菜单 component: views/product/ProductListPage
    └── CategoryListPage.vue
```

菜单种子 SQL 落在 **system 模块**的 `modules/system/migrations/`（新文件，不改既有文件），1 条 dir + N 条 menu，component 路径与 views 文件严格一致。

### 2.2 子决策 A：示例实体复用 codegen 引擎（已确认）

生成的 model/repository/service/controller **不手写模板**：在内存里构造 `model.Table`（借 `pkg/codegen/builder` 的 typing 规则：`MySQLTypeToGo` / `MySQLTypeToHTML` / `BuildGormTag`），调 `codegen.Generate` 产出。

- 生成物与 `make generate-crud` 产物同源同风格，后续加实体无缝衔接；
- service_test.go / 前端 api.test.ts 免费获得（codegen 模板自带）；
- 维护只在 codegen 一处，避免双模板漂移。

模块壳（module.go / migration_source.go / 迁移 SQL / 前端页面壳 / 控制器路由接线）是 newmodule 自己的模板——codegen 不覆盖这些触点。

### 2.3 子决策 B：前端 api 放子目录（已确认）

order 的 api 挤在 `api/admin.ts`（455 行）是历史产物；新模块按 codegen 风格放 `api/{module}/{business}/`，不再养大文件。

### 2.4 不生成的东西（YAGNI）

- **micro 独立 cmd 入口**：member 的 `cmd/main.go` 是 C 端登录特殊产物；新模块默认 mono。报告注明"如需独立部署参照 `modules/member/cmd/main.go` 手工加"；
- **跨模块接口映射**：每个模块映射的接口不同，无法模板化；module.go 里留注释占位提示。

## 3. 既有文件编辑器

三个文件各配专用结构化编辑器，全部先在内存构建、定位失败即整体中止。

### 3.1 `alexgo-server/cmd/main.go` — AST 定位插入

用 `go/parser` 解析后 AST 级插入，再 `postprocess.FormatGo`：

1. **import 块**：找最后一个 `"alexGo-cloud/modules/..."` import，按字母序插入 `"alexGo-cloud/modules/<name>"`；
2. **opts 列表**：找 `opts := []fx.Option{` 中已有的 `xxx.FxModule,` 行，在其后（system/order 之后、member 条件分支之前）插入 `<name>.FxModule,`；
3. 若存在 member 式 `if cfg.Deployment.Mode != "micro"` 分支，装配行插在该分支之前（默认 mono）。

失败条件：找不到 import 块或 opts 字面量 → 报错中止，不落盘。

### 3.2 `alexgo-server/configs/config.yaml` — yaml.Node 保注释插入

用 `goccy/go-yaml`（indirect 升 direct）解析为 `yaml.Node`：

- 定位 `modules:` mapping 节点，追加 entry key=`<name>` value=`true`；
- 序列化保留原有注释与排版。

失败条件：无 `modules:` 节点、或 `<name>` 已存在 → 报错中止。

### 3.3 `modules/system/service/permission_routes.go` — map 字面量插入

用 `go/parser` 定位 `permissionRoutes = map[string][]string{...}` 的 `CompositeLit`：

- 按 key 字母序位置插入每个实体一条：
  ```go
  "product:product": {"/api/admin/product/products", "/api/admin/product/products/*"},
  ```
  key = `<module>:<entity>`，路由取 plural，必含 `/*` 变体（遵循文件头注释的 keyMatch2 规则）；
- AST 重打印 + `FormatGo`。

失败条件：找不到 `permissionRoutes`、key 已存在 → 报错中止。

### 3.4 回滚语义（全有或全无）

```
阶段一（纯内存）：渲染新文件 + 编辑 3 个既有文件 → 全部装进 ChangeSet{creates, patches}
阶段二（落盘）  ：写新文件；改既有文件（写前原文备份进内存）
任一步失败     ：已创建文件删除；既有文件按内存备份回写
-dry-run       ：只打印 ChangeSet 清单，永不落盘
```

### 3.5 菜单种子 SQL（触点 5）

在 system 模块 migrations **新建**一对 up/down：

- 版本号 = 当前 UTC 日期 + 序号（如 `20261019000001`），幂等命名；
- up：`INSERT ... SELECT ... WHERE NOT EXISTS` 幂等插 dir + menu（tenant_id=0，parent 经父目录 JOIN 定位）；参考 `20261009000001_menus_vben_backfill.up.sql` 模式；
- down：按 path 精确删除；
- admin 角色绑定不做——启动 seed 全量 `SetRoleMenus` 兜底。

## 4. 测试策略与验收

### 4.1 单元测试（`tools/newmodule/*_test.go`）

| 测试对象 | 用例 |
|---|---|
| main.go 编辑器 | 读仓库现文件（相对路径）→ 插入后可被 `go/parser` 解析、import 有序、`FxModule` 出现一次；锚点缺失 → 错误 |
| config.yaml 编辑器 | 读现文件 → `modules.<name>: true` 出现、原注释保留；重名 → 错误 |
| permission_routes 编辑器 | 同上；key 字母序正确；重名 → 错误 |
| 菜单种子渲染 | up/down SQL 内容断言（幂等 guard、path、component 与 views 一致） |
| 校验逻辑 | 非法 name（大写/连字符/保留字）、模块已存在、实体名冲突 |

编辑器测试输入从仓库现文件读取（`../../alexgo-server/...` 相对路径），不复制副本——文件变了测试自动跟进。

### 4.2 集成测试（golden + 干跑）

- **golden 清单**：`-dry-run` 跑完整流水线（`name=demo`），ChangeSet 文件路径清单与 golden 快照比对——触点增漏时 golden 变红，提示更新快照；
- **临时目录落盘**：`t.TempDir()` 构造最小仓库骨架（拷贝 3 个既有文件），跑完整落盘断言文件树完整；注入失败（第 2 个编辑器报错）断言既有文件已回滚、无残留新文件。

### 4.3 手工验收 checklist

```
1. make new-module NAME=demo ENTITIES=item
2. go build ./...                          # 通过
3. 启动服务：迁移执行、菜单落库
4. admin 登录前端：菜单出现、列表页可打开、CRUD 可点
5. 清理：删 modules/demo + 回滚 3 个既有文件 + 删菜单迁移对
```

第 5 步删除路径在收尾报告里输出。

### 4.4 明确不做

- 不做 newmodule 端到端 CI 流水线（手工验收足够，CI 的 `go build`/`go test` 覆盖生成物编译）；
- 不做生成物内容级 golden 比对（codegen 模板变产物就变，内容比对是维护负担；只测清单与可编译性）。

### 4.5 收尾报告格式（stdout）

```
✔ 新建模块 product（实体: product, category）
新建:
  modules/product/module.go
  ...（约 25 个文件）
修改:
  alexgo-server/cmd/main.go                 (import + opts)
  alexgo-server/configs/config.yaml         (modules.product: true)
  modules/system/service/permission_routes.go (2 条)
新建(菜单种子):
  modules/system/migrations/20261019000001_product_menus.up.sql
手工项:
  - docs/modules.md: 追加 product 模块条目（内容见下）
  - 如需独立部署: 参照 modules/member/cmd/main.go
验证: go build ./... 通过
```

## 风险与缓解

| 风险 | 缓解 |
|---|---|
| main.go / permission_routes.go 结构变化导致编辑器锚点失效 | 编辑器测试读仓库现文件，结构一变测试即红 |
| codegen 模板升级改变生成物风格 | 预期行为（同源），不锁内容 |
| 菜单种子迁移与 system 既有迁移版本号冲突 | 版本号取 UTC 日期 + 序号，冲突时递增 |
| `-dry-run` 与真实落盘行为不一致 | ChangeSet 构建逻辑共用，dry-run 只跳过落盘阶段 |
