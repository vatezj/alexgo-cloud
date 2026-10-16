# 文件存储（子项目 B / P1-1）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 交付 `pkg/storage` 本地盘驱动 + infra 统一上传/读取 API + `system_files` 元数据表，以「公告附件」一个真实消费方完成验收闭环，解锁头像/codegen 产物/Excel 落盘的依赖面。

**Architecture:** 方案甲——`pkg/storage` 是纯 IO 驱动层（Driver 接口 + LocalDriver，零 DB 零策略）；`modules/system/infra` 簇承载元数据业务层（model/repo/service/controller），路由挂 `/api/admin/infra/file`（JWT+权限码）与 `/api/public/infra/file`（免鉴权，随机 public_key 防枚举）。公告 `SystemNotice` 加 `Attachments` string 字段（JSON 数组文本，原样存取）。

**Tech Stack:** Go（gin + GORM + Fx + glebarez/sqlite 测试）、MySQL SQL 迁移（pkg/migrate）、Vue3/antd（vben form adapter Upload + requestClient 信封拦截器）、vitest。

**Spec:** `docs/superpowers/specs/2026-10-16-file-storage-design.md` —— 计划论证全部来自该 spec，执行者需两文同读。

## Global Constraints

- 统一信封 `{code,msg,data}`，唯一出站出口 `pkg/resp`（`resp.OK/resp.Fail`）；唯一豁免：文件流本体（同 codegen download 先例）。
- 错误码五位制：新增 `30010 文件不存在` / `30011 文件类型不允许` / `30012 文件超过大小限制`（3xxxx → HTTP 400 由 `resp.statusFor` 按段自动映射，勿改 resp）。分层惯例（a9311e6 后）：service 返 `errs.New(code, cause)`，controller 直传 `resp.Fail(ctx, err)`，禁止 strings.Contains 串桥。
- 存储安全：双 MIME 校验（扩展名白名单 ∩ `http.DetectContentType` 头 512 字节）；白名单**刻意不含 svg/html**；public 读响应必须带 `X-Content-Type-Options: nosniff`。
- 随机量：`crypto/rand` 16 字节 hex——`rel_path = yyyy/mm/<32hex>.<ext>`、`public_key = 32hex`（恒生成，含 private 行）。
- 默认配置：`storage.root=./data/uploads`（启动 MkdirAll）、`max_size_mb=10`、白名单 `jpg,jpeg,png,gif,webp,pdf,zip,xlsx`（`allowed_exts` 可覆盖，空则内置）。
- 租户：写与 admin 读手填 `tenant.TenantIDFromContext(ctx)`；**public 读是唯一不带租户过滤的查询**（公告附件跨端可见是产品语义）。
- public 端点免鉴权不免信封（成功/失败仍走 pkg/resp）；`biz_type` 仅归类元数据，后端不校验枚举。
- 删除语义：行删成、盘删败 → zap warn 不报错 → 200；Upload Save 成功但落库失败 → 盘删补偿 + warn 孤儿路径 → 裸错 50000。
- Git：直接落 main、不建功能分支、不 push；conventional 中文提交；**代码提交与 docs/roadmap 提交分开**；git 命令必须从仓库根 `/Users/alex/Desktop/goWork/alexGo-cloud` 跑（cwd 会滞留子目录）。
- 环境：`CGO_ENABLED=0`（本机 cgo 链接坏）；typecheck = `cd apps/web-antd && pnpm typecheck`；测试驱动 `github.com/glebarez/sqlite`（纯 Go，勿用 cgo sqlite）。
- 迁移落地后跑 `./scripts/gen-full-sql.sh` 重生成 `docs/sql/alexgo-cloud-full.sql`（该脚本与 docs/sql 目前未跟踪，随本子项目一并入库）。

## Review Focus

以下五条是 spec 隐含、任务测试最可能漏、且咬人最疼的输入/条件；每条已挂到拥有它的任务的测试步骤里：

1. **public 读被租户条件过滤**——`GetByKey` 若意外带上 `tenant_id` 条件，单租户测试全绿，但跨端/跨租户公告附件一律 404。→ Task 4 测试：租户 1 写入后，**租户 99 的 ctx** 调 `GetByKey` 必须命中；private 行统一 30010。
2. **伪造 multipart Size**——`FileHeader.Size` 是客户端可伪造的声明值；只按声明值预检则可实传超大文件。→ Task 4 测试：传 `Size=0`（声明撒谎）但 Reader 实际 > max，必须 30012 **且盘上无残留文件**。
3. **public 路径挂错组或被中间件误拦**——AuthMiddleware 只拦 `/api/admin/**`；若 public 路由挂进 admin 组（或路径判定改动），免鉴权端点变 401，公告附件全挂。→ Task 5 测试：`auth_test` 给 `/api/public/infra/file/:key` 注册路由并断言无 token → 200；controller 测试无 Authorization 头打真实 public 路径。
4. **seed 按钮的前缀粒度幂等陷阱**——既有 `hasPermPrefix` 按前缀判；`infra:file:upload/download/delete` 三条逐条判会因组内首条 append 后跳过余下两条（只插 1/3，死权限 = 403）。→ Task 6：三条绑一次 `hasPermPrefix(allMenus, "infra:file")` 判据 + `seed_test` 钉三条形态与 `permissionRoutes["infra:file"]` 同源。
5. **公告附件走了 private 上传**——`public:false` 的附件 URL 在 `/api/public/...` 下恒 404（private 行统一 30010），公告一发附件即死。→ Task 8 测试：`uploadNoticeAttachment` 经 `vi.mock` 断言必须以 `{public:true, bizType:'notice'}` 调 `uploadFile`，且 `public_url` 映射进 `url`。

---

## File Structure

| 动作 | 路径 | 职责 |
|---|---|---|
| Create | `pkg/storage/storage.go` | Driver 接口 + `ErrNotFound` 哨兵（纯 IO 合同） |
| Create | `pkg/storage/local.go` | LocalDriver：MkdirAll 盘根、穿越双保险、`O_EXCL` 防覆盖 |
| Create | `pkg/storage/local_test.go` | 往返/ErrNotFound/穿越/O_EXCL 钉驱动行为 |
| Modify | `pkg/errors/code.go` | 30010/30011/30012 常量 + registry 中文文案 |
| Create | `pkg/errors/file_code_test.go` | 三码文案回归 |
| Modify | `pkg/config/config.go` | `Storage{Root,MaxSizeMB,AllowedExts}` 段 |
| Modify | `alexgo-server/configs/config.yaml` | `storage:` 段 |
| Create | `modules/system/infra/model/file.go` | `SystemFile` 元数据模型 |
| Create | `modules/system/infra/migrations/20261016000001_system_files.up.sql` | 生产建表 |
| Create | `modules/system/infra/migrations/20261016000001_system_files.down.sql` | 回滚 |
| Create | `modules/system/infra/repository/file_repo.go` | CRUD；`GetByKey` 唯一无租户过滤 |
| Create | `modules/system/infra/repository/file_repo_test.go` | 租户边界钉仓库查询 |
| Create | `modules/system/infra/service/file_service.go` | 策略层：白名单/双 MIME/大小/随机路径/落库补偿 |
| Create | `modules/system/infra/service/file_service_test.go` | Review Focus ①② 的归属地 |
| Create | `modules/system/infra/controller/admin/file_controller.go` | multipart 上传 + 受控下载 + public 流式读 |
| Create | `modules/system/infra/controller/admin/file_controller_test.go` | 信封形状/public 头/stub 分支 |
| Modify | `modules/system/infra/module.go` | Fx 装配（driver/repo/service/ctrl）+ 双组路由 |
| Modify | `modules/system/service/permission_routes.go` | `"infra:file"` → 路由模板组 |
| Modify | `modules/system/service/seed.go` | `fileStorageButtons` 三条按钮（绑一次前缀判据） |
| Create | `modules/system/service/seed_file_buttons_test.go` | 形态与路由键同源钉 |
| Modify | `pkg/middleware/auth_test.go` | public 路径无 token 放行 case |
| Modify | `modules/system/model/basic.go` | `SystemNotice.Attachments string` |
| Create | `modules/system/migrations/20261016000002_notice_attachments.up.sql` | ALTER 加列 |
| Create | `modules/system/migrations/20261016000002_notice_attachments.down.sql` | 回滚 |
| Create | `modules/system/service/notice_attach_test.go` | 附件字符串往返 |
| Modify | `apps/web-antd/src/api/infra.ts` | `FileMeta` + `uploadFile` |
| Create | `apps/web-antd/src/utils/notice-attachments.ts` | 解析/序列化/上传封装（可测纯函数） |
| Create | `apps/web-antd/src/utils/__tests__/notice-attachments.test.ts` | Review Focus ⑤ 的归属地 |
| Modify | `apps/web-antd/src/api/admin.ts` | `Notice.attachments` + `createNotice` payload |
| Modify | `apps/web-antd/src/views/system/SystemNoticesPage.vue` | 表单 Upload + 附件列 |
| Modify | `apps/web-antd/src/views/system/SystemMyNoticesPage.vue` | 详情渲染附件链接 |
| Regen | `docs/sql/alexgo-cloud-full.sql` | `./scripts/gen-full-sql.sh` |
| Modify | `docs/roadmap.md` | P1-1 状态（**docs 单独提交**） |

---

### Task 1: pkg/storage 驱动层（Driver 接口 + LocalDriver）

**Files:**
- Create: `pkg/storage/storage.go`
- Create: `pkg/storage/local.go`
- Test: `pkg/storage/local_test.go`

**Interfaces:**
- Consumes: 无（本子项目最底层）。
- Produces（后续任务合同，签名不得漂移）:
  - `var ErrNotFound error`——Open/Delete 目标不存在的哨兵。
  - `type Driver interface { Save(ctx context.Context, relPath string, r io.Reader) error; Open(ctx context.Context, relPath string) (io.ReadCloser, error); Delete(ctx context.Context, relPath string) error }`
  - `func NewLocalDriver(root string) (*LocalDriver, error)`——构造即 `filepath.Abs` + `MkdirAll(0o755)`。

- [ ] **Step 1: 写失败测试**

```go
// pkg/storage/local_test.go
package storage

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newDriver(t *testing.T) (*LocalDriver, string) {
	t.Helper()
	root := t.TempDir()
	d, err := NewLocalDriver(root)
	if err != nil {
		t.Fatal(err)
	}
	return d, root
}

// Save→Open 字节往返；Delete 后 Open 报 ErrNotFound。
func TestLocalDriver_RoundTripAndDelete(t *testing.T) {
	d, _ := newDriver(t)
	ctx := context.Background()
	if err := d.Save(ctx, "2026/10/aaaa.bin", strings.NewReader("hello")); err != nil {
		t.Fatalf("Save: %v", err)
	}
	rc, err := d.Open(ctx, "2026/10/aaaa.bin")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	b, _ := io.ReadAll(rc)
	_ = rc.Close()
	if string(b) != "hello" {
		t.Errorf("content = %q, want hello", b)
	}
	if err := d.Delete(ctx, "2026/10/aaaa.bin"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := d.Open(ctx, "2026/10/aaaa.bin"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Open after delete err = %v, want ErrNotFound", err)
	}
	if err := d.Delete(ctx, "2026/10/aaaa.bin"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Delete twice err = %v, want ErrNotFound", err)
	}
}

// 路径穿越双保险：../ 逃逸与绝对路径注入都必须被拒，且不得在 root 外落文件。
func TestLocalDriver_TraversalRejected(t *testing.T) {
	d, root := newDriver(t)
	ctx := context.Background()
	for _, rel := range []string{
		"../../etc/passwd",
		"2026/10/../../../outside.txt",
		filepath.Join(root, "injected-abs.txt"),
	} {
		if err := d.Save(ctx, rel, strings.NewReader("x")); err == nil {
			t.Errorf("Save(%q) succeeded, want error", rel)
		}
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(root), "outside.txt")); !os.IsNotExist(err) {
		t.Errorf("file escaped temp root: err=%v", err)
	}
}

// O_EXCL 防覆盖：同 relPath 二次 Save 报错且首份内容不变。
func TestLocalDriver_NoOverwrite(t *testing.T) {
	d, _ := newDriver(t)
	ctx := context.Background()
	if err := d.Save(ctx, "a/b.bin", strings.NewReader("first")); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := d.Save(ctx, "a/b.bin", strings.NewReader("second")); err == nil {
		t.Fatal("second Save succeeded, want O_EXCL error")
	}
	rc, err := d.Open(ctx, "a/b.bin")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(rc)
	_ = rc.Close()
	if string(b) != "first" {
		t.Errorf("content = %q, want first (must not overwrite)", b)
	}
}

// Save 写入中途失败不留半截文件（io.Reader 中途报错）。
func TestLocalDriver_SaveHalfWriteCleaned(t *testing.T) {
	d, root := newDriver(t)
	if err := d.Save(context.Background(), "x/half.bin", errReader{}); err == nil {
		t.Fatal("Save with failing reader succeeded, want error")
	}
	var leftovers []string
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() {
			leftovers = append(leftovers, p)
		}
		return nil
	})
	if len(leftovers) != 0 {
		t.Errorf("leftover files: %v", leftovers)
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("boom") }
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd /Users/alex/Desktop/goWork/alexGo-cloud && CGO_ENABLED=0 go test ./pkg/storage/ -v`
Expected: FAIL（包不存在 / NewLocalDriver undefined）

