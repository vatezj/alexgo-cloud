# Excel 导出（子项目 C / P1-2）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 三域列表（system 用户 / member 会员 / order 订单）可导出 xlsx：`pkg/excel` 薄共享层 + 三个 export 端点 + 前端两颗导出按钮，5 万行保险丝，权限零改动。

**Architecture:** 方案 A——`pkg/excel` 只做「表头+数据行 → xlsx 字节」与上限/文件名/Disposition 生成，列映射各域私有（controller 里 Column 闭包）。controller 复用既有全量取数方法（system `ListUsers` / order `List` 零接口改动；仅 member 加 `Export`/`ListAll`），先 `Build` 后 `ctx.Data`：Build 返回 error 时零字节已写出，信封安全（codegen download 同款豁免只发生在 `ctx.Data` 之后）。

**Tech Stack:** Go 1.25 / gin / gorm + glebarez/sqlite（纯 Go 测试）/ casbin keyMatch2 / `github.com/xuri/excelize/v2`（唯一新依赖）/ vue3 + @vben/requestClient.download + vitest。

**Spec:** `docs/superpowers/specs/2026-10-19-excel-export-design.md`（权威需求源；本计划是它的论证。冲突以 spec 为准，计划级细化见下）

**计划对 spec 草图的两处细化（评审时按此对照）：**

1. **`Build` 而非 `Write(w)`**：spec §1/§2 草图写 `Write(ctx, w, spec)` 写响应流。本计划改为 `Build[T](spec) ([]byte, error)` 返回字节、controller `ctx.Data`——与 codegen download 先例（内存字节 + ctx.Data，信封豁免注释）同形；excelize 无真流式 API（行数据本就在内存构建）；决策②「不经 P1-1 落盘」不变；Review Focus ②（先查后写）结构性成立。
2. **空 Columns 防护替代「列数与表头数不一致」**：spec §4 测试行假定 `[][]any` 行与独立表头。`Column[T]` 泛型 API 下行值由列闭包产出、不一致结构上不可表示；等价防护改为「空 Columns → `CodeParamInvalid`，不 panic」。

## Global Constraints

- 直接在 main 上工作：不建功能分支、不 push；代码提交与 docs/roadmap 提交分开。
- git 命令一律从仓库根 `/Users/alex/Desktop/goWork/alexGo-cloud` 跑（cwd 会滞留子目录）。
- 提交信息：conventional + 中文描述（`feat(scope): …`）。
- 门禁（Task 7 全量跑）：`CGO_ENABLED=0 go build ./modules/... ./pkg/... && CGO_ENABLED=0 go vet ./modules/... ./pkg/... && CGO_ENABLED=0 go test ./modules/... ./pkg/...`；前端 `cd apps/web-antd && pnpm typecheck && pnpm test`（本机 cgo 链接坏，CGO_ENABLED=0 强制）。
- 测试驱动 `github.com/glebarez/sqlite`（纯 Go）；真插件测试照 `file_repo_test.go`：`gormplugin.Register(db, gormplugin.Options{ExemptTables: []string{"tenants", "casbin_rule"}})`（与 main.go 生产一致）。
- 错误码 `CodeExportTooManyRows = 30013`（3xxxx → HTTP 400 由 `resp.statusFor` 自动映射，**勿改 resp**）；registry 文案「导出行数超过上限」；`errs.New(code, cause)`；`errs.Is(err, code)`。
- `pkg/excel.MaxExportRows = 50000`（包常量，不进 config）；MIME `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet`；文件名 `users_<yyyyMMdd_HHmmss>.xlsx` / `member_users_...` / `orders_...`。
- 导出内容口径：租户内全量（复用既有查询）；状态导原始 int 不译字典；时间 RFC3339（`excel.TimeRFC3339`，nil → 空串）；PasswordHash / Avatar / Password 一律不导出。
- **权限零改动**：不加 `:export` 权限码、不改 `permissionRoutes`、不改 seed——既有 `system:user` / `member:user` / `order:order` 的 `/users/*`、`/orders/*` 模板天然覆盖 export 子路径。
- 信封豁免仅限 attachment 字节流（codegen download 同款）；Build 返回 error 时必须还能发 JSON 信封（上限检查在任何字节之前）。
- 明确不做：Excel 导入（留小票）、独立 `:export` 权限按钮、筛选 query 参数、member/order 前端页面、xlsx 样式/多 sheet/字典翻译列、异步导出队列。
- 前端：只挂 system 用户页 + 订单页两按钮；member 导出后端端点照做（curl 可验收）。

## Review Focus

1. **export 路由真落进 `permissionRoutes` 模板**（casbin Enforce 钉三路径，不信「模板应该匹配」）→ Task 3 `TestRebuild_ExportRoutesCoveredByWildcard`（policy_sync_test.go，Enforce `/api/admin/{system/users|member/users|order/orders}/export`）。
2. **上限检查在写字节之前**（先写后查会发出半个 200）→ Task 2 `TestBuild_TooManyRows`（Build 返回 30013 且零字节契约）+ Task 3 `TestExportUsers_TooManyRows_Envelope400`（controller 侧：响应是 JSON 信封 400、体非 `PK` zip 头）。
3. **member 导出真走 tenant 插件**（别在裸 sqlite/memRepo 自证——B 终审 C2 教训）→ Task 4 `TestListAll_TenantIsolation_UnderPlugin`（真 `gormplugin.Register` + 双租户数据钉跨租户不泄漏）。
4. **`request.ts` Blob 错误特判**（export 400/30013 应弹中文文案而非乱码/通用错）→ Task 6 `blob-error.test.ts`（Blob 信封 → msg 提取；非 JSON → 空串回退）+ `request.ts` 接线改用该函数。
5. **export 路径无 token → 401**（既有 JWT 中间件对 export 子路径同样生效）→ Task 3 `TestAuthMiddleware_AdminExportWithoutToken_401`（auth_test.go，test router 注册 export 路径 + 401/信封 20001）。

## File Structure

| 层 | 文件 | 动作 |
|---|---|---|
| pkg/errors | `pkg/errors/code.go` | 改：+30013 常量 + registry |
| pkg/errors | `pkg/errors/code_test.go` | 改：导出常量清单补 30010-30013 |
| pkg/errors | `pkg/errors/excel_code_test.go` | 新 |
| pkg/excel | `pkg/excel/excel.go` | 新：Build/Spec/Column/MIME/MaxExportRows/FileName/Disposition/TimeRFC3339 |
| pkg/excel | `pkg/excel/excel_test.go` | 新 |
| pkg/middleware | `pkg/middleware/auth_test.go` | 改：+export 路由 + 401 case |
| system | `modules/system/controller/admin/user.go` | 改：+ExportUsers + 列映射 |
| system | `modules/system/controller/admin/user_export_test.go` | 新 |
| system | `modules/system/module.go` | 改：+`GET /users/export` |
| system | `modules/system/service/policy_sync_test.go` | 改：+export 通配覆盖 Enforce 测试 |
| member | `modules/member/repository/member_repo.go` | 改：+ListAll |
| member | `modules/member/repository/member_repo_test.go` | 新：真插件租户隔离 |
| member | `modules/member/service/member.go` | 改：接口 + 实现 +Export |
| member | `modules/member/service/member_test.go` | 改：memRepo +ListAll；+Export 租户测试 |
| member | `modules/member/controller/admin/member_user.go` | 改：+ExportMemberUsers + 列映射 |
| member | `modules/member/controller/admin/member_user_export_test.go` | 新 |
| member | `modules/member/module.go` | 改：+`GET /users/export` |
| order | `modules/order/controller/admin/controller.go` | 改：+ExportOrders + 列映射 |
| order | `modules/order/controller/admin/order_export_test.go` | 新 |
| order | `modules/order/module.go` | 改：+`GET /orders/export` |
| web | `apps/web-antd/src/api/blob-error.ts` | 新：Blob 错误体解析（无副作用模块） |
| web | `apps/web-antd/src/api/blob-error.test.ts` | 新 |
| web | `apps/web-antd/src/api/request.ts` | 改：Blob 特判改调 blobErrorDetail |
| web | `apps/web-antd/src/api/admin.ts` | 改：+exportUsers/exportOrders/saveExportBlob |
| web | `apps/web-antd/src/api/admin.test.ts` | 新：download 传输钉 |
| web | `apps/web-antd/src/views/system/SystemUsersPage.vue` | 改：+导出按钮 |
| web | `apps/web-antd/src/views/order/OrderOrdersPage.vue` | 改：+导出按钮 |
| docs | `docs/roadmap.md` | 改：P1-2 收口（Task 7，与代码提交分开） |

