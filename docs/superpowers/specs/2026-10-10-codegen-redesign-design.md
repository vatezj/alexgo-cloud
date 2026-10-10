# 代码生成系统重设计（对齐 yudao-cloud）· Design Spec

- 日期：2026-10-10
- 状态：待评审
- 参考：yudao-cloud codegen 架构分析（数据库元数据 → 中间数据模型 → Velocity 模板渲染 → 后处理输出）

## 1. 背景与目标

alexGo-cloud 现有 3 套重叠的生成器：

| 工具 | 输入 | 现状 |
|---|---|---|
| `scripts/crud_generator.go` | proto（正则解析） | `make generate-crud` |
| `tools/dbgen` | MySQL 元数据 | `make generate-db-crud` |
| `scripts/db_crud_generator.go` | MySQL 元数据 | 与 dbgen 几乎重复，无 make 引用 |

模板共 11 个 `.tpl`，只覆盖 Go 后端，无统一中间模型、无字段级配置、无管理界面、无前端生成。

**目标（D：全面对齐 yudao）**：重建一套统一的代码生成系统——统一引擎 + 后台配置界面 + vben5_antd 前端生成 + 单表/树表/主子表全模式 + 可开关的测试生成。

### 已确认的需求基线

| 决策点 | 结论 |
|---|---|
| 总体范围 | D：全面对齐 yudao（引擎 + 后台界面 + 前端 + 高级表结构） |
| 代码落地方式 | C：后台预览 + ZIP 下载 与 CLI 写入仓库并存，共用同一引擎与模板 |
| 服务端归属 | A：新建 `modules/infra` |
| 前端模板覆盖 | A：只做 `apps/web-antd`（vben5 + ant-design-vue），目录保留 front-type 扩展维度 |
| 高级特性范围 | C：单表 + 树表 + 三种主子表模式全部进第一期 |
| 旧生成器处置 | A：新引擎验证通过后删除 3 套旧工具与旧模板 |
| 测试模板 | A：可配置开关 `unit-test-enable`，默认开启，前后端测试骨架都生成 |
| 数据库范围 | A：仅 MySQL；元数据读取抽象为 `MetadataReader` 接口保留扩展位 |
| 架构方案 | 方案一：引擎独立成 `pkg/codegen` 纯库，`modules/infra` 做宿主，CLI 复用同一库 |

### 非目标（YAGNI，明确不做）

- 多数据库（PostgreSQL/Oracle 等）——仅留接口
- 多前端框架——仅 vben5_antd
- yudao 的 `DataGenerator` Mock 造数——预览直接渲染模板，不需要
- 配置仓库 YAML 化（GitOps 式）——配置存 DB
- 租户隔离——codegen 为系统级功能

## 2. 整体架构与数据流

```
┌─────────────── 管理后台（web-antd）───────────────┐
│  「基础设施 → 代码生成」菜单页                      │
│  表清单 → 导入 → 字段配置 → 模板配置 → 预览 → 下载  │
└──────────────────────┬──────────────────────────┘
                       │ admin REST API（RBAC 权限点）
┌──────────────────────▼──────────────────────────┐
│ modules/infra（新模块，fx 注册进现有进程）          │
│  controller → service（编排） → repository        │
└──────┬───────────────────────────────┬──────────┘
       │ ①配置读写                      │ ②生成请求
       ▼                               ▼
 codegen_table /               ┌──────────────────────┐
 codegen_column 配置表          │  pkg/codegen（纯库）   │
 (MySQL，快照+可编辑配置)        │  metadata → builder   │
                               │  → template → post    │
┌──────────────────────┐       └──────────┬───────────┘
│ information_schema    │◄──③元数据读取     │
│（业务库 MySQL）        │                  │④渲染产物（内存文件列表）
└──────────────────────┘       ┌──────────▼───────────┐
                               │ 落地：预览(JSON) /    │
                               │ ZIP 下载 / CLI 写盘   │
                               └──────────────────────┘
```

**两条使用链路，共用一次引擎调用：**

1. **在线链路**：后台页面 → `POST /admin-api/infra/codegen/preview` → 引擎渲染 → 返回文件列表（预览）；`POST .../download` → 同一份渲染结果打成 ZIP。
2. **CLI 链路**：`make generate-crud MODULE=order` → `tools/codegen` 读同一张配置表（或 `--dsn` 现场读元数据）→ 引擎渲染 → 按"已存在不覆盖、`--force` 才覆盖"写入 `modules/**`。