- [ ] **Step 3: 最小实现**

```go
// pkg/storage/storage.go
// Package storage 纯 IO 驱动层：只按相对路径读写删，零 DB 零策略。
// relPath 由调用方（service）生成——驱动不得把任何用户输入解析进盘路径。
// 将来 S3/OSS 驱动同样只实现 Driver 三个方法。
package storage

import (
	"context"
	"errors"
	"io"
)

// ErrNotFound 目标相对路径不存在（Open/Delete 未命中）。
var ErrNotFound = errors.New("storage: file not found")

// Driver 存储驱动合同。ctx 供远端驱动做请求取消；本地驱动在入口检查一次。
type Driver interface {
	// Save 以独占创建写入；目标已存在必须报错而非覆盖。
	Save(ctx context.Context, relPath string, r io.Reader) error
	// Open 打开读取流；不存在返回 ErrNotFound。
	Open(ctx context.Context, relPath string) (io.ReadCloser, error)
	// Delete 删除文件；不存在返回 ErrNotFound。
	Delete(ctx context.Context, relPath string) error
}
```

```go
// pkg/storage/local.go
package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// LocalDriver 本地盘驱动：root 下按相对路径读写删。
// 防御：构造时 root 转绝对路径；resolve 用 Clean 锚定 + 前缀约束双保险；
// Save 用 O_CREATE|O_EXCL 独占创建，碰撞即报错不覆盖。
type LocalDriver struct {
	root string // 绝对路径、无尾分隔符
}

// NewLocalDriver 构造驱动并 MkdirAll 盘根（0o755）。
func NewLocalDriver(root string) (*LocalDriver, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("storage: abs root: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("storage: mkdir root %q: %w", abs, err)
	}
	return &LocalDriver{root: abs}, nil
}

// resolve 把 relPath 解析为 root 内绝对路径。
// 第一保险：以 "/" 锚定后 Clean——"../" 在根处被吸收，结构上逃不出 root；
// 第二保险：结果必须以 root+分隔符开头（防实现细节变化）。
func (d *LocalDriver) resolve(relPath string) (string, error) {
	if strings.TrimSpace(relPath) == "" {
		return "", fmt.Errorf("storage: empty rel path")
	}
	clean := filepath.Clean("/" + relPath)
	full := filepath.Join(d.root, clean)
	if !strings.HasPrefix(full, d.root+string(os.PathSeparator)) {
		return "", fmt.Errorf("storage: path escapes root: %q", relPath)
	}
	return full, nil
}

// Save 独占创建写入；写入中途失败删除半截文件。
func (d *LocalDriver) Save(ctx context.Context, relPath string, r io.Reader) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	full, err := d.resolve(relPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return fmt.Errorf("storage: mkdir for %q: %w", relPath, err)
	}
	f, err := os.OpenFile(full, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("storage: create %q: %w", relPath, err)
	}
	if _, err := io.Copy(f, r); err != nil {
		_ = f.Close()
		_ = os.Remove(full)
		return fmt.Errorf("storage: write %q: %w", relPath, err)
	}
	return f.Close()
}

// Open 打开读取流；不存在映射 ErrNotFound。
func (d *LocalDriver) Open(ctx context.Context, relPath string) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	full, err := d.resolve(relPath)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(full)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("storage: open %q: %w", relPath, err)
	}
	return f, nil
}

// Delete 删除文件；不存在映射 ErrNotFound。
func (d *LocalDriver) Delete(ctx context.Context, relPath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	full, err := d.resolve(relPath)
	if err != nil {
		return err
	}
	if err := os.Remove(full); err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound
		}
		return fmt.Errorf("storage: remove %q: %w", relPath, err)
	}
	return nil
}
```

注意 `filepath.Join(root, "/abs/path")` 会吞掉绝对前缀（Go 语义），所以穿越必须靠 Clean 锚定；测试里的绝对路径注入用例就是钉这一点。

- [ ] **Step 4: 跑测试确认通过**

Run: `cd /Users/alex/Desktop/goWork/alexGo-cloud && CGO_ENABLED=0 go test ./pkg/storage/ -v`
Expected: PASS（4 个用例全绿）

- [ ] **Step 5: 提交**

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud
git add pkg/storage/
git commit -m "feat(storage): pkg/storage 本地盘驱动（Driver 合同 + 穿越双保险 + O_EXCL 防覆盖）"
```

---

### Task 2: 错误码 30010-30012 + storage 配置段

**Files:**
- Modify: `pkg/errors/code.go`（常量区 30009 之后 + registry 表）
- Create: `pkg/errors/file_code_test.go`
- Modify: `pkg/config/config.go`（Config struct 尾部 Codegen 段之后）
- Modify: `alexgo-server/configs/config.yaml`（文件末尾追加）

**Interfaces:**
- Consumes: 无。
- Produces:
  - `errs.CodeFileNotFound`(30010) / `errs.CodeFileTypeNotAllowed`(30011) / `errs.CodeFileTooLarge`(30012)——Task 4/5 的 service/controller 直接引用。
  - `config.Config.Storage{Root string; MaxSizeMB int; AllowedExts []string}`（mapstructure `storage`）——Task 4 service 构造读取。

- [ ] **Step 1: 写失败测试**

```go
// pkg/errors/file_code_test.go
package errors

import "testing"

// 文件存储三码文案回归：文案写死中文、码值固定（30010-30012 与 spec §7 一致）。
func TestFileErrorCodes(t *testing.T) {
	cases := []struct {
		code Code
		want string
	}{
		{CodeFileNotFound, "文件不存在"},
		{CodeFileTypeNotAllowed, "文件类型不允许"},
		{CodeFileTooLarge, "文件超过大小限制"},
	}
	for _, tc := range cases {
		if got := tc.code.Msg(); got != tc.want {
			t.Errorf("Code(%d).Msg() = %q, want %q", tc.code, got, tc.want)
		}
	}
}
```

注意：`Code` 的文案取法——若 `Code` 类型没有 `Msg()` 方法（registry 是包内 map），改用既有取文案的公开函数。先看 `pkg/errors` 现有导出：`registry` 是私有 map；公开入口以实际为准——若无公开取法，改为断言 `New(tc.code, nil).Error()` 返回的信封文案（`BizError.Msg()`），即：

```go
		be := New(tc.code, nil)
		if got := be.Msg(); got != tc.want {
			t.Errorf("New(%d).Msg() = %q, want %q", tc.code, got, tc.want)
		}
```

（执行时以 `pkg/errors` 真实导出为准选其一，两版都是完整可跑代码。）

- [ ] **Step 2: 跑测试确认失败**

Run: `cd /Users/alex/Desktop/goWork/alexGo-cloud && CGO_ENABLED=0 go test ./pkg/errors/ -run TestFileErrorCodes -v`
Expected: FAIL（CodeFileNotFound undefined）

- [ ] **Step 3: 实现**

`pkg/errors/code.go` 常量区，`CodeMobileTaken Code = 30009` 之后、`// 5xxxx 系统` 之前：

```go
	// CodeFileNotFound file：按 id/key 查无；public 读 private 行也归此（不泄露存在性）。
	CodeFileNotFound Code = 30010
	// CodeFileTypeNotAllowed file：扩展名不在白名单或探测 mime 与扩展名不符。
	CodeFileTypeNotAllowed Code = 30011
	// CodeFileTooLarge file：超过 storage.max_size_mb 上限。
	CodeFileTooLarge Code = 30012
```

registry 表尾部（`CodeMobileTaken` 条目之后）：

```go
	CodeFileNotFound:        "文件不存在",
	CodeFileTypeNotAllowed:  "文件类型不允许",
	CodeFileTooLarge:        "文件超过大小限制",
```

`pkg/config/config.go`，`Codegen` 段之后、struct 收尾 `}` 之前：

```go
	// Storage：文件存储（pkg/storage 本地盘驱动，子项目 B / P1-1）。
	// 不做 Enabled 开关：本地盘无外部依赖（与 Redis/MQ 的可选依赖不同），YAGNI。
	Storage struct {
		// Root：盘根目录（相对路径按进程 cwd 解析），驱动构造时 MkdirAll。
		Root string `mapstructure:"root"`
		// MaxSizeMB：单文件大小上限（MB）；<=0 时 service 落默认 10。
		MaxSizeMB int `mapstructure:"max_size_mb"`
		// AllowedExts：扩展名白名单（小写、无点）；空则用 service 内置白名单。
		AllowedExts []string `mapstructure:"allowed_exts"`
	} `mapstructure:"storage"`
```

`alexgo-server/configs/config.yaml` 末尾追加：

```yaml
storage:
  root: ./data/uploads        # 盘根，驱动构造时 MkdirAll
  max_size_mb: 10             # 单文件上限
  # allowed_exts: [jpg, jpeg, png, gif, webp, pdf, zip, xlsx]  # 省略则用内置白名单
```

- [ ] **Step 4: 跑测试与构建确认通过**

Run: `cd /Users/alex/Desktop/goWork/alexGo-cloud && CGO_ENABLED=0 go test ./pkg/errors/ -v && CGO_ENABLED=0 go build ./pkg/...`
Expected: PASS + 构建成功

- [ ] **Step 5: 提交**

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud
git add pkg/errors/code.go pkg/errors/file_code_test.go pkg/config/config.go alexgo-server/configs/config.yaml
git commit -m "feat(errors,config): 文件存储错误码 30010-30012 + storage 配置段"
```

---

### Task 3: system_files 表（迁移 + model + repo）

**Files:**
- Create: `modules/system/infra/model/file.go`
- Create: `modules/system/infra/migrations/20261016000001_system_files.up.sql`
- Create: `modules/system/infra/migrations/20261016000001_system_files.down.sql`
- Create: `modules/system/infra/repository/file_repo.go`
- Test: `modules/system/infra/repository/file_repo_test.go`

**Interfaces:**
- Consumes: 无（Task 2 的 config 不在本层）。
- Produces（Task 4 service 合同）:
  - `model.SystemFile` 字段见下（json tag 决定 controller 出站形状）。
  - `type FileRepository interface { Create(ctx context.Context, f *model.SystemFile) error; GetByID(ctx context.Context, tenantID, id uint64) (*model.SystemFile, error); GetByKey(ctx context.Context, key string) (*model.SystemFile, error); Delete(ctx context.Context, tenantID, id uint64) error }`，未命中一律返回 `gorm.ErrRecordNotFound`。
  - `func NewFileRepository(db *gorm.DB) FileRepository`。

- [ ] **Step 1: 写失败测试**

```go
// modules/system/infra/repository/file_repo_test.go
package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/modules/system/infra/model"
	"alexGo-cloud/pkg/tenant"
)

func setupFileRepo(t *testing.T) (*gorm.DB, FileRepository) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.SystemFile{}); err != nil {
		t.Fatal(err)
	}
	return db, NewFileRepository(db)
}

func sampleFile(tenantID uint64, key string) *model.SystemFile {
	return &model.SystemFile{
		Name: "照片.jpg", Ext: "jpg", Mime: "image/jpeg", Size: 12,
		RelPath: "2026/10/deadbeef.jpg", PublicKey: key,
		Public: true, BizType: "notice", TenantID: tenantID,
	}
}