---

### Task 1: 错误码 30013

**Files:**
- Modify: `pkg/errors/code.go`（常量区 39 行后 + registry 66 行后）
- Modify: `pkg/errors/code_test.go`（导出常量清单 20-26 行）
- Test: `pkg/errors/excel_code_test.go`（新）

**Interfaces:**
- Consumes: 既有 `errs.Code` / `registry` / `New` / `Is`（零改动）
- Produces: `errs.CodeExportTooManyRows Code = 30013`；registry 文案「导出行数超过上限」——Task 2 `pkg/excel` 用 `errs.New(errs.CodeExportTooManyRows, nil)` / `errs.Is(err, errs.CodeExportTooManyRows)`

- [ ] **Step 1: 写失败测试**

创建 `pkg/errors/excel_code_test.go`：

```go
package errors

import "testing"

// 导出错误码文案回归：文案写死中文、码值固定（30013 → HTTP 400 由 resp.statusFor
// 的 3xxxx 分支自动映射，勿改 resp）。模式照 file_code_test.go。
func TestExportErrorCodes(t *testing.T) {
	be := New(CodeExportTooManyRows, nil)
	if got := be.Msg(); got != "导出行数超过上限" {
		t.Errorf("New(30013).Msg() = %q, want %q", got, "导出行数超过上限")
	}
	if !Is(be, CodeExportTooManyRows) {
		t.Error("Is(New(30013), CodeExportTooManyRows) must be true")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd /Users/alex/Desktop/goWork/alexGo-cloud && CGO_ENABLED=0 go test ./pkg/errors/ -run TestExportErrorCodes -v`
Expected: 编译失败 `undefined: CodeExportTooManyRows`

- [ ] **Step 3: 实现**

`pkg/errors/code.go` 常量区（`CodeFileTooLarge Code = 30012` 之后、`// 5xxxx 系统` 之前）加：

```go
	// CodeExportTooManyRows excel：导出行数超过 MaxExportRows 上限（5万保险丝）。
	CodeExportTooManyRows Code = 30013
```

registry（`CodeFileTooLarge: "文件超过大小限制",` 之后）加：

```go
	CodeExportTooManyRows: "导出行数超过上限",
```

`pkg/errors/code_test.go` 导出常量清单（顺手补 B 挂账：30010-30012 从未列入）改为完整版：

```go
	// 导出常量必须都在注册表里
	for _, c := range []Code{
		CodeParamInvalid, CodeParamMissing,
		CodeUnauthorized, CodeTokenExpired, CodeForbidden,
		CodeUsernameTaken, CodeUserNotFound, CodeRoleTaken, CodeTenantDisabled, CodeCaptchaInvalid,
		CodeNoticeNotFound, CodeCodegenTableNotFound, CodeCodegenTableTaken,
		CodeMobileTaken,
		CodeFileNotFound, CodeFileTypeNotAllowed, CodeFileTooLarge,
		CodeExportTooManyRows,
		CodeInternal,
	} {
```

- [ ] **Step 4: 跑测试确认通过**

Run: `CGO_ENABLED=0 go test ./pkg/errors/ -v`
Expected: PASS（含 TestExportErrorCodes、TestRegistryNoDuplicateAndAllHaveMsg、TestFileErrorCodes 全绿）

- [ ] **Step 5: 提交**

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud
git add pkg/errors/code.go pkg/errors/code_test.go pkg/errors/excel_code_test.go
git commit -m "feat(errors): 新增 30013 导出行数超限错误码"
```

---

### Task 2: pkg/excel 薄共享层

**Files:**
- Modify: `go.mod` / `go.sum`（`go get github.com/xuri/excelize/v2`）
- Create: `pkg/excel/excel.go`
- Test: `pkg/excel/excel_test.go`

**Interfaces:**
- Consumes: Task 1 的 `errs.CodeExportTooManyRows` / `errs.CodeParamInvalid` / `errs.New` / `errs.Is`
- Produces（Task 3/4/5 controller 依赖，签名逐字）:
  - `const MaxExportRows = 50000`
  - `const MIME = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"`
  - `type Column[T any] struct { Title string; Cell func(T) any }`
  - `type Spec[T any] struct { Sheet string; FileName string; Columns []Column[T]; Rows []T }`
  - `func Build[T any](spec Spec[T]) ([]byte, error)` —— 返回 error 时零字节产出（先查后写契约）
  - `func FileName(base string) string` —— `<base>_<yyyyMMdd_HHmmss>.xlsx`
  - `func Disposition(fileName string) string` —— RFC5987（`mime.FormatMediaType`）
  - `func TimeRFC3339(t *time.Time) string` —— nil → `""`

- [ ] **Step 1: 拉依赖**

Run: `cd /Users/alex/Desktop/goWork/alexGo-cloud && go get github.com/xuri/excelize/v2 && go mod tidy`
Expected: go.mod 出现 excelize/v2；无报错。

- [ ] **Step 2: 写失败测试**

创建 `pkg/excel/excel_test.go`：

```go
package excel

import (
	"bytes"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"

	errs "alexGo-cloud/pkg/errors"
)

type row struct {
	Name string
	Age  int
	When *time.Time
}

func threeCols() []Column[row] {
	return []Column[row]{
		{Title: "姓名", Cell: func(r row) any { return r.Name }},
		{Title: "年龄", Cell: func(r row) any { return r.Age }},
		{Title: "时间", Cell: func(r row) any { return TimeRFC3339(r.When) }},
	}
}

// cell 安全取格：excelize GetRows 会裁剪行尾空格，越界回退空串。
func cell(rows [][]string, r, c int) string {
	if r >= len(rows) || c >= len(rows[r]) {
		return ""
	}
	return rows[r][c]
}

// 往返：Build 出的 xlsx 用 excelize 读回，表头+数据行逐格比对（含 nil 时间→空串）。
func TestBuild_RoundTrip(t *testing.T) {
	when := time.Date(2026, 10, 19, 8, 30, 0, 0, time.UTC)
	data, err := Build(Spec[row]{
		Sheet:    "测试",
		FileName: "t.xlsx",
		Columns:  threeCols(),
		Rows: []row{
			{Name: "张三", Age: 30, When: &when},
			{Name: "李四", Age: 25, When: nil},
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	defer f.Close()
	rows, err := f.GetRows("测试")
	if err != nil {
		t.Fatalf("GetRows: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3 (header + 2)", len(rows))
	}
	for i, w := range []string{"姓名", "年龄", "时间"} {
		if cell(rows, 0, i) != w {
			t.Errorf("header[%d] = %q, want %q", i, cell(rows, 0, i), w)
		}
	}
	if cell(rows, 1, 0) != "张三" || cell(rows, 1, 1) != "30" || cell(rows, 1, 2) != "2026-10-19T08:30:00Z" {
		t.Errorf("row1 = %v", rows[1])
	}
	if cell(rows, 2, 0) != "李四" || cell(rows, 2, 2) != "" {
		t.Errorf("row2 (nil time) = %v", rows[2])
	}
}

// 空 Rows 合法：仅表头 xlsx。
func TestBuild_EmptyRows_HeaderOnly(t *testing.T) {
	data, err := Build(Spec[row]{Sheet: "测试", FileName: "t.xlsx", Columns: threeCols()})
	if err != nil {
		t.Fatalf("Build empty: %v", err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	defer f.Close()
	rows, err := f.GetRows("测试")
	if err != nil {
		t.Fatalf("GetRows: %v", err)
	}
	if len(rows) != 1 || cell(rows, 0, 0) != "姓名" {
		t.Errorf("empty rows sheet = %v, want header only", rows)
	}
}

// 超限：Build 在产出任何字节之前拒绝（RF②）——返回 30013 且零字节。
func TestBuild_TooManyRows(t *testing.T) {
	rows := make([]row, MaxExportRows+1)
	data, err := Build(Spec[row]{Sheet: "测试", FileName: "t.xlsx", Columns: threeCols(), Rows: rows})
	if !errs.Is(err, errs.CodeExportTooManyRows) {
		t.Fatalf("err = %v, want 30013", err)
	}
	if data != nil {
		t.Errorf("data must be nil on error, got %d bytes", len(data))
	}
}

// 空列定义拒绝（不 panic；Column[T] 泛型下「列数不一致」结构上不可表示，此为等价防护）。
func TestBuild_NoColumns(t *testing.T) {
	if _, err := Build(Spec[row]{Rows: []row{{Name: "x"}}}); !errs.Is(err, errs.CodeParamInvalid) {
		t.Fatalf("err = %v, want 10001", err)
	}
}

// FileName / Disposition / TimeRFC3339 纯函数形状。
func TestHelpers(t *testing.T) {
	name := FileName("users")
	if len(name) < len("users_") || name[:6] != "users_" || name[len(name)-5:] != ".xlsx" {
		t.Errorf("FileName = %q", name)
	}
	if got := Disposition("users_20261019_153045.xlsx"); got != "attachment; filename=users_20261019_153045.xlsx" {
		t.Errorf("Disposition = %q", got)
	}
	if TimeRFC3339(nil) != "" {
		t.Error("TimeRFC3339(nil) must be empty")
	}
	when := time.Date(2026, 10, 19, 8, 30, 0, 0, time.UTC)
	if got := TimeRFC3339(&when); got != "2026-10-19T08:30:00Z" {
		t.Errorf("TimeRFC3339 = %q", got)
	}
}
```