**关键约定：**

- 配置表存"**元数据快照 + 生成配置**"（yudao 同款）：导入时快照表结构，表结构变化后可"同步"，用户配置不丢。
- 引擎无状态：输入 = 配置表行 + 元数据，输出 = `[]GeneratedFile{Path, Content}`。预览、ZIP、CLI 是消费同一输出的三种方式。
- 依赖方向：`modules/infra` → `pkg/codegen`；`pkg/codegen` 只依赖 MySQL 驱动和标准库，不 import `modules/*`、不依赖 fx。

## 3. `pkg/codegen` 核心组件与模板体系

### 包结构与职责

```
pkg/codegen/
├── metadata/
│   ├── reader.go          // MetadataReader 接口：ListTables / ReadTable
│   └── mysql.go           // MySQL 实现：information_schema（表/列/注释/主键/自增）
├── model/
│   ├── table.go           // Table：表名、类名、模块、业务名、模板类型、表类型
│   ├── column.go          // Column：列名、Go类型、JSON名、注释、控件、列表/表单/查询开关
│   └── enums.go           // TemplateType、FrontType、HtmlType、DataType
├── builder/
│   ├── builder.go         // 元数据 → model.Table（CodegenBuilder 同款职责）
│   ├── naming.go          // 表名前缀→模块、orders→Order、order_item→OrderItem
│   └── typing.go          // MySQL类型→Go类型/前端控件映射
├── template/
│   ├── engine.go          // text/template 渲染、模板族选择（模板类型×表角色×前后端）
│   ├── bind.go            // 构建模板绑定数据
│   └── paths.go           // 输出路径映射（SERVER_TEMPLATES/FRONT_TEMPLATES 对位）
├── postprocess/
│   ├── golang.go          // go/format.Source（失败=生成错误，不静默）
│   ├── web.go             // Vue/TS 输出整理（可选调 oxfmt）
│   └── collide.go         // 类名/路径碰撞校验（prettyCode 对位）
├── generate.go            // Generate(cfg) []GeneratedFile —— 唯一入口
└── generate_test.go …     // 各层表驱动测试
```

### 模板体系

```
pkg/codegen/templates/
├── server/go/                     # Go 后端（text/template 语法）
│   ├── single/   model|repository|service|controller_admin|controller_app|module_register|test
│   ├── tree/     （含 validateParent 逻辑的变体）
│   └── main_sub/ 主表+子表变体（子表嵌入/独立列表）
└── front/vben5_antd/              # 前端（front-type 维度，仅此一套）
    ├── single/   api.ts | list.vue | form.vue | drawer.vue | types.ts | index.ts | test(可选)
    ├── tree/     （树选择/递归表格变体）
    └── main_sub/ standard|embedded|erp 三种模式变体
```

- 模板名 → 输出路径的映射放 `paths.go`（Go 代码），模板文件不感知路径。
- 模板族选择：`模板类型(single/tree/main_sub) × 角色(主表/子表) × front-type` → 目录 → 渲染该目录全部模板。
- 测试模板受 `unit-test-enable` 开关控制（`configs/config.yaml`，yudao 同款做法，后台只读展示）。
- 旧 `scripts/templates/*.tpl` 迁移时**参考字段清单与代码风格，文件全部重写**（旧模板无子表/树表/前端概念）。

### 与 yudao 概念对位

| yudao | 本方案 |
|---|---|
| CodegenEngine | `template.Engine` + `generate.go` |
| CodegenBuilder | `builder.Builder` |
| CodegenTableDO / ColumnDO | `model.Table` / `model.Column` |
| SERVER_TEMPLATES / FRONT_TEMPLATES | `paths.go` 路径映射 |
| prettyCode | `postprocess`（go/format + 碰撞校验） |
| DataGenerator | 不做 |

## 4. `modules/infra` — 配置存储与 admin API

### 模块结构（对齐现有 `modules/*/` 惯例）

```
modules/infra/
├── module.go            # fx 模块注册（同 system/member/order 模式）
├── model/               # CodegenTable、CodegenColumn（gorm）
├── repository/          # 配置表读写
├── service/             # 编排：导入/同步/预览/ZIP（调 pkg/codegen）
├── controller/          # admin REST（对齐现有路由前缀与响应包裹惯例）
└── migrations/          # 建表迁移 + 菜单/权限点 seed（沿用 migration_source.go 模式）
```