// CRUD + 租户边界：admin 读按 tenant 隔离；GetByKey 不受租户过滤（public 读唯一例外）。
func TestFileRepo_CRUDAndTenantBoundary(t *testing.T) {
	db, repo := setupFileRepo(t)
	ctx1 := tenant.WithTenantID(context.Background(), 1)

	f := sampleFile(1, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err := repo.Create(ctx1, f); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// 本租户命中
	got, err := repo.GetByID(ctx1, 1, f.ID)
	if err != nil || got.Name != "照片.jpg" {
		t.Fatalf("GetByID own tenant: got=%+v err=%v", got, err)
	}
	// 跨租户查无
	if _, err := repo.GetByID(ctx1, 99, f.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("GetByID cross tenant err = %v, want ErrRecordNotFound", err)
	}
	// GetByKey 不带租户条件：另一个租户的 ctx 同样命中（Review Focus ① 的仓库侧半边）
	ctx99 := tenant.WithTenantID(context.Background(), 99)
	gk, err := repo.GetByKey(ctx99, f.PublicKey)
	if err != nil || gk.ID != f.ID {
		t.Errorf("GetByKey cross tenant ctx: got=%+v err=%v, want hit", gk, err)
	}
	// key 不存在
	if _, err := repo.GetByKey(ctx1, "ffffffffffffffffffffffffffffffff"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("GetByKey missing err = %v, want ErrRecordNotFound", err)
	}
	// 删除按租户隔离：错误租户删不掉
	if err := repo.Delete(ctx1, 99, f.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("Delete wrong tenant err = %v, want ErrRecordNotFound", err)
	}
	var n int64
	db.Model(&model.SystemFile{}).Count(&n)
	if n != 1 {
		t.Fatalf("row count = %d after wrong-tenant delete, want 1", n)
	}
	if err := repo.Delete(ctx1, 1, f.ID); err != nil {
		t.Fatalf("Delete own tenant: %v", err)
	}
	if _, err := repo.GetByID(ctx1, 1, f.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("GetByID after delete err = %v, want ErrRecordNotFound", err)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd /Users/alex/Desktop/goWork/alexGo-cloud && CGO_ENABLED=0 go test ./modules/system/infra/repository/ -run TestFileRepo -v`
Expected: FAIL（model.SystemFile undefined）

- [ ] **Step 3: 实现**

```go
// modules/system/infra/model/file.go
package model

import "time"

// SystemFile 文件元数据（子项目 B / P1-1 spec §2）。
// RelPath 只做盘内定位、绝不外泄给客户端；PublicKey 才是对外读键——
// 32 hex 恒生成（含 private 行）：避开 unique 索引空串冲突，是否放行只看 Public。
type SystemFile struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:255" json:"name"` // 原始文件名：只进元数据/Disposition，不进盘路径
	Ext       string    `gorm:"size:32" json:"ext"`   // 小写扩展名、无点
	Mime      string    `gorm:"size:128" json:"mime"` // http.DetectContentType 探测值
	Size      int64     `json:"size"`
	RelPath   string    `gorm:"column:rel_path;size:255;uniqueIndex" json:"rel_path"`
	PublicKey string    `gorm:"column:public_key;size:32;uniqueIndex" json:"public_key"`
	Public    bool      `json:"public"`
	BizType   string    `gorm:"column:biz_type;size:64" json:"biz_type"` // notice/general，仅归类
	TenantID  uint64    `gorm:"column:tenant_id;index" json:"tenant_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (SystemFile) TableName() string { return "system_files" }
```

```sql
-- modules/system/infra/migrations/20261016000001_system_files.up.sql
-- 文件元数据表（子项目 B / P1-1 spec §2）：管理端受控读写带 tenant_id，
-- public 读按 public_key 直取不走过滤（GetByKey，产品语义）。
CREATE TABLE IF NOT EXISTS `system_files` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(255) NOT NULL DEFAULT '',
  `ext` varchar(32) NOT NULL DEFAULT '',
  `mime` varchar(128) NOT NULL DEFAULT '',
  `size` bigint NOT NULL DEFAULT 0,
  `rel_path` varchar(255) NOT NULL,
  `public_key` varchar(32) NOT NULL,
  `public` tinyint(1) NOT NULL DEFAULT 0,
  `biz_type` varchar(64) NOT NULL DEFAULT '',
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0,
  `created_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_system_files_rel_path` (`rel_path`),
  UNIQUE KEY `uk_system_files_public_key` (`public_key`),
  KEY `idx_system_files_tenant` (`tenant_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

```sql
-- modules/system/infra/migrations/20261016000001_system_files.down.sql
DROP TABLE IF EXISTS `system_files`;
```

```go
// modules/system/infra/repository/file_repo.go
package repository

import (
	"context"

	"gorm.io/gorm"

	"alexGo-cloud/modules/system/infra/model"
)

// FileRepository 元数据 CRUD。未命中一律 gorm.ErrRecordNotFound（service 层转 30010）。
type FileRepository interface {
	Create(ctx context.Context, f *model.SystemFile) error
	// GetByID 受控读：按租户隔离。
	GetByID(ctx context.Context, tenantID, id uint64) (*model.SystemFile, error)
	// GetByKey public 读：**唯一不带租户过滤的查询**——公告附件跨端可见是产品语义
	// （spec §2），单租户测试全绿不代表这里可以加 tenant 条件。
	GetByKey(ctx context.Context, key string) (*model.SystemFile, error)
	// Delete 按租户隔离硬删行；0 行影响返回 ErrRecordNotFound。
	Delete(ctx context.Context, tenantID, id uint64) error
}

type fileRepo struct{ db *gorm.DB }

// NewFileRepository 构造文件元数据仓库。
func NewFileRepository(db *gorm.DB) FileRepository { return &fileRepo{db: db} }

func (r *fileRepo) Create(ctx context.Context, f *model.SystemFile) error {
	return r.db.WithContext(ctx).Create(f).Error
}

func (r *fileRepo) GetByID(ctx context.Context, tenantID, id uint64) (*model.SystemFile, error) {
	var f model.SystemFile
	err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).First(&f).Error
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (r *fileRepo) GetByKey(ctx context.Context, key string) (*model.SystemFile, error) {
	var f model.SystemFile
	err := r.db.WithContext(ctx).Where("public_key = ?", key).First(&f).Error
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (r *fileRepo) Delete(ctx context.Context, tenantID, id uint64) error {
	res := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).Delete(&model.SystemFile{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
```

- [ ] **Step 4: 跑测试确认通过（含 infra 包整体回归）**

Run: `cd /Users/alex/Desktop/goWork/alexGo-cloud && CGO_ENABLED=0 go test ./modules/system/infra/... -v`
Expected: PASS（既有 codegen 测试不受影响）

- [ ] **Step 5: 提交**

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud
git add modules/system/infra/model/file.go modules/system/infra/migrations/20261016000001_system_files.up.sql modules/system/infra/migrations/20261016000001_system_files.down.sql modules/system/infra/repository/file_repo.go modules/system/infra/repository/file_repo_test.go
git commit -m "feat(infra): system_files 表（SQL 迁移 + model + 仓库，GetByKey 无租户过滤）"
```

---

### Task 4: file_service 上传策略与元数据编排

**Files:**
- Create: `modules/system/infra/service/file_service.go`
- Test: `modules/system/infra/service/file_service_test.go`

**Interfaces:**
- Consumes:
  - `storage.Driver` / `storage.ErrNotFound`（Task 1）
  - `errs.CodeFileNotFound/CodeFileTypeNotAllowed/CodeFileTooLarge`（Task 2）
  - `model.SystemFile`、`repository.FileRepository`（Task 3）
  - `tenant.TenantIDFromContext(ctx)`（`alexGo-cloud/pkg/tenant`）
- Produces（Task 5 controller 合同）:
  - `type UploadInput struct { Name string; Size int64; Reader io.Reader; Public bool; BizType string }`——controller 从 `FileHeader`/PostForm 填。
  - `type FileService interface { Upload(ctx context.Context, in UploadInput) (*model.SystemFile, error); GetByID(ctx context.Context, id uint64) (*model.SystemFile, error); GetByKey(ctx context.Context, key string) (*model.SystemFile, error); Open(ctx context.Context, f *model.SystemFile) (io.ReadCloser, error); Delete(ctx context.Context, id uint64) error }`
  - `func NewFileService(repo repository.FileRepository, driver storage.Driver, cfg *config.Config) FileService`
  - 行为合同：所有校验类失败返 `errs.New(...)`；查无（gorm.ErrRecordNotFound / storage.ErrNotFound）映射 `CodeFileNotFound`。

- [ ] **Step 1: 写失败测试**

```go
// modules/system/infra/service/file_service_test.go
package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/modules/system/infra/model"
	"alexGo-cloud/modules/system/infra/repository"
	"alexGo-cloud/pkg/config"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/storage"
	"alexGo-cloud/pkg/tenant"
)

// newFileSvc 真 sqlite repo + 真 LocalDriver(t.TempDir())，cfg 可调大小上限。
func newFileSvc(t *testing.T, maxSizeMB int) (FileService, *gorm.DB, string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.SystemFile{}); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	drv, err := storage.NewLocalDriver(root)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.Storage.MaxSizeMB = maxSizeMB
	return NewFileService(repository.NewFileRepository(db), drv, cfg), db, root
}

// jpegBytes 头 3 字节 FF D8 FF——DetectContentType 判 image/jpeg。
func jpegBytes(n int) []byte {
	b := bytes.Repeat([]byte{0x00}, n)
	copy(b, []byte{0xFF, 0xD8, 0xFF, 0xE0})
	return b
}

var relPathRe = regexp.MustCompile(`^\d{4}/\d{2}/[0-9a-f]{32}\.[a-z]+$`)

// 成功路径：分桶 rel_path、32hex public_key、落库字段齐全、盘上有真实字节。
func TestUpload_Success(t *testing.T) {
	svc, db, root := newFileSvc(t, 1)
	ctx := tenant.WithTenantID(context.Background(), 1)

	meta, err := svc.Upload(ctx, UploadInput{
		Name: "照片.JPG", Size: 4, Reader: bytes.NewReader(jpegBytes(4)),
		Public: true, BizType: "notice",
	})
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if !relPathRe.MatchString(meta.RelPath) {
		t.Errorf("RelPath = %q, want yyyy/mm/<32hex>.<ext>", meta.RelPath)
	}
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(meta.PublicKey) {
		t.Errorf("PublicKey = %q, want 32 hex", meta.PublicKey)
	}
	if meta.Ext != "jpg" || meta.Mime != "image/jpeg" || meta.Size != 4 {
		t.Errorf("ext/mime/size = %q/%q/%d, want jpg/image/jpeg/4", meta.Ext, meta.Mime, meta.Size)
	}
	if meta.TenantID != 1 || !meta.Public || meta.BizType != "notice" {
		t.Errorf("tenant/public/biz = %d/%v/%q, want 1/true/notice", meta.TenantID, meta.Public, meta.BizType)
	}
	// 盘上有字节
	b, err := os.ReadFile(filepath.Join(root, meta.RelPath))
	if err != nil || len(b) != 4 {
		t.Errorf("disk read: n=%d err=%v", len(b), err)
	}
	// 落库
	var row model.SystemFile
	if err := db.First(&row, meta.ID).Error; err != nil {
		t.Fatalf("db row: %v", err)
	}
	if row.PublicKey != meta.PublicKey {
		t.Errorf("db PublicKey = %q, want %q", row.PublicKey, meta.PublicKey)
	}
}

// 双 MIME 拒：扩展名不在白名单（svg）与头探测不符（jpg 装 gif 头）都是 30011。
func TestUpload_MimeRejections_30011(t *testing.T) {
	svc, _, _ := newFileSvc(t, 1)
	ctx := tenant.WithTenantID(context.Background(), 1)
	for _, tc := range []struct {
		name    string
		in      UploadInput
	}{
		{"svg 白名单拒绝", UploadInput{Name: "a.svg", Reader: bytes.NewReader([]byte("<svg></svg>"))}},
		{"扩展名与探测头不符", UploadInput{Name: "a.jpg", Reader: bytes.NewReader([]byte("GIF89a...."))}},
		{"无扩展名", UploadInput{Name: "noext", Reader: bytes.NewReader(jpegBytes(4))}},
	} {
		if _, err := svc.Upload(ctx, tc.in); !errs.Is(err, errs.CodeFileTypeNotAllowed) {
			t.Errorf("%s: err = %v, want CodeFileTypeNotAllowed", tc.name, err)
		}
	}
}

// 超限 30012——Review Focus ②：Size 声明撒谎（传 0），实际字节流超 max 必须被拒，
// 且盘上不得残留半截文件（字节级限流，而非只信 FileHeader.Size）。
func TestUpload_ByteStreamLimited_DeclaredSizeLies_30012(t *testing.T) {
	svc, _, root := newFileSvc(t, 1) // max = 1MB
	ctx := tenant.WithTenantID(context.Background(), 1)
	over := bytes.Repeat([]byte{0xFF, 0xD8, 0xFF, 0xE0}, (1<<20)/4+1) // > 1MB
	if _, err := svc.Upload(ctx, UploadInput{Name: "big.jpg", Size: 0, Reader: bytes.NewReader(over)}); !errs.Is(err, errs.CodeFileTooLarge) {
		t.Fatalf("oversize stream err = %v, want CodeFileTooLarge", err)
	}
	// 盘上不得残留
	n := countFiles(t, root)
	if n != 0 {
		t.Errorf("leftover files = %d, want 0 (over-limit write must be removed)", n)
	}
	// 声明值撒谎的另一半：Size 声明超限也直接拒
	if _, err := svc.Upload(ctx, UploadInput{Name: "big.jpg", Size: 2 << 20, Reader: bytes.NewReader(jpegBytes(4))}); !errs.Is(err, errs.CodeFileTooLarge) {
		t.Errorf("declared oversize err = %v, want CodeFileTooLarge", err)
	}
}

func countFiles(t *testing.T, root string) int {
	t.Helper()
	n := 0
	_ = filepath.Walk(root, func(_ string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() {
			n++
		}
		return nil
	})
	return n
}

// 租户边界（Review Focus ①）：租户 1 上传后，租户 99 的 ctx 按 key 仍能 GetByKey 命中；
// private 行 GetByKey 统一 30010；GetByID 跨租户 30010。
func TestUpload_GetByKey_TenantBoundary(t *testing.T) {
	svc, _, _ := newFileSvc(t, 1)
	ctx1 := tenant.WithTenantID(context.Background(), 1)

	pub, err := svc.Upload(ctx1, UploadInput{Name: "p.jpg", Reader: bytes.NewReader(jpegBytes(4)), Public: true})
	if err != nil {
		t.Fatal(err)
	}
	priv, err := svc.Upload(ctx1, UploadInput{Name: "q.jpg", Reader: bytes.NewReader(jpegBytes(4)), Public: false})
	if err != nil {
		t.Fatal(err)
	}

	ctx99 := tenant.WithTenantID(context.Background(), 99)
	got, err := svc.GetByKey(ctx99, pub.PublicKey) // 跨租户 ctx 必须命中
	if err != nil || got.ID != pub.ID {
		t.Errorf("GetByKey cross tenant ctx: got=%+v err=%v, want hit", got, err)
	}
	// private 行：统一 30010（不泄露存在性）
	if _, err := svc.GetByKey(ctx99, priv.PublicKey); !errs.Is(err, errs.CodeFileNotFound) {
		t.Errorf("GetByKey private err = %v, want CodeFileNotFound(30010)", err)
	}
	// 跨租户受控读 30010
	if _, err := svc.GetByID(ctx99, pub.ID); !errs.Is(err, errs.CodeFileNotFound) {
		t.Errorf("GetByID cross tenant err = %v, want CodeFileNotFound", err)
	}
	// key 查无 → 30010
	if _, err := svc.GetByKey(ctx99, strings.Repeat("f", 32)); !errs.Is(err, errs.CodeFileNotFound) {
		t.Errorf("GetByKey missing err = %v, want CodeFileNotFound", err)
	}
}

// Delete：行删 + 盘删都生效；盘文件被预先移走（模拟盘删未命中）→ 仍返回 nil
//（spec §7：行删成盘删败不报错，warn 留 rel_path 给后续 GC）。
func TestUpload_Delete_RowAndDisk(t *testing.T) {
	svc, db, root := newFileSvc(t, 1)
	ctx := tenant.WithTenantID(context.Background(), 1)
	meta, err := svc.Upload(ctx, UploadInput{Name: "d.jpg", Reader: bytes.NewReader(jpegBytes(4))})
	if err != nil {
		t.Fatal(err)
	}
	disk := filepath.Join(root, meta.RelPath)
	if err := os.Remove(disk); err != nil { // 先抽掉盘文件
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, meta.ID); err != nil {
		t.Errorf("Delete with missing disk file err = %v, want nil (warn only)", err)
	}
	var n int64
	db.Model(&model.SystemFile{}).Count(&n)
	if n != 0 {
		t.Errorf("row count = %d, want 0", n)
	}
	// 再删一次：行已无 → 30010
	if err := svc.Delete(ctx, meta.ID); !errs.Is(err, errs.CodeFileNotFound) {
		t.Errorf("Delete twice err = %v, want CodeFileNotFound", err)
	}
}

// 落库失败补偿：repo.Create 报错 → 盘上不得留孤儿文件（spec §7）。
func TestUpload_CreateFails_DiskCompensated(t *testing.T) {
	drvRoot := t.TempDir()
	drv, err := storage.NewLocalDriver(drvRoot)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.Storage.MaxSizeMB = 1
	svc := NewFileService(failingCreateRepo{}, drv, cfg)
	ctx := tenant.WithTenantID(context.Background(), 1)
	if _, err := svc.Upload(ctx, UploadInput{Name: "o.jpg", Reader: bytes.NewReader(jpegBytes(4))}); err == nil {
		t.Fatal("Upload with failing repo succeeded, want error")
	}
	if n := countFiles(t, drvRoot); n != 0 {
		t.Errorf("orphan files = %d, want 0 (compensated)", n)
	}
}

// failingCreateRepo Create 恒失败，其余方法不被调用。
type failingCreateRepo struct{}

func (failingCreateRepo) Create(context.Context, *model.SystemFile) error {
	return errors.New("db down")
}
func (failingCreateRepo) GetByID(context.Context, uint64, uint64) (*model.SystemFile, error) {
	return nil, errors.New("unused")
}
func (failingCreateRepo) GetByKey(context.Context, string) (*model.SystemFile, error) {
	return nil, errors.New("unused")
}
func (failingCreateRepo) Delete(context.Context, uint64, uint64) error { return errors.New("unused") }

var _ io.Reader = bytes.NewReader(nil) // 保持 io 导入被使用（Reader 合同）
```

（最后那行 `var _ io.Reader` 若 linter 抱怨可直接删掉 import `io`——以实际编译为准，两版均可。）

- [ ] **Step 2: 跑测试确认失败**

Run: `cd /Users/alex/Desktop/goWork/alexGo-cloud && CGO_ENABLED=0 go test ./modules/system/infra/service/ -run 'TestUpload' -v`
Expected: FAIL（FileService / NewFileService undefined）

- [ ] **Step 3: 实现**

```go
// modules/system/infra/service/file_service.go
package service

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"

	"alexGo-cloud/modules/system/infra/model"
	"alexGo-cloud/modules/system/infra/repository"
	"alexGo-cloud/pkg/config"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/logger"
	"alexGo-cloud/pkg/storage"
	"alexGo-cloud/pkg/tenant"
)

