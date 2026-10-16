# 文件存储（子项目 B / P1-1）设计

> 日期：2026-10-16
> 状态：已与用户逐节确认（架构方案甲 + 五节设计）
> 范围：`pkg/storage` 本地盘驱动、统一上传 API（infra 子域）、文件元数据表、公告附件验收闭环
> 不在范围：头像接入、codegen 产物落盘、Excel 落盘（各自后续小票）、S3/OSS 驱动、文件管理页

## 背景与目标

现状盘点（2026-10-16）：后端零文件上传设施——无 storage 包、无 multipart 处理、无静态
文件服务；`user.avatar`/`member_user.avatar` 只是 string 字段，`SystemNotice` 无附件位，
codegen ZIP 是内存渲染流式下载不落盘。前端 ant-design-vue Upload 可用但 views 零使用。

目标（roadmap P1-1）：交付 `pkg/storage`（本地盘驱动 + 接口留缝）与统一上传 API，
以「公告附件」一个真实消费方做验收闭环；解锁后续头像、codegen 产物、Excel 落盘的
依赖面（P1-2 Excel 依赖本子项目）。

## 决策记录（用户逐项确认）

| 决策点 | 选择 |
|---|---|
| 访问模型 | 混合：admin 受控面（JWT+权限码）+ public 免鉴权读（随机 key 防枚举） |
| 交付边界 | API + 公告附件一个接入当验收；头像/codegen/Excel 各立后续小票 |
| 端点归属 | 挂 infra 子域（`/api/admin/infra/file`，yudao 同构；pkg 层独立于模块边界供跨模块用） |
| 分层切法 | 方案甲：`pkg/storage` 纯 IO 驱动层（无 DB）+ infra 簇元数据业务层 |
| public URL 键 | 独立 `public_key`（32 hex），非自增 id（防枚举）、非 rel_path（不泄露布局，CDN/S3 迁移 URL 稳定） |

## 1. 架构与分层

```
调用方：file controller（一期）／codegen CLI、Excel（将来）
              │
              ▼
modules/system/infra/service/file_service.go
  · 元数据落库（谁/何时/多大/mime/租户/public）
  · 策略：随机文件名 + yyyy/mm 分桶、大小上限、mime 校验
              │
              ▼
pkg/storage（纯 IO，零 DB、零策略）
  · Driver 接口：Save(ctx, relPath, r io.Reader) / Open(ctx, relPath) / Delete(ctx, relPath)
  · LocalDriver：root 下按相对路径读写删；路径穿越防御
```

- `relPath` 由 **service 生成**（`yyyy/mm/<随机ID>.<ext>`，随机 ID = `crypto/rand`
  16 字节 hex），driver 只执行字节读写——将来 S3/OSS 驱动同样只实现三个方法。
- **不做 `Enabled` 开关**：本地盘无外部依赖（与 Redis/MQ 的可选依赖不同），YAGNI。
- 新增组件：

| 层 | 文件 |
|---|---|
| pkg | `pkg/storage/storage.go`（Driver 接口 + `ErrNotFound` 哨兵）、`local.go`、`local_test.go`；`pkg/config/config.go` 加 `Storage` 段 |
| infra 簇 | `modules/system/infra/model/file.go`、`repository/file_repo.go`、`service/file_service.go`、`controller/admin/file_controller.go` |
| 公告接入 | `SystemNotice` 加 `Attachments` 字段；notice 前端表单/列表附件位 |
| 前端 | `apps/web-antd/src/api/infra.ts` 加 `uploadFile`；notice form/list 接入 |

## 2. 数据模型

`system_files`（入既有 AutoMigrate 清单，tenant gormplugin 自动隔离查询）：

```go
type SystemFile struct {
    ID        uint64    `gorm:"primaryKey" json:"id"`
    Name      string    `json:"name"`        // 原始文件名（只进元数据/Disposition，不进盘路径）
    Ext       string    `json:"ext"`         // 小写扩展名，无点
    Mime      string    `json:"mime"`        // http.DetectContentType 探测值
    Size      int64     `json:"size"`
    RelPath   string    `gorm:"column:rel_path;uniqueIndex" json:"rel_path"`
    PublicKey string    `gorm:"column:public_key;size:32;uniqueIndex" json:"public_key"`
    Public    bool      `json:"public"`
    BizType   string    `gorm:"column:biz_type" json:"biz_type"` // notice/general，仅归类
    TenantID  uint64    `gorm:"column:tenant_id;index" json:"tenant_id"`
    CreatedAt time.Time `json:"created_at"`
}
func (SystemFile) TableName() string { return "system_files" }
```

- `public_key` **恒生成**（private 行也有 32 hex）：避开 unique 索引空串冲突；是否放行
  只看 `Public`。
- 租户隔离：写与 admin 读走 tenant 插件；**public 读是唯一例外**——按 key 直取不受
  租户过滤（公告附件跨端可见是产品语义）。

公告附件（`SystemNotice` 加列）：

- `Attachments string`——JSON 数组文本（照 `DataScopeDeptIDs` 的 string 惯例），元素
  `{id, name, size, url}`，`url=/api/public/infra/file/<key>`。
- 后端**原样存取不解析**（公告内容本就是 admin 受信富文本，YAGNI）。

## 3. API 面

统一信封（`pkg/resp`），前缀与权限码照 infra 子域惯例：