- [ ] **Step 3: 跑测试确认失败**

Run: `CGO_ENABLED=0 go test ./pkg/excel/ -v`
Expected: 编译失败 `undefined: Build`（包尚不存在时为 `no Go files` / package not found）

- [ ] **Step 4: 实现**

创建 `pkg/excel/excel.go`：

```go
// Package excel 导出 xlsx 的薄共享层：表头+数据行构建、行数上限、下载文件名与
// Content-Disposition。列映射由各域 controller 私有提供（Column 闭包），
// 本包不感知任何业务模型。
package excel

import (
	"errors"
	"mime"
	"time"

	"github.com/xuri/excelize/v2"

	errs "alexGo-cloud/pkg/errors"
)

// MaxExportRows 导出行数保险丝：租户内全量可能很大，超限直接 30013 拒绝。
const MaxExportRows = 50000

// MIME xlsx 的 Content-Type（下载流豁免信封的响应体类型）。
const MIME = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

// Column 一列：标题 + 从行取值的闭包（返回 excelize 可写的 string/int/int64/
// float64/bool；时间请走 TimeRFC3339 转字符串）。
type Column[T any] struct {
	Title string
	Cell  func(T) any
}

// Spec 一次导出的完整描述。
type Spec[T any] struct {
	Sheet    string      // sheet 名（如 "用户"）
	FileName string      // 下载文件名（用 FileName() 生成）
	Columns  []Column[T] // 列（表头 + 取值闭包）
	Rows     []T         // 数据行
}

// Build 先完整构建 xlsx 到内存再返回字节。
// 契约：返回非 nil error 时一个字节都没产出——调用方发信封是安全的
// （Review Focus ② 先查后写；信封豁免只发生在调用方 ctx.Data 写字节之后，
// 与 codegen download 同款）。excelize 行数据本就驻留内存，Build 非流式是
// 事实形态而非妥协（决策②「不经 P1-1 落盘」不受影响）。
func Build[T any](spec Spec[T]) ([]byte, error) {
	if len(spec.Rows) > MaxExportRows {
		return nil, errs.New(errs.CodeExportTooManyRows, nil)
	}
	if len(spec.Columns) == 0 {
		return nil, errs.New(errs.CodeParamInvalid, errors.New("excel: no columns"))
	}

	f := excelize.NewFile()
	defer f.Close()
	sheet := spec.Sheet
	if sheet == "" {
		sheet = "Sheet1"
	}
	// NewFile 默认建 Sheet1；改名到目标 sheet。
	f.SetSheetName("Sheet1", sheet)

	for i, col := range spec.Columns {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(sheet, cell, col.Title); err != nil {
			return nil, err
		}
	}
	for r, row := range spec.Rows {
		for c, col := range spec.Columns {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+2)
			if err := f.SetCellValue(sheet, cell, col.Cell(row)); err != nil {
				return nil, err
			}
		}
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// FileName 带时间戳的下载文件名：<base>_<yyyyMMdd_HHmmss>.xlsx。
func FileName(base string) string {
	return base + "_" + time.Now().Format("20060102_150405") + ".xlsx"
}

// Disposition RFC5987 Content-Disposition：mime.FormatMediaType 对 ASCII 名内联
// filename=、非 ASCII 自动升级 filename*=utf-8''…（file_controller serve 段先例）。
func Disposition(fileName string) string {
	return mime.FormatMediaType("attachment", map[string]string{"filename": fileName})
}

// TimeRFC3339 时间列取值：RFC3339；nil → 空串（LoginDate 等可空时间不 panic）。
func TimeRFC3339(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}
```

- [ ] **Step 5: 跑测试确认通过**

Run: `CGO_ENABLED=0 go test ./pkg/excel/ -v`
Expected: PASS（RoundTrip / EmptyRows / TooManyRows / NoColumns / Helpers 全绿）

- [ ] **Step 6: 提交**

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud
git add go.mod go.sum pkg/excel/excel.go pkg/excel/excel_test.go
git commit -m "feat(pkg/excel): 新增 xlsx 导出薄层（Build/上限/RFC5987）"
```

---

### Task 3: system 用户导出端点

**Files:**
- Modify: `modules/system/controller/admin/user.go`（imports + 文件尾部 ExportUsers）
- Create: `modules/system/controller/admin/user_export_test.go`
- Modify: `modules/system/module.go`（181 行 `GET /users` 之后）
- Modify: `modules/system/service/policy_sync_test.go`（文件尾部 +RF① 测试）
- Modify: `pkg/middleware/auth_test.go`（newRouter + export 路由；+401 case）

**Interfaces:**
- Consumes: Task 2 `excel.Spec/Column/Build/FileName/Disposition/TimeRFC3339/MIME/MaxExportRows`；既有 `service.UserService.ListUsers(ctx) ([]*model.User, error)`（零接口改动，租户内全量）；`model.User` 字段 ID/Username/Nickname/Email/Mobile/Sex/Status/DeptID/Remark/LoginIP/LoginDate(*time.Time)/CreatedAt
- Produces: `func (c *UserController) ExportUsers(ctx *gin.Context)`；路由 `GET /api/admin/system/users/export`——Task 6 前端 `exportUsers()` 指向此路径；RF① casbin 测试同时覆盖 Task 4/5 的 export 路径（permissionRoutes 已有键，无代码依赖）

- [ ] **Step 1: 写失败测试（controller）**

创建 `modules/system/controller/admin/user_export_test.go`：

```go
package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"

	"alexGo-cloud/modules/system/model"
	"alexGo-cloud/modules/system/service"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/excel"
)

// stubUserService：只关心 ListUsers 的桩（嵌入接口，其余方法不触达）。
type stubUserService struct {
	service.UserService
	users []*model.User
	err   error
}

func (s *stubUserService) ListUsers(context.Context) ([]*model.User, error) {
	return s.users, s.err
}

func doExport(t *testing.T, svc service.UserService) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	c := NewUserController(svc)
	r.GET("/api/admin/system/users/export", c.ExportUsers)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/admin/system/users/export", nil))
	return w
}