// defaultAllowedExts 内置白名单（spec §3/§4）：刻意不含 svg/html（存储型 XSS 面）。
var defaultAllowedExts = []string{"jpg", "jpeg", "png", "gif", "webp", "pdf", "zip", "xlsx"}

// extMimeOK 扩展名 → DetectContentType 允许值集合（双 MIME 校验的第二半）。
// zip/xlsx 都是 PK 容器，探测值同为 application/zip。
var extMimeOK = map[string][]string{
	"jpg":  {"image/jpeg"},
	"jpeg": {"image/jpeg"},
	"png":  {"image/png"},
	"gif":  {"image/gif"},
	"webp": {"image/webp"},
	"pdf":  {"application/pdf"},
	"zip":  {"application/zip"},
	"xlsx": {"application/zip"},
}

// UploadInput controller 侧收集的上传输入。
// Size 只作预检快拒——最终以字节流限流为准（FileHeader.Size 客户端可伪造）。
type UploadInput struct {
	Name    string // 原始文件名（含扩展名）
	Size    int64  // 声明大小（预检）
	Reader  io.Reader
	Public  bool
	BizType string // 仅归类元数据，不校验枚举
}

// FileService 文件元数据业务：策略（随机名/白名单/大小/双 MIME）+ 落库 + 驱动编排。
type FileService interface {
	Upload(ctx context.Context, in UploadInput) (*model.SystemFile, error)
	GetByID(ctx context.Context, id uint64) (*model.SystemFile, error)
	// GetByKey public 读入口：private/不存在统一 CodeFileNotFound（不泄露存在性）。
	GetByKey(ctx context.Context, key string) (*model.SystemFile, error)
	Open(ctx context.Context, f *model.SystemFile) (io.ReadCloser, error)
	Delete(ctx context.Context, id uint64) error
}

type fileService struct {
	repo   repository.FileRepository
	driver storage.Driver
	max    int64 // 字节上限
	exts   map[string]struct{}
}

// NewFileService 构造文件服务。maxSizeMB<=0 落默认 10（config 缺省防御）。
func NewFileService(repo repository.FileRepository, driver storage.Driver, cfg *config.Config) FileService {
	mb := 10
	exts := defaultAllowedExts
	if cfg != nil {
		if cfg.Storage.MaxSizeMB > 0 {
			mb = cfg.Storage.MaxSizeMB
		}
		if len(cfg.Storage.AllowedExts) > 0 {
			exts = cfg.Storage.AllowedExts
		}
	}
	set := make(map[string]struct{}, len(exts))
	for _, e := range exts {
		set[strings.ToLower(strings.TrimSpace(e))] = struct{}{}
	}
	return &fileService{repo: repo, driver: driver, max: int64(mb) * 1024 * 1024, exts: set}
}

// Upload 全流程（spec §5）：白名单 → 大小双保险 → 头探测双 MIME → 生成路径 →
// Save（限流字节）→ 超限补偿删 → 落库（失败盘删补偿）。
func (s *fileService) Upload(ctx context.Context, in UploadInput) (*model.SystemFile, error) {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(in.Name), "."))
	if ext == "" {
		return nil, errs.New(errs.CodeFileTypeNotAllowed, fmt.Errorf("no extension in %q", in.Name))
	}
	if _, ok := s.exts[ext]; !ok {
		return nil, errs.New(errs.CodeFileTypeNotAllowed, fmt.Errorf("ext %q not in allowlist", ext))
	}
	if in.Size > s.max {
		return nil, errs.New(errs.CodeFileTooLarge, fmt.Errorf("declared %d > max %d", in.Size, s.max))
	}

	// 头 512 字节探测：Peek 不消费，后续经同一 bufio 流入 Save。
	br := bufio.NewReaderSize(in.Reader, 512)
	head, _ := br.Peek(512)
	sniffed := http.DetectContentType(head)
	allowedMimes, known := extMimeOK[ext]
	if !known {
		return nil, errs.New(errs.CodeFileTypeNotAllowed, fmt.Errorf("no mime mapping for ext %q", ext))
	}
	matched := false
	for _, m := range allowedMimes {
		if sniffed == m {
			matched = true
			break
		}
	}
	if !matched {
		return nil, errs.New(errs.CodeFileTypeNotAllowed,
			fmt.Errorf("sniffed %q mismatches ext %q", sniffed, ext))
	}

	relPath, publicKey, err := s.genPaths(ext)
	if err != nil {
		return nil, err
	}

	// 字节级限流：写 max+1 字节后仍可读 → 超限，删掉刚写的文件再拒。
	lr := &io.LimitedReader{R: br, N: s.max + 1}
	cr := &countingReader{r: lr}
	if err := s.driver.Save(ctx, relPath, cr); err != nil {
		return nil, err // O_EXCL 碰撞/IO 裸错 → resp.Fail 兜底 50000
	}
	if cr.n > s.max {
		if derr := s.driver.Delete(ctx, relPath); derr != nil && !errors.Is(derr, storage.ErrNotFound) {
			warnf("oversize cleanup failed", zap.String("rel_path", relPath), zap.Error(derr))
		}
		return nil, errs.New(errs.CodeFileTooLarge, fmt.Errorf("stream %d > max %d", cr.n, s.max))
	}

	f := &model.SystemFile{
		Name: in.Name, Ext: ext, Mime: sniffed, Size: cr.n,
		RelPath: relPath, PublicKey: publicKey,
		Public: in.Public, BizType: in.BizType,
		TenantID: tenant.TenantIDFromContext(ctx), CreatedAt: time.Now(),
	}
	if err := s.repo.Create(ctx, f); err != nil {
		// 补偿：盘删尽力；删不掉只留 warn（孤儿路径靠日志人工/后续 GC）。
		if derr := s.driver.Delete(ctx, relPath); derr != nil && !errors.Is(derr, storage.ErrNotFound) {
			warnf("orphan file after db failure", zap.String("rel_path", relPath), zap.Error(derr))
		}
		return nil, err
	}
	return f, nil
}

// genPaths 分桶相对路径 + 独立 public_key（均 crypto/rand 16 字节 hex）。
func (s *fileService) genPaths(ext string) (relPath, publicKey string, err error) {
	id, err := randHex()
	if err != nil {
		return "", "", err
	}
	key, err := randHex()
	if err != nil {
		return "", "", err
	}
	return time.Now().Format("2006/01") + "/" + id + "." + ext, key, nil
}