| 方法 | 路径 | 鉴权 | 权限码 | 说明 |
|---|---|---|---|---|
| POST | `/api/admin/infra/file/upload` | admin JWT | `infra:file:upload` | multipart 字段 `file` + 表单域 `public`（"true"/"false"，默认 false）+ 可选 `biz_type` → `{id,name,size,mime,public_url}` |
| GET | `/api/admin/infra/file/:id` | admin JWT | `infra:file:download` | 受控读（混合模型受控半边；一期无消费方，冒烟覆盖） |
| DELETE | `/api/admin/infra/file/:id` | admin JWT | `infra:file:delete` | 硬删行+盘；引用方自理，不级联 |
| GET | `/api/public/infra/file/:key` | **免鉴权**（挂 `/api/public` 组，不在 JWT 中间件链内） | — | 按 `public_key` 读 `Public=true` 行；private/不存在统一 30010 |

免鉴权不免信封：public 端点成功/失败同样走 `pkg/resp`（文件流本体是唯一例外，同
codegen download 的流式豁免）。`biz_type` 为归类元数据，后端不校验枚举。

三权限码入种子（`infra:file:upload/download/delete`），超管默认持有。

## 4. 配置

`pkg/config` 加段，`configs/config.yaml`：

```yaml
storage:
  root: ./data/uploads        # 盘根，启动 MkdirAll
  max_size_mb: 10             # 单文件上限
  # allowed_exts 省略时用内置白名单 jpg,jpeg,png,gif,webp,pdf,zip,xlsx（可覆盖）
```

## 5. 数据流

上传：FormFile → 大小校验（`max_size_mb`）→ 扩展名白名单 ∩ `DetectContentType`
（读文件头 512 字节，不一致即拒）→ service 生成 `rel_path` + `public_key` →
`driver.Save` → 元数据落库（TenantID 取自 ctx）→ 信封返回（public 行带 `public_url`）。

public 读：`public_key` 查库 → `driver.Open` → 流式输出，带 `Content-Type`（存库探测
值，不信任扩展名）、`X-Content-Type-Options: nosniff`、`Content-Disposition: inline`
（RFC5987 原始文件名）、`Cache-Control: public, max-age=3600`。

## 6. 安全

1. **路径穿越双保险**：用户输入永不进盘路径；LocalDriver 内 `filepath.Clean` + 强制
   前缀在 root 绝对路径内。
2. **双 MIME 校验** + 白名单刻意不含 svg/html（存储型 XSS 面）；响应 `nosniff`。
3. `public_key` 用 `crypto/rand` 16 字节 hex。
4. 复用全局入口限流（pkg/limiter）；AccessLog 自动覆盖新端点。
5. LocalDriver 写入用 `O_CREATE|O_EXCL`——service 恒生成新随机名，碰撞即报错不覆盖。

## 7. 错误码与错误处理

registry 新增（3xxxx → HTTP 400，沿用 `statusFor`）：

| 码 | 文案 | 场景 |
|---|---|---|
| 30010 | 文件不存在 | GetByKey/GetById 查无；public 读 private 行也归此（不泄露存在性） |
| 30011 | 文件类型不允许 | 扩展名不在白名单或探测 mime 与扩展名不符 |
| 30012 | 文件超过大小限制 | 超 `max_size_mb` |

分层照 a9311e6 后惯例：service 返 BizError（校验类带 cause 落 warn），controller 直传
`resp.Fail`。

| 场景 | 处理 |
|---|---|
| Save 成功但落库失败 | 尽力盘删补偿 + zap warn 孤儿路径 → 50000 |
| 删除：行删成、盘删败 | 不回滚不报错，zap warn 留 rel_path（GC 后续票）→ 200 |
| 其余 IO/DB 裸错 | `resp.Fail` 兜底 50000 + 原始错误仅进日志 |

## 8. 前端接入

- `api/infra.ts`：`uploadFile(file: File, opts?: {bizType?: string; public?: boolean})`——
  FormData 经 `requestClient`（信封拦截器解包后得元数据对象）；admin 受控下载用既有
  `requestClient.download`；public URL 直接 `<a href>`/`<img src>` 无需 API 函数。
- notice 表单：antd Upload（`customRequest` → `uploadFile(..., {public:true,
  bizType:'notice'})`）维护附件数组；提交时 stringify 进 `attachments`；列表/详情渲染
  附件链接。

## 9. 测试与验收

| 层 | 覆盖 |
|---|---|
| `pkg/storage/local_test.go` | `t.TempDir()`：Save/Open 字节往返、Delete 后 `ErrNotFound`、`../` 穿越被拒、`O_EXCL` 防覆盖 |
| `infra/service/file_service_test.go`（sqlite 内存） | 白名单/探测 mime 双拒 30011；超限 30012；成功落库字段齐全（分桶 rel_path、32hex public_key）；private 行 GetByKey 统一 30010；Delete 行+盘都删；落库失败补偿后盘无孤儿 |
| controller（httptest+stub） | multipart 信封形状、public 端点头（Disposition/nosniff）、401/403 走既有中间件测试模式 |
| 公告接入 | notice 带 attachments 往返；前端 vitest 附件数组 stringify |

门禁：`CGO_ENABLED=0 go build/vet/test ./modules/... ./pkg/...` + `pnpm typecheck` +
vitest 全绿。冒烟（需 MySQL，缺环境则留票）：上传 jpg → public URL 浏览器直开 →
公告带附件 → app 端公告列表见附件链接。

## 非目标（一期明确不做，记为局限）

- 文件垃圾回收（GC）：删除不级联、孤儿补偿靠 warn 日志人工/后续处理
- zip 炸弹/病毒内容扫描、租户级存储配额
- 文件管理列表页
- S3/OSS/MinIO 驱动（Driver 接口已留缝）
- 断点续传/分片上传
- 头像、codegen 产物落盘、Excel 落盘接入（各自后续小票；P1-2 Excel 依赖本子项目落地）