// 成功路径：200 + xlsx MIME + Disposition(users_) + 读回表头/首行关键字段。
func TestExportUsers_OK_XlsxRoundTrip(t *testing.T) {
	when := time.Date(2026, 10, 19, 8, 30, 0, 0, time.UTC)
	svc := &stubUserService{users: []*model.User{{
		ID: 1, Username: "alice", Nickname: "爱丽丝", Email: "a@x.com",
		Mobile: "13800000000", Sex: 2, Status: 1, DeptID: 7, Remark: "备注",
		LoginIP: "1.2.3.4", LoginDate: &when, CreatedAt: when,
	}}}
	w := doExport(t, svc)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != excel.MIME {
		t.Errorf("Content-Type = %q, want %q", ct, excel.MIME)
	}
	cd := w.Header().Get("Content-Disposition")
	if !strings.HasPrefix(cd, "attachment") || !strings.Contains(cd, "users_") {
		t.Errorf("Content-Disposition = %q", cd)
	}
	f, err := excelize.OpenReader(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	defer f.Close()
	rows, err := f.GetRows("用户")
	if err != nil {
		t.Fatalf("GetRows: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if rows[0][0] != "ID" || rows[0][1] != "用户名" || len(rows[0]) != 12 {
		t.Errorf("header = %v, want 12 cols starting ID/用户名", rows[0])
	}
	if rows[1][1] != "alice" || rows[1][2] != "爱丽丝" || rows[1][10] != "2026-10-19T08:30:00Z" {
		t.Errorf("row1 = %v", rows[1])
	}
}

// 超限：Build 在写字节前拒 → 信封 400/30013（RF② controller 侧半边；体非 PK zip 头）。
func TestExportUsers_TooManyRows_Envelope400(t *testing.T) {
	users := make([]*model.User, excel.MaxExportRows+1)
	for i := range users {
		users[i] = &model.User{ID: uint64(i + 1)}
	}
	w := doExport(t, &stubUserService{users: users})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%.80s", w.Code, w.Body.String())
	}
	var env struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("body must be JSON envelope: %v", err)
	}
	if env.Code != int(errs.CodeExportTooManyRows) || env.Msg != "导出行数超过上限" {
		t.Errorf("envelope = %d/%q, want 30013/导出行数超过上限", env.Code, env.Msg)
	}
	if bytes.HasPrefix(w.Body.Bytes(), []byte("PK")) {
		t.Error("30013 response must not start with xlsx zip magic")
	}
}

// 取行 DB 错 → 50000 信封（写流前，安全）。
func TestExportUsers_ListError_Envelope(t *testing.T) {
	w := doExport(t, &stubUserService{err: errs.New(errs.CodeInternal, nil)})
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `CGO_ENABLED=0 go test ./modules/system/controller/admin/ -run TestExportUsers -v`
Expected: 编译失败 `c.ExportUsers undefined`

- [ ] **Step 3: 实现 ExportUsers**

`modules/system/controller/admin/user.go` imports 区加：

```go
	"net/http"

	"alexGo-cloud/modules/system/model"
	"alexGo-cloud/pkg/excel"
```

文件尾部加（列映射按 spec §2：12 列，状态原始 int，时间 RFC3339，Avatar/PasswordHash 不导）：

```go
// ExportUsers 导出当前租户全量用户 xlsx（GET /users/export）。
// 先取行、再 Build、最后 ctx.Data——Build 返回 error 时一个字节未写，信封安全
// （RF② 先查后写；codegen download 同款豁免只发生在 ctx.Data 之后）。
func (c *UserController) ExportUsers(ctx *gin.Context) {
	users, err := c.userSvc.ListUsers(ctx.Request.Context())
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	spec := excel.Spec[*model.User]{
		Sheet:    "用户",
		FileName: excel.FileName("users"),
		Columns: []excel.Column[*model.User]{
			{Title: "ID", Cell: func(u *model.User) any { return u.ID }},
			{Title: "用户名", Cell: func(u *model.User) any { return u.Username }},
			{Title: "昵称", Cell: func(u *model.User) any { return u.Nickname }},
			{Title: "邮箱", Cell: func(u *model.User) any { return u.Email }},
			{Title: "手机", Cell: func(u *model.User) any { return u.Mobile }},
			{Title: "性别", Cell: func(u *model.User) any { return u.Sex }},
			{Title: "状态", Cell: func(u *model.User) any { return u.Status }},
			{Title: "部门ID", Cell: func(u *model.User) any { return u.DeptID }},
			{Title: "备注", Cell: func(u *model.User) any { return u.Remark }},
			{Title: "登录IP", Cell: func(u *model.User) any { return u.LoginIP }},
			{Title: "登录时间", Cell: func(u *model.User) any { return excel.TimeRFC3339(u.LoginDate) }},
			{Title: "创建时间", Cell: func(u *model.User) any { return excel.TimeRFC3339(&u.CreatedAt) }},
		},
		Rows: users,
	}
	data, err := excel.Build(spec)
	if err != nil {
		resp.Fail(ctx, err) // 30013 → 400 信封（statusFor 3xxxx 自动映射）
		return
	}
	ctx.Header("Content-Disposition", excel.Disposition(spec.FileName))
	ctx.Data(http.StatusOK, excel.MIME, data)
}
```

- [ ] **Step 4: 跑 controller 测试确认通过**

Run: `CGO_ENABLED=0 go test ./modules/system/controller/admin/ -run TestExportUsers -v`
Expected: PASS（3 个用例）

- [ ] **Step 5: 注册路由 + 跑编译**

`modules/system/module.go` `adminGroup.GET("/users", m.adminCtrl.ListUsers)` 之后加：

```go
	adminGroup.GET("/users/export", m.userCtrl.ExportUsers)
```

Run: `CGO_ENABLED=0 go build ./modules/system/...`
Expected: 编译通过（GET 静态段 /users/export 与 PUT /users/:id 不同方法树，无 gin 冲突）

- [ ] **Step 6: 写 RF① casbin 测试（钉 export 路由落进 permissionRoutes 模板）**

`modules/system/service/policy_sync_test.go` 文件尾部加：

```go
// RF①（P1-2）：export 子路由必须被既有 permissionRoutes 通配覆盖——三域导出零
// 新权限码，靠 /users/*、/orders/* 模板（keyMatch2）。不靠「模板应该匹配」的信念，
// 直接 Enforce 三条 export 路径 + 对照拒绝未授权路径。
func TestRebuild_ExportRoutesCoveredByWildcard(t *testing.T) {
	e := newMemEnforcer(t)
	role := &model.Role{ID: 1, Code: "admin", TenantID: 1}
	menus := []*model.Menu{
		{ID: 1, Permission: "system:user:list", Type: "menu"},
		{ID: 2, Permission: "member:user:manage", Type: "button"},
		{ID: 3, Permission: "order:order:*", Type: "menu"},
	}
	if err := rebuildRolePolicies(context.Background(), e, role, menus); err != nil {
		t.Fatalf("rebuild error = %v", err)
	}
	if _, err := e.AddRoleForUser("1:1:admin", "1:role:admin"); err != nil {
		t.Fatal(err)
	}
	for _, obj := range []string{
		"/api/admin/system/users/export",
		"/api/admin/member/users/export",
		"/api/admin/order/orders/export",
	} {
		if ok, _ := e.Enforce("1:1:admin", obj, "GET"); !ok {
			t.Errorf("export route %s must be covered by wildcard template", obj)
		}
	}
	if ok, _ := e.Enforce("1:1:admin", "/api/admin/infra/files", "GET"); ok {
		t.Error("unassigned route must be denied")
	}
}
```

Run: `CGO_ENABLED=0 go test ./modules/system/service/ -run TestRebuild_ExportRoutesCoveredByWildcard -v`
Expected: PASS（三条 export 路径 Enforce=true，infra 路径=false）

- [ ] **Step 7: 写 401 测试（export 路径无 token）**

`pkg/middleware/auth_test.go` newRouter 内 `r.GET("/api/admin/system/users", ...)` 之后加注册：

```go
	r.GET("/api/admin/system/users/export", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
```

文件尾部加：

```go
// P1-2 RF⑤：export 子路径同样被 JWT 中间件拦截（middleware 按 /api/admin/** 前缀，
// 不依赖具体路由注册；此处注册路由以证「有路由无 token 仍 401」）。
func TestAuthMiddleware_AdminExportWithoutToken_401(t *testing.T) {
	w := do(newRouter(testCfg(), nil), "GET", "/api/admin/system/users/export", "")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
	if code, msg, _ := envelopeOf(t, w); code != 20001 || msg != "未登录或登录已失效" {
		t.Errorf("code/msg = %d/%q, want 20001/未登录或登录已失效", code, msg)
	}
}
```

Run: `CGO_ENABLED=0 go test ./pkg/middleware/ -run TestAuthMiddleware_AdminExport -v`
Expected: PASS

- [ ] **Step 8: 提交**

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud
git add modules/system/controller/admin/user.go modules/system/controller/admin/user_export_test.go \
  modules/system/module.go modules/system/service/policy_sync_test.go \
  pkg/middleware/auth_test.go
git commit -m "feat(system): 用户列表导出 xlsx 端点"
```

---

### Task 4: member 会员导出端点（含真插件租户测试）

**Files:**
- Modify: `modules/member/repository/member_repo.go`（接口 + 实现加 ListAll）
- Create: `modules/member/repository/member_repo_test.go`
- Modify: `modules/member/service/member.go`（接口 + 实现加 Export）
- Modify: `modules/member/service/member_test.go`（memRepo 加 ListAll；+Export 测试）
- Modify: `modules/member/controller/admin/member_user.go`（+ExportMemberUsers）
- Create: `modules/member/controller/admin/member_user_export_test.go`
- Modify: `modules/member/module.go`（66 行 `GET /users` 之后）

**Interfaces:**
- Consumes: Task 2 `pkg/excel`；既有 `tenant.TenantIDFromContext(ctx)`；`model.MemberUser`（ID/Nickname/Mobile/Status/RegisterIP/LoginIP/LoginDate(*time.Time)/CreatedAt/Deleted(bool)/TenantID）
- Produces:
  - `MemberRepository.ListAll(ctx context.Context, tenantID uint64) ([]*model.MemberUser, error)`——Where `tenant_id = ? AND deleted = 0` + `Order("id desc")`，**不走 repo.Paginate**（Paginate page/size≤0 会兜底 1/10）
  - `MemberService.Export(ctx context.Context) ([]*model.MemberUser, error)`——`s.repo.ListAll(ctx, tenant.TenantIDFromContext(ctx))`
  - `func (c *UserController) ExportMemberUsers(ctx *gin.Context)`；路由 `GET /api/admin/member/users/export`（curl 可验收；member 无前端页面）

- [ ] **Step 1: 写失败测试（repo 真插件租户隔离——RF③）**

创建 `modules/member/repository/member_repo_test.go`：

```go
package repository

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/modules/member/model"
	"alexGo-cloud/pkg/tenant"
	"alexGo-cloud/pkg/tenant/gormplugin"
)