func randHex() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("storage: rand: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func (s *fileService) GetByID(ctx context.Context, id uint64) (*model.SystemFile, error) {
	f, err := s.repo.GetByID(ctx, tenant.TenantIDFromContext(ctx), id)
	if err != nil {
		if errors.Is(err, gormNotFound(err)) {
			return nil, errs.New(errs.CodeFileNotFound, nil)
		}
		return nil, err
	}
	return f, nil
}

func (s *fileService) GetByKey(ctx context.Context, key string) (*model.SystemFile, error) {
	f, err := s.repo.GetByKey(ctx, key)
	if err != nil {
		if errors.Is(err, gormNotFound(err)) {
			return nil, errs.New(errs.CodeFileNotFound, nil)
		}
		return nil, err
	}
	if !f.Public {
		// private 行与不存在同码（30010），不泄露存在性（spec §7）。
		return nil, errs.New(errs.CodeFileNotFound, nil)
	}
	return f, nil
}

func (s *fileService) Open(ctx context.Context, f *model.SystemFile) (io.ReadCloser, error) {
	rc, err := s.driver.Open(ctx, f.RelPath)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, errs.New(errs.CodeFileNotFound, nil)
		}
		return nil, err
	}
	return rc, nil
}

func (s *fileService) Delete(ctx context.Context, id uint64) error {
	f, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, tenant.TenantIDFromContext(ctx), id); err != nil {
		return err
	}
	// 行删成、盘删败 → 不回滚不报错，warn 留 rel_path（spec §7，GC 后续票）。
	if err := s.driver.Delete(ctx, f.RelPath); err != nil && !errors.Is(err, storage.ErrNotFound) {
		warnf("disk delete failed", zap.String("rel_path", f.RelPath), zap.Error(err))
	}
	return nil
}

// countingReader 统计实际写入字节数（超限判定依据）。
type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}
```

实现注：`gormNotFound(err)` 是 `func gormNotFound(err error) bool { return errors.Is(err, gorm.ErrRecordNotFound) }` 的就地写法——直接写 `errors.Is(err, gorm.ErrRecordNotFound)` 即可，import `gorm.io/gorm`；`warnf` 就地展开为：

```go
func warnf(msg string, fields ...zap.Field) {
	if logger.Log != nil {
		logger.Log.Warn(msg, fields...)
	}
}
```

（若 lint 要求注释，补一行 "// warnf 日志缺省不 panic——logger 未初始化时静默跳过。"）

- [ ] **Step 4: 跑测试确认通过**

Run: `cd /Users/alex/Desktop/goWork/alexGo-cloud && CGO_ENABLED=0 go test ./modules/system/infra/service/ -v`
Expected: PASS（TestUpload_* 全绿；注意 TestUpload_ByteStreamLimited 用例构造 1MB 内存，秒级可跑）

- [ ] **Step 5: 提交**

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud
git add modules/system/infra/service/file_service.go modules/system/infra/service/file_service_test.go
git commit -m "feat(infra): file_service 上传策略（白名单/双 MIME/字节限流/随机路径/落库补偿）"
```

---

### Task 5: file controller + 模块装配 + 中间件放行钉

**Files:**
- Create: `modules/system/infra/controller/admin/file_controller.go`
- Test: `modules/system/infra/controller/admin/file_controller_test.go`
- Modify: `modules/system/infra/module.go`（Fx provider + `NewModule` 签名 + `RegisterRoutes`）
- Modify: `pkg/middleware/auth_test.go`（public 路径放行 case）

**Interfaces:**
- Consumes:
  - Task 4 的 `service.FileService` / `service.UploadInput`
  - `resp.OK/resp.Fail`、`errs.CodeParamMissing/CodeFileTypeNotAllowed`（测试断言用）
  - `server.Module` 接口（`RegisterRoutes(r *gin.RouterGroup)`，入参是 `/api` group，见 `alexgo-server/server/http.go`）
- Produces:
  - 路由：`POST /api/admin/infra/file/upload`、`GET /api/admin/infra/file/:id`、`DELETE /api/admin/infra/file/:id`、`GET /api/public/infra/file/:key`。
  - `func NewFileController(svc service.FileService) *FileController`；`(*FileController).RegisterAdminRoutes(g *gin.RouterGroup)` / `RegisterPublicRoutes(g *gin.RouterGroup)`。
  - Fx：`newStorageDriver(cfg *config.Config) (storage.Driver, error)`（构造即 MkdirAll）。

- [ ] **Step 1: 写失败测试（controller + 中间件放行）**

```go
// modules/system/infra/controller/admin/file_controller_test.go
package admin

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/system/infra/model"
	errs "alexGo-cloud/pkg/errors"
)

// fileStub 记录调用、按 case 返回。
type fileStub struct {
	upload    *model.SystemFile
	uploadErr error
	gotIn     serviceUploadInput // 捕获传入参数
ByKey     *model.SystemFile
	getByKeyErr error
	openRc   io.ReadCloser
	openErr  error
	getByID  *model.SystemFile
	getByIDErr error
	deleteErr error
}

// serviceUploadInput 与 service.UploadInput 同形（stub 不 import service 避免环，
// 字段必须与 Task 4 合同一一对应）。
type serviceUploadInput struct {
	Name, BizType string
	Size          int64
	Public        bool
}

func (s *fileStub) Upload(_ any, in interface {
	// 占位不可编译——见 Step 3 注：stub 直接实现 service.FileService 接口，
	// 参数类型必须是 service.UploadInput（以真实接口为准）。
}) (*model.SystemFile, error) {
	return nil, nil
}
```

**上面是占位示意——不要照抄。** 实际 stub 直接实现 Task 4 的接口（import `service` 包，同包 `admin` 已 import service 无环）：

```go
type fileStub struct {
	upload      *model.SystemFile
	uploadErr   error
	gotIn       service.UploadInput
	getByKey    *model.SystemFile
	getByKeyErr error
	openRc      io.ReadCloser
	openErr     error
	getByID     *model.SystemFile
	getByIDErr  error
	deleteErr   error
}

func (s *fileStub) Upload(_ context.Context, in service.UploadInput) (*model.SystemFile, error) {
	s.gotIn = in
	return s.upload, s.uploadErr
}
func (s *fileStub) GetByID(context.Context, uint64) (*model.SystemFile, error) {
	return s.getByID, s.getByIDErr
}
func (s *fileStub) GetByKey(context.Context, string) (*model.SystemFile, error) {
	return s.getByKey, s.getByKeyErr
}
func (s *fileStub) Open(context.Context, *model.SystemFile) (io.ReadCloser, error) {
	return s.openRc, s.openErr
}
func (s *fileStub) Delete(context.Context, uint64) error { return s.deleteErr }

// newFileTestRouter 同时挂 admin 与 public 两组——照真实模块的相对前缀。
func newFileTestRouter(stub *fileStub) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	c := NewFileController(stub)
	c.RegisterAdminRoutes(r.Group("/api/admin/infra/file"))
	c.RegisterPublicRoutes(r.Group("/api/public/infra/file"))
	return r
}

func multipartReq(t *testing.T, path string, filename string, content []byte, form map[string]string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatal(err)
	}
	for k, v := range form {
		if err := w.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", path, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

// 上传信封形状：code=0，data 五字段；public=true → public_url 带 key；
// public=false → public_url 空串。multipart 表单域（public/biz_type）透传 service。
func TestUpload_EnvelopeShape(t *testing.T) {
	stub := &fileStub{upload: &model.SystemFile{
		ID: 7, Name: "a.jpg", Size: 4, Mime: "image/jpeg",
		Public: true, PublicKey: strings.Repeat("a", 32),
	}}
	r := newFileTestRouter(stub)
	req := multipartReq(t, "/api/admin/infra/file/upload", "a.jpg", []byte("x"), map[string]string{
		"public": "true", "biz_type": "notice",
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status=%d body=%s", w.Code, w.Body)
	}
	var env struct {
		Code int `json:"code"`
		Data struct {
			ID        uint64 `json:"id"`
			Name      string `json:"name"`
			Size      int64  `json:"size"`
			Mime      string `json:"mime"`
			PublicURL string `json:"public_url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Code != 0 || env.Data.ID != 7 || env.Data.PublicURL != "/api/public/infra/file/"+strings.Repeat("a", 32) {
		t.Errorf("envelope = %+v, want code=0 with public_url", env)
	}
	if !stub.gotIn.Public || stub.gotIn.BizType != "notice" || stub.gotIn.Name != "a.jpg" {
		t.Errorf("service input = %+v, want Public=true BizType=notice", stub.gotIn)
	}
	// 缺 file 字段 → 10002
	w2 := httptest.NewRecorder()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("public", "false")
	_ = mw.Close()
	req2 := httptest.NewRequest("POST", "/api/admin/infra/file/upload", &buf)
	req2.Header.Set("Content-Type", mw.FormDataContentType())
	r.ServeHTTP(w2, req2)
	if code, _ := codeOfBody(t, w2); code != 10002 {
		t.Errorf("missing file envelope code = %d, want 10002", code)
	}
	// service 30011 直传
	stubErr := &fileStub{uploadErr: errs.New(errs.CodeFileTypeNotAllowed, nil)}
	r2 := newFileTestRouter(stubErr)
	w3 := httptest.NewRecorder()
	r2.ServeHTTP(w3, multipartReq(t, "/api/admin/infra/file/upload", "a.svg", []byte("x"), nil))
	if code, _ := codeOfBody(t, w3); code != 30011 || w3.Code != 400 {
		t.Errorf("mime reject: http=%d code=%d, want 400/30011", w3.Code, code)
	}
}

// public 读：免鉴权（无 Authorization 头）、流式透传 + nosniff/inline/Cache-Control；
// 私有/不存在统一 30010。
func TestPublicGet_HeadersAndEnvelope(t *testing.T) {
	meta := &model.SystemFile{Name: "照片.jpg", Mime: "image/jpeg", PublicKey: strings.Repeat("b", 32), Public: true}
	stub := &fileStub{
		getByKey: meta,
		openRc:   io.NopCloser(bytes.NewReader([]byte{0xFF, 0xD8, 0xFF})),
	}
	r := newFileTestRouter(stub)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/public/infra/file/"+meta.PublicKey, nil))
	if w.Code != 200 {
		t.Fatalf("status=%d body=%s", w.Code, w.Body)
	}
	if got := w.Header().Get("Content-Type"); got != "image/jpeg" {
		t.Errorf("Content-Type = %q, want image/jpeg (存库探测值，不信扩展名)", got)
	}
	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("nosniff missing")
	}
	if cd := w.Header().Get("Content-Disposition"); !strings.HasPrefix(cd, "inline") {
		t.Errorf("Content-Disposition = %q, want inline", cd)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "public, max-age=3600" {
		t.Errorf("Cache-Control = %q", cc)
	}
	if w.Body.Len() != 3 {
		t.Errorf("body len = %d, want 3 (stream passthrough)", w.Body.Len())
	}

	// private/不存在 → 30010 信封（流不输出）
	stub2 := &fileStub{getByKeyErr: errs.New(errs.CodeFileNotFound, nil)}
	r2 := newFileTestRouter(stub2)
	w2 := httptest.NewRecorder()
	r2.ServeHTTP(w2, httptest.NewRequest("GET", "/api/public/infra/file/"+strings.Repeat("c", 32), nil))
	if code, _ := codeOfBody(t, w2); code != 30010 || w2.Code != 400 {
		t.Errorf("missing/private: http=%d code=%d, want 400/30010", w2.Code, code)
	}
}

// 受控下载：attachment disposition + 流式字节；DELETE 动作信封 data:null。
func TestAdminDownloadAndDelete(t *testing.T) {
	meta := &model.SystemFile{Name: "报告.pdf", Mime: "application/pdf", RelPath: "x"}
	stub := &fileStub{
		getByID: meta,
		openRc:  io.NopCloser(bytes.NewReader([]byte("%PDF-1.4"))),
	}
	r := newFileTestRouter(stub)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/admin/infra/file/5", nil))
	if w.Code != 200 {
		t.Fatalf("download status=%d body=%s", w.Code, w.Body)
	}
	if cd := w.Header().Get("Content-Disposition"); !strings.HasPrefix(cd, "attachment") {
		t.Errorf("Content-Disposition = %q, want attachment", cd)
	}
	if !strings.HasPrefix(w.Body.String(), "%PDF") {
		t.Errorf("body = %q, want pdf bytes", w.Body.String()[:min(4, w.Body.Len())])
	}

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest("DELETE", "/api/admin/infra/file/5", nil))
	if code, data := codeOfBody(t, w2); code != 0 || data != "null" {
		t.Errorf("delete envelope code=%d data=%s, want 0/null", code, data)
	}
}