### 配置表

**`codegen_table`**（= CodegenTableDO）：`id`、`table_name`(唯一)、`table_comment`、`module`、`business_name`、`class_name`、`template_type`(1单表/2树表/3主子表)、`front_type`(1=vben5_antd 预留)、`parent_table_id`(子表行指向主表行)、`remark`、`created_at/updated_at/deleted_at`。

**`codegen_column`**（= CodegenColumnDO）：`table_id`、`name`、`type`、`comment`、`go_type`、`json_name`、`is_pk`、`auto_increment`、`nullable`、`html_type`(Input/Textarea/InputNumber/Select/Switch/DatePicker…)、`list_enable`、`form_enable`、`query_enable`、`query_operation`(eq/like/between)、`list_required`、`form_required`、`dict_type`(预留)、`sort_order`、时间戳。

### 导入/同步语义

- **导入**：浏览业务库 `information_schema` 未导入的表 → 选表 → builder 按命名/类型规则初置配置行（用户可再改）。
- **同步**：列 diff——新增列追加配置行（默认开关关闭）、消失列标记 `deprecated`（不物理删，保护已有配置）、类型/注释刷新快照但**保留用户改过的开关**。

### admin API（前缀对齐现有 admin-api 惯例）

```
GET    /codegen/db-tables                  # 浏览可导入的库表（含已导入标记）
POST   /codegen/import                     # 导入选中表 → 生成配置行
GET    /codegen/tables                     # 分页：已导入表配置列表
GET    /codegen/tables/{id}                # 表详情
PUT    /codegen/tables/{id}                # 更新表级配置
DELETE /codegen/tables/{id}                # 删除（级联删字段配置）
POST   /codegen/tables/{id}/sync           # 同步表结构
GET    /codegen/tables/{id}/columns        # 字段配置列表
PUT    /codegen/tables/{id}/columns        # 批量保存字段配置
POST   /codegen/preview                    # body: tableIds[] → []{path, content}
POST   /codegen/download                   # body: tableIds[] → ZIP 流
```

**权限**：新菜单「基础设施 → 代码生成」+ 按钮级权限点，随迁移 seed 进 RBAC；系统级功能，不做租户隔离。

## 5. 后台页面 + CLI 落盘 + 旧工具迁移

### 后台页面（`apps/web-antd/src/views/infra/codegen/`）

1. **表配置列表页**（主页面，路由由 seed 菜单驱动）
   - Tab「数据源表」：可导入库表 + 已导入标记 + 导入按钮；Tab「已导入配置」：分页表格（表名/类名/模块/模板类型）
   - 行操作：配置、同步、预览、下载、删除
2. **配置详情**（Drawer 抽屉，两栏 Tab）
   - Tab1 基础配置：模块/业务名/类名/模板类型/前端类型/备注
   - Tab2 字段配置：可编辑表格（列表/表单/查询开关、控件下拉、必填、操作符、字典），仿 yudao 字段编辑网格
3. **预览 Modal**：左侧文件树（后端/前端分组）+ 右侧代码高亮 + 底部「下载 ZIP」

API 封装：`src/api/infra/codegen.ts`（对齐 `api/admin.ts` request 封装惯例）。

### CLI（`tools/codegen`）

```
go run ./tools/codegen --module=order --tables=order_item   # 从配置表生成（日常主用法）
go run ./tools/codegen --module=order                       # 模块下全部已导入表
go run ./tools/codegen --dsn=... --tables=t1 --import       # 现场读元数据并写入配置表
# 通用：--force 覆盖已存在文件（默认跳过并列出跳过清单）
```

- 写盘规则沿用旧约定：已存在不覆盖，`--force` 才覆盖；module_register 类聚合注册文件用标记块（`// <codegen:xxx>` … `// </codegen:xxx>`）做幂等插入/更新——比旧工具更稳（旧工具只能整跳过或整覆盖）。
- Makefile：`generate-crud` / `generate-db-crud` 改指向 `tools/codegen`；`generate-all` 保持 `proto + generate-crud` 语义，用法不变、实现替换。

### 旧工具迁移（对应决策 A）

1. 新引擎通过测试 + 试生成对比验收后：
2. 删除 `scripts/crud_generator.go`、`scripts/db_crud_generator.go`、`tools/dbgen`、`scripts/templates/`。
3. README / architecture.md 相关段落改写为新工具用法。