// ListAll 租户隔离：真注册 gormplugin（生产装配，main.go 同款 Options）——裸
// sqlite/memRepo 的 Where 自测钉不住插件层（B 子项目终审 C2 教训，RF③）。
func TestListAll_TenantIsolation_UnderPlugin(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.MemberUser{}); err != nil {
		t.Fatal(err)
	}
	if err := gormplugin.Register(db, gormplugin.Options{
		ExemptTables: []string{"tenants", "casbin_rule"},
	}); err != nil {
		t.Fatal(err)
	}
	repo := NewMemberRepository(db)

	ctx1 := tenant.WithTenantID(context.Background(), 1)
	ctx2 := tenant.WithTenantID(context.Background(), 2)
	for _, u := range []*model.MemberUser{
		{Nickname: "甲", Mobile: "138", TenantID: 1},
		{Nickname: "乙", Mobile: "139", TenantID: 1},
	} {
		if err := repo.Create(ctx1, u); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}
	if err := repo.Create(ctx2, &model.MemberUser{Nickname: "丙", Mobile: "1380", TenantID: 2}); err != nil {
		t.Fatalf("Create tenant2: %v", err)
	}

	got, err := repo.ListAll(ctx1, 1)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListAll count = %d, want 2 (tenant 1 only)", len(got))
	}
	for _, u := range got {
		if u.TenantID != 1 {
			t.Errorf("cross-tenant leak: id=%d tenant=%d", u.ID, u.TenantID)
		}
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `CGO_ENABLED=0 go test ./modules/member/repository/ -v`
Expected: 编译失败 `undefined: repo.ListAll`（接口无此方法）

- [ ] **Step 3: 实现 repo.ListAll**

`modules/member/repository/member_repo.go` 接口内 `UpdateStatus` 注释前加：

```go
	// ListAll 租户内全量（导出用；不截断、不走 Paginate——后者 page/size≤0 兜底 1/10）。
	ListAll(ctx context.Context, tenantID uint64) ([]*model.MemberUser, error)
```

文件尾部（或 List 之后）加实现：

```go
// ListAll 租户内全量导出行：与 List 同 Where 口径（tenant_id + deleted=0），
// id desc 保序；无 Offset/Limit——导出要的是全量，不是分页。
func (r *memberRepo) ListAll(ctx context.Context, tid uint64) ([]*model.MemberUser, error) {
	var list []*model.MemberUser
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND deleted = 0", tid).
		Order("id desc").
		Find(&list).Error
	return list, err
}
```

- [ ] **Step 4: 跑 repo 测试确认通过**

Run: `CGO_ENABLED=0 go test ./modules/member/repository/ -v`
Expected: PASS（真插件下 ListAll 只回租户 1 的 2 行）

- [ ] **Step 5: memRepo 同步加 ListAll（接口变更编译修复）+ 写 service 失败测试**

`modules/member/service/member_test.go` memRepo 的 `List` 方法之后加（`sort` 已在 imports）：

```go
func (m *memRepo) ListAll(_ context.Context, tid uint64) ([]*model.MemberUser, error) {
	var list []*model.MemberUser
	for _, u := range m.byMobile {
		if u.TenantID == tid && !u.Deleted {
			list = append(list, u)
		}
	}
	// 与真实仓储同形：id desc 排序。
	sort.Slice(list, func(i, j int) bool { return list[i].ID > list[j].ID })
	return list, nil
}
```

同文件尾部加 service 测试：

```go
// Export 透传 ctx 租户到 ListAll（插件层钉在 repo 测试；此处钉 service 取数路径）。
func TestExport_FiltersByTenant(t *testing.T) {
	repo := newMemRepo()
	for _, u := range []*model.MemberUser{
		{Mobile: "138", Nickname: "甲", TenantID: 1},
		{Mobile: "139", Nickname: "乙", TenantID: 1},
		{Mobile: "1380", Nickname: "丙", TenantID: 2},
	} {
		if err := repo.Create(context.Background(), u); err != nil {
			t.Fatal(err)
		}
	}
	svc := NewMemberService(repo, nil, nil)
	got, err := svc.Export(tenant.WithTenantID(context.Background(), 1))
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("Export count = %d, want 2", len(got))
	}
	for _, u := range got {
		if u.TenantID != 1 {
			t.Errorf("cross-tenant row: id=%d tenant=%d", u.ID, u.TenantID)
		}
	}
}
```

- [ ] **Step 6: 跑测试确认失败（svc.Export 未定义）**

Run: `CGO_ENABLED=0 go test ./modules/member/service/ -run TestExport_FiltersByTenant -v`
Expected: 编译失败 `svc.Export undefined`（若先因 memRepo 缺 ListAll 编译失败，先完成 Step 5 的 memRepo 补丁）

- [ ] **Step 7: 实现 service.Export**

`modules/member/service/member.go` 接口 `Disable` 之后加：

```go
	// Export 租户内全量会员（导出用；与 List 同查询口径、不截断不分页）。
	Export(ctx context.Context) ([]*model.MemberUser, error)
```

实现（`Disable` 实现之后）加：

```go
// Export 租户内全量会员：租户 ID 取自 ctx（tenant 插件经 ctx 照常隔离），经
// repo.ListAll 不截断返回——controller 组 excel 列映射。
func (s *memberService) Export(ctx context.Context) ([]*model.MemberUser, error) {
	return s.repo.ListAll(ctx, tenant.TenantIDFromContext(ctx))
}
```

Run: `CGO_ENABLED=0 go test ./modules/member/service/ -v`
Expected: PASS（TestExport_FiltersByTenant + 既有用例全绿；既有 stubSvc/stubListSvc 嵌入接口不受影响）

- [ ] **Step 8: 写 controller 失败测试**

创建 `modules/member/controller/admin/member_user_export_test.go`：

```go
package admin

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"

	"alexGo-cloud/modules/member/model"
	"alexGo-cloud/modules/member/service"
	"alexGo-cloud/pkg/excel"
)

// stubExportSvc：只关心 Export 的桩（嵌入接口，照 stubListSvc 模式）。
type stubExportSvc struct {
	service.MemberService
	members []*model.MemberUser
	err     error
}

func (s *stubExportSvc) Export(context.Context) ([]*model.MemberUser, error) {
	return s.members, s.err
}

func TestExportMemberUsers_XlsxRoundTrip(t *testing.T) {
	when := time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC)
	svc := &stubExportSvc{members: []*model.MemberUser{{
		ID: 1, Nickname: "会员甲", Mobile: "13811112222", Status: 1,
		RegisterIP: "5.6.7.8", LoginIP: "1.2.3.4", LoginDate: &when, CreatedAt: when,
	}}}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	c := NewUserController(svc)
	r.GET("/api/admin/member/users/export", c.ExportMemberUsers)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/admin/member/users/export", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != excel.MIME {
		t.Errorf("Content-Type = %q, want %q", ct, excel.MIME)
	}
	if cd := w.Header().Get("Content-Disposition"); !strings.HasPrefix(cd, "attachment") || !strings.Contains(cd, "member_users_") {
		t.Errorf("Content-Disposition = %q", cd)
	}
	f, err := excelize.OpenReader(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	defer f.Close()
	rows, err := f.GetRows("会员")
	if err != nil {
		t.Fatalf("GetRows: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if rows[0][0] != "ID" || rows[0][7] != "创建时间" || len(rows[0]) != 8 {
		t.Errorf("header = %v, want 8 cols ID..创建时间", rows[0])
	}
	if rows[1][1] != "会员甲" || rows[1][2] != "13811112222" {
		t.Errorf("row1 = %v", rows[1])
	}
}
```

- [ ] **Step 9: 跑测试确认失败**

Run: `CGO_ENABLED=0 go test ./modules/member/controller/admin/ -run TestExportMemberUsers -v`
Expected: 编译失败 `c.ExportMemberUsers undefined`

- [ ] **Step 10: 实现 ExportMemberUsers + 注册路由**

`modules/member/controller/admin/member_user.go` imports 加：

```go
	"net/http"

	"alexGo-cloud/pkg/excel"
```

文件尾部加（列映射按 spec §2：8 列）：

```go
// ExportMemberUsers 导出当前租户全量会员 xlsx（GET /users/export）。
// 租户隔离双层：svc.Export 取 ctx 租户 + repo.ListAll Where + gormplugin 经 ctx
// 照常作用；Build 失败字节未写，信封安全（RF②）。
func (c *UserController) ExportMemberUsers(ctx *gin.Context) {
	members, err := c.svc.Export(ctx.Request.Context())
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	spec := excel.Spec[*model.MemberUser]{
		Sheet:    "会员",
		FileName: excel.FileName("member_users"),
		Columns: []excel.Column[*model.MemberUser]{
			{Title: "ID", Cell: func(u *model.MemberUser) any { return u.ID }},
			{Title: "昵称", Cell: func(u *model.MemberUser) any { return u.Nickname }},
			{Title: "手机", Cell: func(u *model.MemberUser) any { return u.Mobile }},
			{Title: "状态", Cell: func(u *model.MemberUser) any { return u.Status }},
			{Title: "注册IP", Cell: func(u *model.MemberUser) any { return u.RegisterIP }},
			{Title: "登录IP", Cell: func(u *model.MemberUser) any { return u.LoginIP }},
			{Title: "登录时间", Cell: func(u *model.MemberUser) any { return excel.TimeRFC3339(u.LoginDate) }},
			{Title: "创建时间", Cell: func(u *model.MemberUser) any { return excel.TimeRFC3339(&u.CreatedAt) }},
		},
		Rows: members,
	}
	data, err := excel.Build(spec)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	ctx.Header("Content-Disposition", excel.Disposition(spec.FileName))
	ctx.Data(http.StatusOK, excel.MIME, data)
}
```

`modules/member/module.go` `adminGroup.GET("/users", m.userCtrl.List)` 之后加：

```go
	adminGroup.GET("/users/export", m.userCtrl.ExportMemberUsers)
```

Run: `CGO_ENABLED=0 go test ./modules/member/... -v`
Expected: PASS（export + 既有全部用例；PUT /users/:id/status 与 GET /users/export 不同方法树，无 gin 冲突）

- [ ] **Step 11: 提交**

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud
git add modules/member/repository/member_repo.go modules/member/repository/member_repo_test.go \
  modules/member/service/member.go modules/member/service/member_test.go \
  modules/member/controller/admin/member_user.go modules/member/controller/admin/member_user_export_test.go \
  modules/member/module.go
git commit -m "feat(member): 会员全量导出端点（ListAll + 真插件租户测试）"
```

---

### Task 5: order 订单导出端点

**Files:**
- Modify: `modules/order/controller/admin/controller.go`（imports + ExportOrders）
- Create: `modules/order/controller/admin/order_export_test.go`
- Modify: `modules/order/module.go`（61 行 `POST /orders` 之后）

**Interfaces:**
- Consumes: Task 2 `pkg/excel`；既有 `service.OrderService.List(ctx) ([]*model.Order, error)`（已全量、零接口改动，租户插件经 ctx 生效）；`model.Order`（ID/OrderNo/UserID/ProductID/Amount(int64 分)/Status/CreatedAt/UpdatedAt）
- Produces: `func (c *AdminOrderController) ExportOrders(ctx *gin.Context)`；路由 `GET /api/admin/order/orders/export`——Task 6 前端 `exportOrders()` 指向此路径

- [ ] **Step 1: 写失败测试**

创建 `modules/order/controller/admin/order_export_test.go`：

```go
package admin

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"

	"alexGo-cloud/modules/order/model"
	"alexGo-cloud/modules/order/service"
	"alexGo-cloud/pkg/excel"
)