// codeOfBody 解析信封 code 与 data 原文（controller 测试内联小工具）。
func codeOfBody(t *testing.T, w *httptest.ResponseRecorder) (int, string) {
	t.Helper()
	var env struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, w.Body.String())
	}
	return env.Code, string(env.Data)
}
```

（`min` 若 Go<1.21 无内建则手写 `func min(a, b int) int {...}`；codegen_controller_test 已用 `min`——同包若已有定义则复用、勿重复声明。）

中间件放行钉——`pkg/middleware/auth_test.go`：

1. `newRouter` 内追加注册：

```go
	r.GET("/api/public/infra/file/:key", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
```

2. `TestAuthMiddleware_NonAdminPaths_WithoutToken_200` 的 case 表追加一行：

```go
		{"GET", "/api/public/infra/file/abc123"},
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd /Users/alex/Desktop/goWork/alexGo-cloud && CGO_ENABLED=0 go test ./modules/system/infra/controller/... ./pkg/middleware/ -run 'TestUpload|TestPublic|TestAdminDownload|TestAuthMiddleware_NonAdmin' -v`
Expected: FAIL（NewFileController undefined；middleware 新 case 因路由未注册 404）

- [ ] **Step 3: 实现 controller 与装配**

```go
// modules/system/infra/controller/admin/file_controller.go
package admin

import (
	"errors"
	"io"
	"mime"
	"net/http"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/system/infra/service"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/resp"
	"alexGo-cloud/pkg/storage"
)

// FileController 统一文件端点：admin 受控面（JWT+权限码）+ public 免鉴权读。
// 免鉴权不免信封（spec §3）：JSON 分支全走 pkg/resp；流本体是唯一豁免。
type FileController struct {
	svc service.FileService
}

// NewFileController 构造文件控制器。
func NewFileController(svc service.FileService) *FileController {
	return &FileController{svc: svc}
}

// RegisterAdminRoutes 挂 /admin/infra/file（入参为 /api group 下的子组）。
func (c *FileController) RegisterAdminRoutes(g *gin.RouterGroup) {
	g.POST("/upload", c.upload)
	g.GET("/:id", c.downloadByID)
	g.DELETE("/:id", c.delete)
}

// RegisterPublicRoutes 挂 /public/infra/file——AuthMiddleware 只拦 /api/admin/**
// （pkg/middleware/auth.go），本组天然免登录，防枚举靠随机 public_key。
func (c *FileController) RegisterPublicRoutes(g *gin.RouterGroup) {
	g.GET("/:key", c.publicGet)
}

// upload multipart：字段 file + 表单域 public("true"/"false", 默认 false) + 可选 biz_type。
func (c *FileController) upload(ctx *gin.Context) {
	fh, err := ctx.FormFile("file")
	if err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamMissing, err))
		return
	}
	src, err := fh.Open()
	if err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	defer src.Close()

	meta, err := c.svc.Upload(ctx.Request.Context(), service.UploadInput{
		Name:    fh.Filename,
		Size:    fh.Size, // 仅预检；service 内按字节流限流兜底（Size 可伪造）
		Reader:  src,
		Public:  ctx.PostForm("public") == "true",
		BizType: ctx.PostForm("biz_type"),
	})
	if err != nil {
		resp.Fail(ctx, err) // service 已返 BizError；DB 裸错兜底 50000
		return
	}
	resp.OK(ctx, gin.H{
		"id":         meta.ID,
		"name":       meta.Name,
		"size":       meta.Size,
		"mime":       meta.Mime,
		"public_url": publicURL(meta),
	})
}

// downloadByID 受控下载（混合模型受控半边）：attachment 头 + 流式字节。
func (c *FileController) downloadByID(ctx *gin.Context) {
	id, ok := pathID(ctx)
	if !ok {
		return
	}
	f, err := c.svc.GetByID(ctx.Request.Context(), id)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	c.serve(ctx, f, "attachment")
}

// publicGet 按 public_key 读 Public=true 行；private/不存在统一 30010。
func (c *FileController) publicGet(ctx *gin.Context) {
	f, err := c.svc.GetByKey(ctx.Request.Context(), ctx.Param("key"))
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	c.serve(ctx, f, "inline")
}

// serve 流式输出（spec §5）：存库 mime、nosniff、RFC5987 文件名、缓存 1h。
func (c *FileController) serve(ctx *gin.Context, f *model.SystemFile, disposition string) {
	rc, err := c.svc.Open(ctx.Request.Context(), f)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	defer rc.Close()
	ctx.Header("Content-Type", f.Mime)
	ctx.Header("X-Content-Type-Options", "nosniff")
	ctx.Header("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": f.Name}))
	if disposition == "inline" {
		ctx.Header("Cache-Control", "public, max-age=3600")
	}
	ctx.Status(http.StatusOK)
	_, _ = io.Copy(ctx.Writer, rc) // 头已发，拷贝失败无法回信封——与 codegen download 同豁免
}

func (c *FileController) delete(ctx *gin.Context) {
	id, ok := pathID(ctx)
	if !ok {
		return
	}
	if err := c.svc.Delete(ctx.Request.Context(), id); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}

// pathID 解析 :id；非法 → 10001（信封），ok=false 表示已写响应。
func pathID(ctx *gin.Context) (uint64, bool) {
	var id uint64
	if err := ginShouldBindID(ctx, &id); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return 0, false
	}
	return id, true
}
```

实现注（执行者就地展开，两版等价）：
- `import "alexGo-cloud/modules/system/infra/model"`（`*model.SystemFile` 在 serve/downloadByID 用到）。
- `ginShouldBindID` 就地改为既有惯例——本仓库 controller 解析 id 的写法以 codegen_controller 为准（`strconv.ParseUint(ctx.Param("id"), 10, 64)` 包 `errs.New(errs.CodeParamInvalid, err)`）。直接写成：

```go
func pathID(ctx *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil || id == 0 {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return 0, false
	}
	return id, true
}
```

- `publicURL(f *model.SystemFile) string`：

```go
// publicURL public 行给对外链接；private 恒空串（前端不渲染死链）。
func publicURL(f *model.SystemFile) string {
	if !f.Public {
		return ""
	}
	return "/api/public/infra/file/" + f.PublicKey
}
```

- import 补 `"strconv"`、`"alexGo-cloud/modules/system/infra/model"`。

`modules/system/infra/module.go` 改四处：

```go
// FxModule 新增 provider（在 admin.NewCodegenController 之后）：
		newStorageDriver,
		repository.NewFileRepository,
		newFileService,
		admin.NewFileController,
```

```go
// newStorageDriver 本地盘驱动：构造即 Abs+MkdirAll。不做 Enabled 开关——
// 本地盘无外部依赖（spec §1，与 Redis/MQ 可选依赖不同），YAGNI。
func newStorageDriver(cfg *config.Config) (storage.Driver, error) {
	root := "./data/uploads"
	if cfg != nil && cfg.Storage.Root != "" {
		root = cfg.Storage.Root
	}
	return storage.NewLocalDriver(root)
}

// newFileService 组装文件服务（配置注入大小上限与白名单覆盖）。
func newFileService(
	repo repository.FileRepository,
	driver storage.Driver,
	cfg *config.Config,
) service.FileService {
	return service.NewFileService(repo, driver, cfg)
}
```

`infraModule` 与 `NewModule`/`RegisterRoutes`：

```go
type infraModule struct {
	codegenCtrl *admin.CodegenController
	fileCtrl    *admin.FileController
}

// NewModule 聚合控制器，产出 server.Module（挂进 group:"modules"）。
func NewModule(codegenCtrl *admin.CodegenController, fileCtrl *admin.FileController) server.Module {
	return &infraModule{codegenCtrl: codegenCtrl, fileCtrl: fileCtrl}
}

// RegisterRoutes 挂载 infra 路由。入参是 /api group（见 server/http.go），
// 故这里用相对前缀：admin 受控组 + public 免鉴权组。
func (m *infraModule) RegisterRoutes(r *gin.RouterGroup) {
	m.codegenCtrl.RegisterRoutes(r.Group("/admin/infra/codegen"))
	m.fileCtrl.RegisterAdminRoutes(r.Group("/admin/infra/file"))
	m.fileCtrl.RegisterPublicRoutes(r.Group("/public/infra/file"))
}
```

import 补 `"alexGo-cloud/pkg/storage"`。既有 `/admin/infra/codegen` 的 `g := ...` 行内写法按上面展开保持等价。

- [ ] **Step 4: 跑测试确认通过**

Run: `cd /Users/alex/Desktop/goWork/alexGo-cloud && CGO_ENABLED=0 go test ./modules/system/infra/... ./pkg/middleware/ -v && CGO_ENABLED=0 go build ./...`
Expected: PASS + 构建成功

- [ ] **Step 5: 提交**

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud
git add modules/system/infra/controller/admin/file_controller.go modules/system/infra/controller/admin/file_controller_test.go modules/system/infra/module.go pkg/middleware/auth_test.go
git commit -m "feat(infra): file controller + Fx 装配（admin 受控 + public 免鉴权流式读）"
```

---

### Task 6: infra:file 权限接线（permissionRoutes + seed 按钮）

**Files:**
- Modify: `modules/system/service/permission_routes.go`（map 加一行）
- Modify: `modules/system/service/seed.go`（`fileStorageButtons` 包变量 + 按钮补齐段插一段）
- Test: `modules/system/service/seed_file_buttons_test.go`

**Interfaces:**
- Consumes:
  - `permissionRoutes` map 与 `permPrefix`（同包既有）
  - seed 内既有 `hasPermPrefix(allMenus, prefix)`、`rootID`（`/system` 目录锚点）、`allMenus` append → 尾部 `SetRoleMenus` 全量绑 admin 的既有机制
- Produces:
  - `permissionRoutes["infra:file"] = {"/api/admin/infra/file", "/api/admin/infra/file/*"}`——路由→权限组映射（keyMatch2）。
  - 包变量 `fileStorageButtons`（三条 perm）——测试与 seed 共用同一数据源。

- [ ] **Step 1: 写失败测试**

```go
// modules/system/service/seed_file_buttons_test.go
package service

import "testing"

// 文件存储三按钮与 permissionRoutes "infra:file" 组同源（与 TestSeedButtonPerms_MatchRouteKeys
// 同一防线）：漂移 = 死权限 403（按钮在但路由组不在）或路由失保护。
func TestSeedFileButtons_MatchRouteKey(t *testing.T) {
	if len(fileStorageButtons) != 3 {
		t.Fatalf("len(fileStorageButtons) = %d, want 3 (upload/download/delete)", len(fileStorageButtons))
	}
	routes, ok := permissionRoutes["infra:file"]
	if !ok {
		t.Fatal(`permissionRoutes 缺 "infra:file" 组——三按钮全是死权限`)
	}
	if len(routes) != 2 || routes[0] != "/api/admin/infra/file" || routes[1] != "/api/admin/infra/file/*" {
		t.Errorf("permissionRoutes[infra:file] = %v, want 集合路径 + /* 变体", routes)
	}
	want := map[string]bool{
		"infra:file:upload":   true,
		"infra:file:download": true,
		"infra:file:delete":   true,
	}
	for _, b := range fileStorageButtons {
		if !want[b.perm] {
			t.Errorf("意外按钮 perm = %q", b.perm)
		}
		// permPrefix 尺度必须命中路由组键（与既有三组按钮同判据）
		if permPrefix(b.perm) != "infra:file" {
			t.Errorf("permPrefix(%q) = %q, want infra:file", b.perm, permPrefix(b.perm))
		}
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd /Users/alex/Desktop/goWork/alexGo-cloud && CGO_ENABLED=0 go test ./modules/system/service/ -run TestSeedFileButtons -v`
Expected: FAIL（fileStorageButtons undefined）

- [ ] **Step 3: 实现**

`modules/system/service/permission_routes.go`，`"infra:codegen"` 行后加：

```go
	"infra:file":   {"/api/admin/infra/file", "/api/admin/infra/file/*"},
```

`modules/system/service/seed.go`，包级（`hasPermPrefix` 函数附近）：