## 6. 错误处理

**`pkg/codegen` 分层类型化错误**（`errors.Is/As`，带上下文）：

| 错误类 | 触发点 | 上下文 |
|---|---|---|
| `ErrMetadataUnreachable` | 元数据读取失败 | DSN 脱敏后的目标库信息 |
| `ErrTableNotFound` / `ErrColumnInvalid` | 配置引用不存在的表/列 | 表/列名 + 配置行 id |
| `ErrTypeMappingUnknown` | MySQL 类型无法映射 | 列名 + 原始类型 |
| `ErrTemplateMissing` | 模板族/模板文件缺失 | 模板类型 × 角色组合 |
| `ErrRender` | 模板渲染失败 | 模板文件名 + 行号 |
| `ErrFormat` | `go/format` 失败 | 输出路径 + 语法错误位置（失败即报错，不产出未格式化垃圾） |
| `ErrPathCollide` | 路径/类名冲突 | 冲突双方路径 |

- **聚合**：一次生成收集所有错误再返回（multierror 风格），改一轮配置能解决的不逼用户跑多次。
- **service 层**：包装为 `pkg/errors` 业务错误码，对齐现有 admin API 错误响应惯例。
- **CLI**：错误逐条打印 stderr（含修复建议），非零退出；结束输出摘要 `生成 N / 跳过 M / 失败 K`。
- **副作用边界**：预览/ZIP 纯内存，引擎失败无副作用；CLI 先全部渲染成功再逐文件写，不存在写一半状态。

## 7. 测试策略（TDD，对齐仓库测试密度）

| 层 | 手段 |
|---|---|
| `builder/naming` `builder/typing` | 表驱动单测：表名→类名/模块、MySQL 类型→Go/控件、树表/主子表命名 |
| `template` + `postprocess` | **Golden 测试**：fixture 表 → 渲染 → `testdata/` 期望输出逐字节比对（go/format 后）；覆盖 单表/树表/主子表 × 后端/前端 |
| `metadata/mysql` | 接口层 fake reader 测上层；MySQL 实现 docker-compose 集成测试（`-tags integration`） |
| `modules/infra` service | fake repository + 真引擎（fixture 模板），测导入/同步 diff/预览编排 |
| controller | 按现有 `*_test.go` HTTP 测试惯例 |
| **端到端冒烟** | `make codegen-smoke`：order 真实表生成到临时目录 → `go build`/`go vet` 通过 → 前端产物 golden 比对 |

前端 Vue 产物第一期以 golden 比对为准（vue-tsc 全量检查留作后续 CI 选项）。

## 8. 实施里程碑

| # | 里程碑 | 内容 | 验收标准 |
|---|---|---|---|
| **M1** | 引擎地基 | `pkg/codegen` 全组件 + 单表后端模板族（含测试骨架）+ CLI 草稿模式（`--dsn` 直读写盘） | golden 通过；order 真实表产物 `go build ./...` 通过 |
| **M2** | 配置持久化 | `modules/infra`：迁移建表 + 导入/同步 + admin API + 菜单权限 seed | HTTP 测试：导入→改配置→同步 diff 通过 |
| **M3** | 在线链路打通 | 预览/ZIP API + 后台三页面 + CLI 改读配置表 + `make generate-*` 切换 | 浏览器完成"导入→配置→预览→ZIP"；CLI 与旧命令用法等价 |
| **M4** | 前端生成 | vben5_antd 单表模板族（api/list/form/drawer/types + Vitest 测试骨架）+ 接入预览与 CLI | golden 通过；生成页在 web-antd 本地可打开 |
| **M5** | 树表 | 模板类型=2 后端+前端变体 + 页面树交互配置 | 树表族 golden + 冒烟 |
| **M6** | 主子表 | 三种模式（标准/内嵌/ERP）后端+前端变体 + 关联配置 | 三模式 golden + 冒烟 |
| **M7** | 收尾 | `make codegen-smoke` + 删除 3 套旧工具旧模板 + 文档改写 | 冒烟绿；删除后 CI 绿；文档更新 |

**依赖**：M1→M2→M3 严格串行；M4 在 M3 后；M5/M6 依赖 M4；M7 收尾。

**贯穿约定**：每里程碑 TDD；每个里程碑独立提交进 main（项目约定直接在 main 工作）。