// stubOrderSvc：只关心 List 的桩（嵌入接口，Create 不触达）。
type stubOrderSvc struct {
	service.OrderService
	orders []*model.Order
	err    error
}

func (s *stubOrderSvc) List(context.Context) ([]*model.Order, error) {
	return s.orders, s.err
}

func TestExportOrders_XlsxRoundTrip(t *testing.T) {
	when := time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC)
	svc := &stubOrderSvc{orders: []*model.Order{{
		ID: 9, OrderNo: "ORD-001", UserID: 1, ProductID: 2,
		Amount: 19900, Status: 1, CreatedAt: when, UpdatedAt: when,
	}}}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	c := NewAdminOrderController(svc)
	r.GET("/api/admin/order/orders/export", c.ExportOrders)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/admin/order/orders/export", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != excel.MIME {
		t.Errorf("Content-Type = %q, want %q", ct, excel.MIME)
	}
	if cd := w.Header().Get("Content-Disposition"); !strings.HasPrefix(cd, "attachment") || !strings.Contains(cd, "orders_") {
		t.Errorf("Content-Disposition = %q", cd)
	}
	f, err := excelize.OpenReader(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	defer f.Close()
	rows, err := f.GetRows("订单")
	if err != nil {
		t.Fatalf("GetRows: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if rows[0][0] != "ID" || rows[0][1] != "订单号" || rows[0][4] != "金额(分)" || len(rows[0]) != 8 {
		t.Errorf("header = %v, want 8 cols ID..更新时间", rows[0])
	}
	// 金额导原始 int64 分：19900 → "19900"。
	if rows[1][1] != "ORD-001" || rows[1][4] != "19900" {
		t.Errorf("row1 = %v", rows[1])
	}
}

// 空表：仅表头 200 合法（spec §3）。
func TestExportOrders_Empty_HeaderOnly200(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	c := NewAdminOrderController(&stubOrderSvc{})
	r.GET("/api/admin/order/orders/export", c.ExportOrders)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/admin/order/orders/export", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	f, err := excelize.OpenReader(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	defer f.Close()
	rows, _ := f.GetRows("订单")
	if len(rows) != 1 {
		t.Errorf("rows = %d, want header only", len(rows))
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `CGO_ENABLED=0 go test ./modules/order/controller/admin/ -run TestExportOrders -v`
Expected: 编译失败 `c.ExportOrders undefined`

- [ ] **Step 3: 实现 ExportOrders**

`modules/order/controller/admin/controller.go` imports 加：

```go
	"net/http"

	"alexGo-cloud/pkg/excel"
```

文件尾部加（列映射按 spec §2：8 列；金额原始分）：

```go
// ExportOrders 导出当前租户全量订单 xlsx（GET /orders/export）。
// 复用既有全量 List（租户插件经 ctx 生效）；Build 失败字节未写，信封安全（RF②）。
func (c *AdminOrderController) ExportOrders(ctx *gin.Context) {
	orders, err := c.orderSvc.List(ctx.Request.Context())
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	spec := excel.Spec[*model.Order]{
		Sheet:    "订单",
		FileName: excel.FileName("orders"),
		Columns: []excel.Column[*model.Order]{
			{Title: "ID", Cell: func(o *model.Order) any { return o.ID }},
			{Title: "订单号", Cell: func(o *model.Order) any { return o.OrderNo }},
			{Title: "用户ID", Cell: func(o *model.Order) any { return o.UserID }},
			{Title: "产品ID", Cell: func(o *model.Order) any { return o.ProductID }},
			{Title: "金额(分)", Cell: func(o *model.Order) any { return o.Amount }},
			{Title: "状态", Cell: func(o *model.Order) any { return o.Status }},
			{Title: "创建时间", Cell: func(o *model.Order) any { return excel.TimeRFC3339(&o.CreatedAt) }},
			{Title: "更新时间", Cell: func(o *model.Order) any { return excel.TimeRFC3339(&o.UpdatedAt) }},
		},
		Rows: orders,
	}
	data, err := excel.Build(spec)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	ctx.Header("Content-Disposition", excel.Disposition(spec.FileName))
	ctx.Data(http.StatusOK, excel.MIME, data)
}
```

- [ ] **Step 4: 注册路由 + 跑测试**

`modules/order/module.go` `adminGroup.POST("/orders", m.ctrl.CreateOrder)` 之后加：

```go
	adminGroup.GET("/orders/export", m.ctrl.ExportOrders)
```

Run: `CGO_ENABLED=0 go test ./modules/order/... -v`
Expected: PASS（export 两用例 + 既有全绿；GET 静态 /orders/export 与 GET /orders 无冲突）

- [ ] **Step 5: 提交**

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud
git add modules/order/controller/admin/controller.go modules/order/controller/admin/order_export_test.go \
  modules/order/module.go
git commit -m "feat(order): 订单列表导出 xlsx 端点"
```

---

### Task 6: 前端导出按钮 + download 传输钉 + Blob 错误特判抽函数

**Files:**
- Create: `apps/web-antd/src/api/blob-error.ts`
- Create: `apps/web-antd/src/api/blob-error.test.ts`
- Modify: `apps/web-antd/src/api/request.ts`（114-130 行 Blob 特判块改调 blobErrorDetail）
- Modify: `apps/web-antd/src/api/admin.ts`（+exportUsers/exportOrders/saveExportBlob）
- Create: `apps/web-antd/src/api/admin.test.ts`
- Modify: `apps/web-antd/src/views/system/SystemUsersPage.vue`（import + onExport + 模板按钮）
- Modify: `apps/web-antd/src/views/order/OrderOrdersPage.vue`（同款）

**Interfaces:**
- Consumes: Task 3 路径 `GET /admin/system/users/export`、Task 5 路径 `GET /admin/order/orders/export`（requestClient baseURL=/api 自动补前缀）；既有 `requestClient.download<Blob>(path, config)`（@vben FileDownloader：默认 GET + responseType blob + responseReturn body 返回 Blob）；既有 `saveBlob`（infra.ts，downloadFileFromBlob 封装）
- Produces:
  - `blobErrorDetail(blob: Blob): Promise<string>`——Blob 错误体 → 信封 msg，非 JSON → `""`
  - `exportUsers(): Promise<Blob>` / `exportOrders(): Promise<Blob>`——`requestClient.download<Blob>(path, { timeout: 60_000 })`
  - `saveExportBlob(blob: Blob, base: string): void`——落盘 `<base>_<yyyyMMdd_HHmmss>.xlsx`
  - 页面 `onExport` 处理器 + `#toolbar-tools` 导出按钮（system 页 `system:user:list`、订单页 `order:order:*`，与页面既有按钮同权限码）

- [ ] **Step 1: 写 blob-error 失败测试**

创建 `apps/web-antd/src/api/blob-error.test.ts`：

```ts
import { describe, expect, it } from 'vitest';

import { blobErrorDetail } from './blob-error';

// RF④：download 的 400 错误体是 Blob——必须能解出信封 msg（export 30013 弹
// 「导出行数超过上限」而非乱码/通用错）。
describe('blobErrorDetail', () => {
  it('信封 JSON blob → 提取 msg', async () => {
    const blob = new Blob([
      JSON.stringify({ code: 30013, msg: '导出行数超过上限', data: null }),
    ]);
    await expect(blobErrorDetail(blob)).resolves.toBe('导出行数超过上限');
  });

  it('缺 msg → 回退 message 字段', async () => {
    const blob = new Blob([JSON.stringify({ code: 50000, message: '系统繁忙' })]);
    await expect(blobErrorDetail(blob)).resolves.toBe('系统繁忙');
  });

  it('非 JSON blob → 空串（调用方回退通用文案）', async () => {
    await expect(blobErrorDetail(new Blob(['not json']))).resolves.toBe('');
  });

  it('空对象 → 空串', async () => {
    await expect(blobErrorDetail(new Blob(['{}']))).resolves.toBe('');
  });
});
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd /Users/alex/Desktop/goWork/alexGo-cloud/apps/web-antd && pnpm vitest run src/api/blob-error.test.ts`
Expected: FAIL——模块 `./blob-error` 不存在

- [ ] **Step 3: 实现 blob-error.ts + 接线 request.ts**

创建 `apps/web-antd/src/api/blob-error.ts`（无副作用模块，可被 vitest 直接 import——request.ts 顶层建 client，测试 import 它会拖入整个客户端栈）：

```ts
// download 的 400 错误体是 Blob——转文本解析信封 msg，否则只显 axios 通用
// 文案（P1-2 RF④：export 30013 要弹「导出行数超过上限」而非乱码/通用错）。
export async function blobErrorDetail(blob: Blob): Promise<string> {
  try {
    const body = JSON.parse(await blob.text()) as {
      error?: string;
      message?: string;
      msg?: string;
    };
    return body?.msg ?? body?.message ?? body?.error ?? '';
  } catch {
    return '';
  }
}
```

`apps/web-antd/src/api/request.ts` 顶部 import 区加：

```ts
import { blobErrorDetail } from './blob-error';
```

将 114-130 行的 Blob 特判块（`if (responseData instanceof Blob) { void responseData.text()...}`）替换为：

```ts
      if (responseData instanceof Blob) {
        void blobErrorDetail(responseData)
          .then((detail: string) => {
            message.error(detail || msg);
          })
          .catch(() => message.error(msg));
        return;
      }
```

保留块上方的既有注释（download responseType='blob' … M3 终审跟进票③ … fire-and-forget）。

- [ ] **Step 4: 跑测试确认通过**

Run: `pnpm vitest run src/api/blob-error.test.ts`
Expected: PASS（4 用例）

- [ ] **Step 5: 写 admin.ts 传输失败测试**

创建 `apps/web-antd/src/api/admin.test.ts`（模式照 infra.test.ts：vi.mock('./request')）：

```ts
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { exportOrders, exportUsers } from './admin';
import { requestClient } from './request';

vi.mock('./request', () => ({
  requestClient: {
    delete: vi.fn(),
    download: vi.fn().mockResolvedValue(new Blob()),
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
  },
}));

// P1-2 传输钉：export 走 requestClient.download（默认 GET + responseType
// blob + responseReturn body → Blob），超时放宽 60s（同 codegen 下载）。
describe('export transport contract', () => {
  beforeEach(() => {
    vi.mocked(requestClient.download).mockClear();
  });

  it('exportUsers 走 download，路径与超时正确', async () => {
    await exportUsers();
    expect(requestClient.download).toHaveBeenCalledWith(
      '/admin/system/users/export',
      expect.objectContaining({ timeout: 60_000 }),
    );
  });

  it('exportOrders 走 download，路径与超时正确', async () => {
    await exportOrders();
    expect(requestClient.download).toHaveBeenCalledWith(
      '/admin/order/orders/export',
      expect.objectContaining({ timeout: 60_000 }),
    );
  });
});
```

- [ ] **Step 6: 跑测试确认失败**

Run: `pnpm vitest run src/api/admin.test.ts`
Expected: FAIL——`exportUsers` 未导出

- [ ] **Step 7: 实现 admin.ts 导出函数**

`apps/web-antd/src/api/admin.ts` import 区加：

```ts
import { saveBlob } from '#/api/infra';
```

文件尾部加：

```ts
// ---- Excel 导出（P1-2）----
// download 默认 GET + responseType blob（@vben FileDownloader）；60s 放宽同 codegen。
export function exportUsers(): Promise<Blob> {
  return requestClient.download<Blob>('/admin/system/users/export', {
    timeout: 60_000,
  });
}

export function exportOrders(): Promise<Blob> {
  return requestClient.download<Blob>('/admin/order/orders/export', {
    timeout: 60_000,
  });
}

// saveExportBlob 统一落盘文件名 <base>_<yyyyMMdd_HHmmss>.xlsx（与服务端
// Content-Disposition 同构；浏览器实际保存名以客户端为准）。
export function saveExportBlob(blob: Blob, base: string) {
  const d = new Date();
  const p = (n: number) => String(n).padStart(2, '0');
  const stamp = `${d.getFullYear()}${p(d.getMonth() + 1)}${p(d.getDate())}_${p(d.getHours())}${p(d.getMinutes())}${p(d.getSeconds())}`;
  saveBlob(blob, `${base}_${stamp}.xlsx`);
}
```

- [ ] **Step 8: 跑测试确认通过**

Run: `pnpm vitest run src/api/admin.test.ts`
Expected: PASS（2 用例）

- [ ] **Step 9: 两页面加导出按钮**

`apps/web-antd/src/views/system/SystemUsersPage.vue`：

script import 行改为：

```ts
import {
  createUser,
  exportUsers,
  listUsers,
  resetUserPassword,
  saveExportBlob,
} from '#/api/admin';
```

`onResetPassword` 函数之后加：

```ts
async function onExport() {
  try {
    const blob = await exportUsers();
    saveExportBlob(blob, 'users');
  } catch {
    // 拦截器已 toast（含 download 400 的 Blob 特判）
  }
}
```

模板 `#toolbar-tools` 内「新建用户」Button 之后加：

```vue
        <Button
          class="mr-2"
          v-access:code="['system:user:list']"
          @click="onExport"
        >
          导出
        </Button>
```

`apps/web-antd/src/views/order/OrderOrdersPage.vue`：

script import 行改为：

```ts
import { createOrder, exportOrders, listOrders, saveExportBlob } from '#/api/admin';
```

`defineOptions` 之后或 formApi 定义前加：

```ts
async function onExport() {
  try {
    const blob = await exportOrders();
    saveExportBlob(blob, 'orders');
  } catch {
    // 拦截器已 toast（含 download 400 的 Blob 特判）
  }
}
```

模板 `#toolbar-tools` 内「新建订单」Button 之后加：

```vue
        <Button
          class="mr-2"
          v-access:code="['order:order:*']"
          @click="onExport"
        >
          导出
        </Button>
```

- [ ] **Step 10: typecheck + 全量前端测试**

Run: `pnpm typecheck && pnpm test`
Expected: 两命令全绿（vitest 含 blob-error 4 用例 + admin 2 用例 + 既有全部）

- [ ] **Step 11: 提交**

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud
git add apps/web-antd/src/api/blob-error.ts apps/web-antd/src/api/blob-error.test.ts \
  apps/web-antd/src/api/request.ts apps/web-antd/src/api/admin.ts apps/web-antd/src/api/admin.test.ts \
  apps/web-antd/src/views/system/SystemUsersPage.vue apps/web-antd/src/views/order/OrderOrdersPage.vue
git commit -m "feat(web): 导出按钮两颗 + download 传输钉 + Blob 错误特判抽函数"
```

---

### Task 7: 全量门禁 + roadmap P1-2 收口

**Files:**
- Modify: `docs/roadmap.md`（35 行 P1-2 行）

**Interfaces:**
- Consumes: Task 1-6 全部产出
- Produces: P1-2 收口记录（docs 提交与代码提交分开——本任务唯一提交是 docs）

- [ ] **Step 1: Go 全量门禁**

Run:

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud
CGO_ENABLED=0 go build ./modules/... ./pkg/...
CGO_ENABLED=0 go vet ./modules/... ./pkg/...
CGO_ENABLED=0 go test ./modules/... ./pkg/...
```

Expected: 三条全绿。若有失败：修复后重跑；修复代码随修复点归入对应任务风格的独立提交（`fix(...)`），本任务不吞并代码修复。

- [ ] **Step 2: 前端全量门禁**

Run: `cd apps/web-antd && pnpm typecheck && pnpm test`
Expected: 全绿。

- [ ] **Step 3: 冒烟自查（可选，失败不阻塞——浏览器项留票）**

Run: `CGO_ENABLED=0 go build ./alexgo-server/cmd && ./alexgo-server/cmd/cmd`（或项目既有启动方式）起服务后：

```bash
# 未带 token → 401 信封 20001
curl -s -o /dev/null -w '%{http_code}\n' http://localhost:8080/api/admin/system/users/export
# 带 token → 下载 xlsx（登录取 token 后）；file 命令或 python -c 读回验证 zip 头 PK
```

Expected: 401 / 200+PK。失败记录进收口汇报，不回滚（Task 3-6 测试已钉行为）。

- [ ] **Step 4: roadmap 收口**

`docs/roadmap.md` 35 行 P1-2 行的依赖列改为 `✅`（照 P1-1 收口样式），内容列追加导出已落地：

```markdown
| P1-2 | **Excel 导入导出** | 用户/会员/订单列表导出（export 落地：`pkg/excel` + 三域 `/export` 端点 + 前端两按钮；导入留小票） | ✅ |
```

- [ ] **Step 5: docs 提交（与代码提交分开）**

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud
git add docs/roadmap.md
git commit -m "docs(roadmap): P1-2 Excel 导出收口"
```

- [ ] **Step 6: 收口汇报清单（写给最终汇报，不产码）**

- 三域 curl 验收路径：`GET /api/admin/{system/users|member/users|order/orders}/export`
- 留票项：前端两按钮浏览器手验（登录 → 用户页/订单页点导出 → Excel 打开）、member 导出 curl 手验、Excel 导入小票、member 前端页面小票