```go
// fileStorageButtons 文件存储三按钮（子项目 B / P1-1）：三条同属 infra:file 前缀。
// 必须整组绑一次 hasPermPrefix 判据——前缀粒度下逐条判会因组内首条 append 后
// 跳过余下两条（只插 1/3 = 死权限 403，Review Focus ④）。seed_test 钉与
// permissionRoutes["infra:file"] 同源。
var fileStorageButtons = []struct {
	perm string
	name string
	icon string
	sort int
}{
	{perm: "infra:file:upload", name: "文件上传", icon: "upload", sort: 14},
	{perm: "infra:file:download", name: "文件下载", icon: "download", sort: 15},
	{perm: "infra:file:delete", name: "文件删除", icon: "trash-2", sort: 16},
}
```

seed 流程内：**三组按钮循环（`for _, b := range []struct{...}` 含 system:auth/tenant/member 三行）结束之后、`menuIDs := make(...)` 构造之前**，插入：

```go
	// 文件存储按钮（子项目 B / P1-1）：三条同属 infra:file 前缀，绑一次幂等判——
	// 逐条判会因组内首条 append 后跳过余下两条。fresh 库与老库统一走本路径，
	// append 进 allMenus → 尾部 SetRoleMenus 全量绑 admin。
	if rootID != 0 && !hasPermPrefix(allMenus, "infra:file") {
		for _, b := range fileStorageButtons {
			btn := &model.Menu{
				ParentID:   rootID,
				Type:       "button",
				Name:       b.name,
				Path:       "",
				Component:  "",
				Icon:       b.icon,
				Permission: b.perm,
				Sort:       b.sort,
				Status:     1,
				TenantID:   tid,
				CreatedAt:  now,
				UpdatedAt:  now,
			}
			if err := p.Menus.Create(ctx, btn); err == nil {
				allMenus = append(allMenus, btn)
			}
		}
	}
```

- [ ] **Step 4: 跑测试确认通过（seed 全量回归）**

Run: `cd /Users/alex/Desktop/goWork/alexGo-cloud && CGO_ENABLED=0 go test ./modules/system/service/ -v`
Expected: PASS（TestSeed* / TestHas* 全绿）

- [ ] **Step 5: 提交**

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud
git add modules/system/service/permission_routes.go modules/system/service/seed.go modules/system/service/seed_file_buttons_test.go
git commit -m "feat(system): infra:file 权限接线（路由组 + seed 三按钮整组幂等）"
```

---

### Task 7: 公告附件字段（model + 迁移 + 往返测试）

**Files:**
- Modify: `modules/system/model/basic.go`（`SystemNotice` 加 `Attachments`）
- Create: `modules/system/migrations/20261016000002_notice_attachments.up.sql`
- Create: `modules/system/migrations/20261016000002_notice_attachments.down.sql`
- Test: `modules/system/service/notice_attach_test.go`

**Interfaces:**
- Consumes: 既有 `NoticeService`（`modules/system/service/basic.go`，controller `var req model.SystemNotice` 直接 bind——加字段即自动进出，无白名单过滤）。
- Produces:
  - `model.SystemNotice.Attachments string`——JSON 数组文本（元素 `{id,name,size,url}`，`url=/api/public/infra/file/<key>`）；**后端原样存取不解析**（spec §2）。
  - 前端 `Notice.attachments` 的后端对应物（Task 8 消费）。

- [ ] **Step 1: 写失败测试**

```go
// modules/system/service/notice_attach_test.go
package service

import (
	"context"
	"testing"

	"alexGo-cloud/modules/system/model"
	"alexGo-cloud/pkg/tenant"
)

// 附件字段往返：Create 原样存、List 原样取（spec §2：后端不解析 JSON 内容）。
func TestNoticeService_AttachmentsRoundTrip(t *testing.T) {
	svc, _ := setupNoticeService(t)
	ctx := tenant.WithTenantID(context.Background(), 1)
	att := `[{"id":7,"name":"照片.jpg","size":4,"url":"/api/public/infra/file/aaaa"}]`

	n, err := svc.Create(ctx, &model.SystemNotice{Title: "带附件", Content: "c", Attachments: att})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	list, err := svc.List(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("List: n=%d err=%v", len(list), err)
	}
	if list[0].Attachments != att {
		t.Errorf("Attachments = %q, want round-trip exact", list[0].Attachments)
	}
	// Update 覆盖也原样存
	n2 := *n
	n2.Attachments = `[{"id":8,"name":"b.pdf","size":9,"url":"/api/public/infra/file/bbbb"}]`
	if err := svc.Update(ctx, &n2); err != nil {
		t.Fatalf("Update: %v", err)
	}
	list, _ = svc.List(ctx)
	if list[0].Attachments != n2.Attachments {
		t.Errorf("Attachments after update = %q, want %q", list[0].Attachments, n2.Attachments)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd /Users/alex/Desktop/goWork/alexGo-cloud && CGO_ENABLED=0 go test ./modules/system/service/ -run TestNoticeService_Attachments -v`
Expected: FAIL（model.SystemNotice.Attachments undefined）

- [ ] **Step 3: 实现**

`modules/system/model/basic.go`，`SystemNotice` 结构体内 `UpdatedAt` 之前加：

```go
	// Attachments 附件 JSON 数组文本（元素 {id,name,size,url}，url=/api/public/infra/file/<key>）。
	// 照 DataScopeDeptIDs 的 string 惯例（spec §2）：后端原样存取不解析——
	// 公告内容本就是 admin 受信富文本。size 4096 防 MySQL 默认 varchar(191) 截断。
	Attachments string    `gorm:"column:attachments;size:4096" json:"attachments"`
```

```sql
-- modules/system/migrations/20261016000002_notice_attachments.up.sql
-- 公告附件位（子项目 B / P1-1 spec §2）：JSON 数组文本，后端原样存取不解析。
-- ALTER 非幂等——老环境走应用内 migrate（pkg/migrate 按版本记录），勿重复执行。
ALTER TABLE `system_notices` ADD COLUMN `attachments` varchar(4096) NOT NULL DEFAULT '';
```

```sql
-- modules/system/migrations/20261016000002_notice_attachments.down.sql
ALTER TABLE `system_notices` DROP COLUMN `attachments`;
```

- [ ] **Step 4: 跑测试确认通过**

Run: `cd /Users/alex/Desktop/goWork/alexGo-cloud && CGO_ENABLED=0 go test ./modules/system/service/ -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud
git add modules/system/model/basic.go modules/system/migrations/20261016000002_notice_attachments.up.sql modules/system/migrations/20261016000002_notice_attachments.down.sql modules/system/service/notice_attach_test.go
git commit -m "feat(system): 公告附件字段（Attachments + ALTER 迁移 + 原样往返测试）"
```

---

### Task 8: 前端上传 API + 公告附件表单/展示

**Files:**
- Modify: `apps/web-antd/src/api/infra.ts`（文件尾追加 File 段）
- Create: `apps/web-antd/src/utils/notice-attachments.ts`
- Create: `apps/web-antd/src/utils/__tests__/notice-attachments.test.ts`
- Modify: `apps/web-antd/src/api/admin.ts`（`Notice` 接口 + `createNotice` payload）
- Modify: `apps/web-antd/src/views/system/SystemNoticesPage.vue`（表单 Upload + 附件列）
- Modify: `apps/web-antd/src/views/system/SystemMyNoticesPage.vue`（详情渲染附件）

**Interfaces:**
- Consumes:
  - Task 5 路由 `POST /api/admin/infra/file/upload`（multipart 字段 `file` + 表单域 `public`/`biz_type` → 信封 data `{id,name,size,mime,public_url}`；`requestClient` 信封拦截器 code===0 已解包，拿到裸对象）
  - Task 7 `Notice.attachments`（string | 未定义兼容老数据）
  - vben form adapter：`Upload` 组件已注册（`adapter/component/index.ts` `withPreviewUpload`，modelPropName `fileList`；`customRequest` 经 attrs 透传）
- Produces:
  - `uploadFile(file: File, opts?: {bizType?: string; public?: boolean}): Promise<FileMeta>`
  - `parseAttachments(raw?: string | null): NoticeAttachment[]` / `stringifyAttachments(list: NoticeAttachment[]): string` / `attachmentsFromUploadList(list?): NoticeAttachment[]` / `uploadNoticeAttachment(file: File): Promise<NoticeAttachment>`

- [ ] **Step 1: 写失败测试**

```ts
// apps/web-antd/src/utils/__tests__/notice-attachments.test.ts
import type { NoticeAttachment } from '#/utils/notice-attachments';

import {
  attachmentsFromUploadList,
  parseAttachments,
  stringifyAttachments,
  uploadNoticeAttachment,
} from '#/utils/notice-attachments';
import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('#/api/infra', () => ({
  uploadFile: vi.fn(),
}));

import { uploadFile } from '#/api/infra';

describe('notice-attachments', () => {
  beforeEach(() => {
    vi.mocked(uploadFile).mockReset();
  });

  it('parseAttachments 解析合法 JSON 数组', () => {
    const raw = JSON.stringify([
      { id: 1, name: 'a.jpg', size: 4, url: '/api/public/infra/file/k1' },
    ]);
    expect(parseAttachments(raw)).toHaveLength(1);
    expect(parseAttachments(raw)[0]?.name).toBe('a.jpg');
  });

  it('parseAttachments 对空串/非法 JSON/非数组返回 []（老数据无附件不炸列表）', () => {
    expect(parseAttachments(undefined)).toEqual([]);
    expect(parseAttachments(null)).toEqual([]);
    expect(parseAttachments('')).toEqual([]);
    expect(parseAttachments('not-json')).toEqual([]);
    expect(parseAttachments('{"a":1}')).toEqual([]);
    expect(parseAttachments('[1,2]')).toEqual([]); // 元素非对象也不炸
  });

  it('stringifyAttachments → parseAttachments 往返恒等', () => {
    const list: NoticeAttachment[] = [
      { id: 2, name: 'b.pdf', size: 9, url: '/api/public/infra/file/k2' },
    ];
    expect(parseAttachments(stringifyAttachments(list))).toEqual(list);
    expect(stringifyAttachments([])).toBe('[]');
  });

  it('attachmentsFromUploadList 只收 done 且有 response 的项', () => {
    const list = [
      { status: 'done', response: { id: 1, name: 'a.jpg', size: 4, public_url: '/api/public/infra/file/k1' } },
      { status: 'uploading' },
      { status: 'done' }, // 无 response
      { status: 'error' },
    ];
    expect(attachmentsFromUploadList(list as any)).toEqual([
      { id: 1, name: 'a.jpg', size: 4, url: '/api/public/infra/file/k1' },
    ]);
    expect(attachmentsFromUploadList(undefined)).toEqual([]);
  });

  // Review Focus ⑤：公告附件必须恒 public:true——private 行的 public_url
  // 在 /api/public 下恒 404（30010），公告一发附件即死。
  it('uploadNoticeAttachment 以 public:true + bizType:notice 上传并映射 url', async () => {
    vi.mocked(uploadFile).mockResolvedValue({
      id: 7,
      name: 'c.jpg',
      size: 5,
      mime: 'image/jpeg',
      public_url: '/api/public/infra/file/kk7',
    });
    const file = new File(['x'], 'c.jpg', { type: 'image/jpeg' });
    const att = await uploadNoticeAttachment(file);
    expect(uploadFile).toHaveBeenCalledWith(file, {
      bizType: 'notice',
      public: true,
    });
    expect(att).toEqual({
      id: 7,
      name: 'c.jpg',
      size: 5,
      url: '/api/public/infra/file/kk7',
    });
  });
});
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd /Users/alex/Desktop/goWork/alexGo-cloud/apps/web-antd && pnpm vitest run src/utils/__tests__/notice-attachments.test.ts`
Expected: FAIL（`#/utils/notice-attachments` 模块不存在）

- [ ] **Step 3: 实现 API 与工具**

`apps/web-antd/src/api/infra.ts` 文件尾追加：

```ts
// ---- 文件存储（infra file，/admin/infra/file/*；子项目 B / P1-1）----
// 上传走 requestClient——信封拦截器 code===0 解包后即本裸对象。
// public 行带 public_url（/api/public/infra/file/<key>，免鉴权随机键）；
// private 行 public_url 恒空串。
export interface FileMeta {
  id: number;
  name: string;
  size: number;
  mime: string;
  public_url: string;
}

/**
 * 统一上传（multipart：字段 file + 表单域 public/biz_type）。
 * 管理端受控下载用既有 requestClient.download('/admin/infra/file/' + id)；
 * public URL 无需 API 函数，<a href>/<img src> 直连。
 */
export function uploadFile(
  file: File,
  opts?: { bizType?: string; public?: boolean },
) {
  const fd = new FormData();
  fd.append('file', file);
  fd.append('public', String(opts?.public ?? false));
  if (opts?.bizType) {
    fd.append('biz_type', opts.bizType);
  }
  return requestClient.post<FileMeta>('/admin/infra/file/upload', fd);
}
```

`apps/web-antd/src/utils/notice-attachments.ts` 新建：

```ts
import { uploadFile } from '#/api/infra';

/** 公告附件元素（后端原样存取的 JSON 数组元素，spec §2）。 */
export interface NoticeAttachment {
  id: number;
  name: string;
  size: number;
  url: string; // /api/public/infra/file/<key>
}

/** antd UploadFile 的最小鸭子形状（customRequest onSuccess 后 response=FileMeta）。 */
interface UploadItemLike {
  response?: { id?: number; name?: string; size?: number; public_url?: string };
  status?: string;
}

/** 解析附件 JSON 文本；空/非法/非数组一律 []（老公告无此字段不炸列表）。 */
export function parseAttachments(raw?: null | string): NoticeAttachment[] {
  if (!raw) return [];
  try {
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    return parsed.filter(
      (x): x is NoticeAttachment =>
        !!x &&
        typeof x === 'object' &&
        typeof x.id === 'number' &&
        typeof x.name === 'string' &&
        typeof x.url === 'string',
    );
  } catch {
    return [];
  }
}

/** 序列化为后端原样存储的 JSON 文本。 */
export function stringifyAttachments(list: NoticeAttachment[]): string {
  return JSON.stringify(list ?? []);
}

/** Upload fileList → 附件数组：只收 done 且携带 response（= 上传成功回包）的项。 */
export function attachmentsFromUploadList(
  list?: null | UploadItemLike[],
): NoticeAttachment[] {
  if (!Array.isArray(list)) return [];
  const out: NoticeAttachment[] = [];
  for (const f of list) {
    const r = f?.response;
    if (f?.status !== 'done' || !r?.public_url) continue;
    out.push({
      id: r.id ?? 0,
      name: r.name ?? '',
      size: r.size ?? 0,
      url: r.public_url,
    });
  }
  return out;
}

/**
 * 公告附件上传：恒 public:true + bizType:notice（Review Focus ⑤）——
 * private 行的 URL 在 /api/public 下恒 404，公告附件必须可跨端直读。
 */
export async function uploadNoticeAttachment(file: File): Promise<NoticeAttachment> {
  const meta = await uploadFile(file, { bizType: 'notice', public: true });
  return { id: meta.id, name: meta.name, size: meta.size, url: meta.public_url };
}
```

`apps/web-antd/src/api/admin.ts` 两处改：

```ts
export interface Notice {
  attachments: string; // 附件 JSON 数组文本（可为 ''——老数据无附件）
  content: string;
  created_at: string;
  id: number;
  status: number;
  title: string;
}
```

```ts
export function createNotice(payload: {
  attachments: string;
  content: string;
  status: number;
  title: string;
}) {
  return requestClient.post<Notice>('/admin/system/notices', payload);
}
```

- [ ] **Step 4: 跑工具层测试确认通过**

Run: `cd /Users/alex/Desktop/goWork/alexGo-cloud/apps/web-antd && pnpm vitest run src/utils/__tests__/notice-attachments.test.ts`
Expected: PASS（5 个用例全绿）

- [ ] **Step 5: 接页面（表单 Upload + 两处展示）**

`SystemNoticesPage.vue`：

1. script 顶部 import：

```ts
import {
  attachmentsFromUploadList,
  parseAttachments,
  stringifyAttachments,
  uploadNoticeAttachment,
} from '#/utils/notice-attachments';
```

2. `gridOptions.columns` 在 `content` 列（若有）或 `status` 列后加：

```ts
    { field: 'attachments', slots: { default: 'attachments' }, title: '附件', width: 180 },
```

（列定义以文件真实字段为准，插在操作列之前。）

3. `useVbenForm` 的 `handleSubmit`：

```ts
  handleSubmit: async (values) => {
    await createNotice({
      ...(values as { content: string; status: number; title: string }),
      // fileList（Upload modelValue）→ 已上传成功项 → 后端原样存的 JSON 文本
      attachments: stringifyAttachments(
        attachmentsFromUploadList(
          (values as { attachments?: unknown }).attachments as Parameters<
            typeof attachmentsFromUploadList
          >[0],
        ),
      ),
    });
    message.success('创建成功');
    modalApi.close();
    gridApi.reload();
  },
```

4. `schema` 数组 `content` 项之后插入 Upload schema：

```ts
    {
      component: 'Upload',
      componentProps: {
        // customRequest 接管上传（antd 有 customRequest 则不发默认请求）：
        // 每个文件走统一上传接口，onSuccess(response) 会让 antd 把 response
        // 挂到 file.response（attachmentsFromUploadList 消费）。
        customRequest: async ({
          file,
          onError,
          onSuccess,
        }: {
          file: File;
          onError: (e: Error) => void;
          onSuccess: (data: unknown) => void;
        }) => {
          try {
            onSuccess(await uploadNoticeAttachment(file));
          } catch (e) {
            onError(e instanceof Error ? e : new Error(String(e)));
          }
        },
        fileList: [],
        maxCount: 5,
        multiple: true,
      },
      fieldName: 'attachments',
      label: '附件',
    },
```

5. 模板 `<Grid>` 内、`#actions` 之前加附件列 slot：

```vue
      <template #attachments="{ row }">
        <a
          v-for="a in parseAttachments(row.attachments)"
          :key="a.id"
          class="mr-2"
          :href="a.url"
          target="_blank"
        >
          {{ a.name }}
        </a>
      </template>
```

`SystemMyNoticesPage.vue`（详情 Modal 渲染，`current?.content` 的 div 之后）：

```vue
      <div
        v-for="a in parseAttachments(current?.attachments)"
        :key="a.id"
        class="mt-1"
      >
        <a :href="a.url" target="_blank">附件：{{ a.name }}</a>
      </div>
```

script 补 `import { parseAttachments } from '#/utils/notice-attachments';`，`NoticeItem` 类型来自 admin.ts 已含 `attachments`（继承 Notice）。`current` 的类型若有局部声明，确认含 `attachments?: string`。

- [ ] **Step 6: typecheck + 全量 vitest**

Run: `cd /Users/alex/Desktop/goWork/alexGo-cloud/apps/web-antd && pnpm typecheck && pnpm test`
Expected: vue-tsc 0 error；vitest 全绿（新 5 用例 + 既有）

- [ ] **Step 7: 提交**

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud
git add apps/web-antd/src/api/infra.ts apps/web-antd/src/api/admin.ts apps/web-antd/src/utils/notice-attachments.ts apps/web-antd/src/utils/__tests__/notice-attachments.test.ts apps/web-antd/src/views/system/SystemNoticesPage.vue apps/web-antd/src/views/system/SystemMyNoticesPage.vue
git commit -m "feat(web): 文件上传 API + 公告附件表单与展示（恒 public 上传）"
```

---

### Task 9: 全量门禁 + full.sql 重生成 + roadmap 收口

**Files:**
- Regen: `docs/sql/alexgo-cloud-full.sql`（含此前未跟踪的 `scripts/gen-full-sql.sh` 与 `docs/sql/` 一并入库）
- Modify: `docs/roadmap.md`（P1-1 状态；**单独 docs 提交**）

**Interfaces:**
- Consumes: Task 1-8 全部产物。
- Produces: 可验收的门禁记录 + roadmap 状态；无新代码接口。

- [ ] **Step 1: Go 全量门禁**

Run: `cd /Users/alex/Desktop/goWork/alexGo-cloud && CGO_ENABLED=0 go build ./... && CGO_ENABLED=0 go vet ./... && CGO_ENABLED=0 go test ./modules/... ./pkg/...`
Expected: build/vet 0 错误；test 全绿（若有既有 flaky 先确认与本子项目无关再记录）

- [ ] **Step 2: 前端全量门禁**

Run: `cd /Users/alex/Desktop/goWork/alexGo-cloud/apps/web-antd && pnpm typecheck && pnpm test`
Expected: vue-tsc 0 error；vitest 全绿

- [ ] **Step 3: 迁移完整性（可选，需 MySQL 可达）**

Run: `cd /Users/alex/Desktop/goWork/alexGo-cloud && CGO_ENABLED=0 go test ./modules/system/infra/ -tags integration -run TestMigrate -v -count=1`
Expected: PASS（`migrate_test.go` 跑 `file://modules/system/infra/migrations`，会执行 `20261016000001_system_files`）
不可达则记录留票（既有惯例：MySQL 缺环境 → 冒烟票），不阻塞提交。

- [ ] **Step 4: 重生成全量 SQL 并提交代码侧**

Run: `cd /Users/alex/Desktop/goWork/alexGo-cloud && ./scripts/gen-full-sql.sh`
Expected: `generated: docs/sql/alexgo-cloud-full.sql (N migrations)`，N 比改前 +2（system_files + notice_attachments）

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud
git add scripts/gen-full-sql.sh docs/sql/
git commit -m "chore(sql): 全量建库脚本入库并重生成（system_files + 公告附件列）"
```

（脚本与 docs/sql 此前未跟踪——full.sql 是迁移的下游产物，不入库则新环境建库缺表。）

- [ ] **Step 5: roadmap 收口（docs 单独提交）**

`docs/roadmap.md` 状态总览 `| P1 | 常用管理能力 | 1/5 |` 改为 `2/5`；P1-1 行「依赖」列改为状态呈现，与 P1-5 行样式一致：

```markdown
| P1-1 | **文件存储** | `pkg/storage`：本地盘驱动 + 统一上传 API（`/api/admin/infra/file`）。解锁：公告附件、头像、codegen 导入产物、Excel 导出落地 | ✅ |
```

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud
git add docs/roadmap.md
git commit -m "docs(roadmap): P1-1 文件存储收口（pkg/storage + infra 上传 API + 公告附件闭环）"
```

- [ ] **Step 6: 冒烟清单（需 MySQL + 浏览器，缺环境则留票不阻塞）**

手工验证项（与 spec §9 对应）：
1. 起服务（`CGO_ENABLED=0 go build ./alexgo-server/cmd` 后运行，本机 cgo 链接坏）→ 管理端登录。
2. 新建公告 → 上传 jpg（附件出现于列表）→ 提交成功。
3. 公告列表附件链接可点；「我的通知」详情渲染附件链接。
4. 浏览器直开 `/api/public/infra/file/<key>`：图片内联显示，响应头含 `nosniff` 与 `inline`。
5. 直开一个 `.svg`/`.html` 上传尝试 → 30011 拒绝。
6. `curl -X POST /api/admin/infra/file/upload`（无 token）→ 401；带 token 无权限码账号 → 403。

留票格式（若环境不可达）：在任务完成汇报中列明「冒烟 6 项未手验：MySQL/浏览器缺环境」，照 A 子项目遗留票惯例。

---

## 自审记录（writing-plans Self-Review）

1. **Spec 覆盖**：§1 分层（T1/T4）、§2 数据模型+附件（T3/T7）、§3 API 面 4 端点+三权限码（T5/T6）、§4 配置（T2）、§5 数据流（T4/T5）、§6 安全五条（T1 穿越/O_EXCL、T4 双 MIME+rand、T5 nosniff、全局限流沿用既有）、§7 错误码与补偿（T2/T4）、§8 前端（T8）、§9 测试验收（各任务 + T9 门禁/冒烟）、非目标未越界（无 GC/无管理页/无 S3 实现——Driver 留缝）。
2. **占位扫描**：T5 Step 1 首段 stub 是**明确标注的反例示意**（「上面是占位示意——不要照抄」后给完整可编译版）；T2 Step 1 给出两版等价可跑代码并注明以真实导出为准；其余无 TBD/TODO/「similar to」。
3. **类型一致性**：`UploadInput{Name,Size,Reader,Public,BizType}`（T4 定义 = T5 填充 = T5 stub 捕获）；`FileService` 五方法签名 T4→T5 stub 一致；`FileRepository` 四方法 T3→T4 一致；`fileStorageButtons` 结构 T6 定义=测试断言；`NoticeAttachment{id,name,size,url}` T8 工具=测试=页面渲染一致；`FileMeta{public_url}` T8 API=工具=customRequest 一致。
4. **Review Focus**：五条各自有归属任务与具体测试（①T4 TestUpload_GetByKey_TenantBoundary + T3 仓库侧、②T4 TestUpload_ByteStreamLimited_DeclaredSizeLies、③T5 auth_test 放行 case + TestPublicGet、④T6 TestSeedFileButtons + 整组判据、⑤T8 uploadNoticeAttachment vi.mock 断言）。
