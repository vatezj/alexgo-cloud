# Swagger API 文档规范（swaggo，每模块独立 spec）

alexGo-cloud 后端 API 文档基于 swaggo 生成。**每个模块一份独立 spec**，挂载在
`/swagger/<模块>/index.html`；模块内部再按 **后台管理 / App 端** 两侧分 tag。

- 入口：`GET /swagger`（302 跳到首个模块 UI）；各模块 UI 在 `/swagger/<模块>/index.html`
- 开关：`server.swagger_enabled`（默认 `true`；对外生产环境显式置 `false` 关掉，避免接口形状外泄）
- 生成：`make swagger`（逐模块 `swag init` → `scripts/swagger_inject_taggroups.py` 逐目录注入 `x-tagGroups`）
- 产物：`docs/swagger/<模块>/<模块>_{docs.go,swagger.json,swagger.yaml}`，**入库提交**（运行时读 docs.go 编译进二进制的 docTemplate，改完必须重新生成并提交）

## 一、目录与实例结构

```
modules/<模块>/swagger_doc.go        # swag 入口：@title/@BasePath/@securityDefinitions（每模块一份）
docs/swagger/<模块>/<模块>_docs.go   # 生成（swag.Register("<模块>", ...) 实例注册）
docs/swagger/<模块>/<模块>_swagger.json
docs/swagger/<模块>/<模块>_swagger.yaml
```

- `make swagger` 对 `SWAG_MODULES`（Makefile 顶部，当前 fishgame system scorecard member order）
  逐个执行 `swag init -g swagger_doc.go --dir modules/<模块> -o docs/swagger/<模块> --instanceName <模块> --parseDepth 3`
- `--dir` 限定只扫本模块目录 → 各 spec 互不串味（不会把别的模块的路由扫进来）
- `--instanceName` 让生成的 docs.go 以模块名注册（`swag.Register("fishgame", ...)`），
  http.go 里 `ginSwagger.InstanceName(模块名)` 各挂各的
- 新增模块时：建 `modules/<模块>/swagger_doc.go`（照抄现有模块改名）→ Makefile `SWAG_MODULES`
  和 http.go 的挂载清单各加一项 → `make swagger`

## 二、tag 命名规则（核心约定）

```
<模块名>-<侧>     侧 ∈ {admin, app}
```

- admin 侧（后台管理，走 `/api/admin/**`，pkg/resp 信封）→ tag 后缀 `-admin`，如 `fishgame-admin`、`system-admin`
- app 侧（客户端契约，`/api/v1/**`、`/api/app/**`、`/api/public/**`，裸 JSON）→ tag 后缀 `-app`，如 `fishgame-app`

`scripts/swagger_inject_taggroups.py` 在每次 `make swagger` 后遍历 `docs/swagger/*/`，
按 tag 后缀把各模块侧栏分成「后台管理」/「App 端」两组。
**只要 tag 遵循后缀命名，新模块无需改任何脚本代码。**

### 侧栏分组是怎么来的

Swagger UI 只认 spec 顶层的 `x-tagGroups` 扩展字段。swag 不会生成它，
所以生成后由注入脚本补写到 **swagger.json 和 docs.go 两处**（运行时走 docs.go 的 docTemplate，
只改 json 不生效——脚本会同时改，勿手动只改一个）。生成的 const 名带实例后缀
（如 `docTemplatefishgame`），脚本用 `docTemplate\w*` 正则匹配。

## 三、通用注解（每模块 swagger_doc.go，已就位）

各模块根目录的 `swagger_doc.go` 定义该模块 spec 的全局项：

- `@BasePath /api` —— `@Router` 路径**相对**于 `/api` 写
- `@securityDefinitions.apikey Bearer` —— admin 侧，`Authorization: Bearer <token>`
- `@securityDefinitions.apikey X-Session` —— app 侧，`X-Session: <session>` 自鉴权头

两侧响应约定不同（写进 `@Description` 让读文档的人一眼看到）：

| 侧 | 响应形状 |
|---|---|
| admin | 统一 pkg/resp 信封 `{code, msg, data}` |
| app | 裸 JSON，无信封：成功直出 payload / 失败 `{"error":"..."}` |

## 四、注解模板

### admin 侧模板

```go
// Xxx godoc
//
//	@Summary		一句话动作
//	@Description	权限 <module>:<code>；<校验/边界说明>；统一 pkg/resp 信封。
//	@Tags			<module>-admin
//	@Produce		json
//	@Security		Bearer
//	@Param			page	query	integer	false	"页码"	Example(1)
//	@Param			body	body	service.XxxInput	true	"入参"
//	@Success		200		{object}	map[string]interface{}
//	@Router			/admin/<module>/<资源> [get]
func (c *XxxController) Xxx(ctx *gin.Context) { ... }
```

### app 侧模板

```go
// Xxx godoc
//
//	@Summary		一句话动作
//	@Description	body {...} → {...}；失败 401 {"error":"..."}。裸 JSON 无信封。
//	@Tags			<module>-app
//	@Accept			json
//	@Produce		json
//	@Security		X-Session          // 自鉴权入口（登录）与公开端点省略此行
//	@Param			body	body	object	true	"入参说明"	Example({"code": "..."})
//	@Success		200		{object}	map[string]string
//	@Failure		401		{object}	map[string]string
//	@Router			/<app 组路径>/<资源> [post]
func (c *XxxController) Xxx(ctx *gin.Context) { ... }
```

## 五、swag 解析坑（踩过的，勿再踩）

1. **`@Success` 行末尾禁止挂示例 JSON** —— swag 会把示例和类型粘成一个 token，
   报 `invalid type: any/object/interface{} {`。形状写进 `@Description`，或用 `Example` 放 `@Param` 行。
2. **类型写法**：`any` 不认、裸 `object` 当 success 类型不认 → 用 `map[string]interface{}`（admin 信封）
   或 `map[string]string`（app 裸错误体）；结构体直接写 `service.XxxInput` / `service.XxxOut`（会进 definitions）。
3. **`@Param` 行的 `Example({...})` 是合法的**，JSON 示例放这里没问题。
4. **`@Router` 不写 `/api` 前缀**（BasePath 已带）。
5. **`swag init` 的 `-g` 是相对 `--dir` 的路径**（`-g swagger_doc.go --dir modules/<模块>`）。
6. 生成扫描目录被 `--dir` 限定在本模块 → 引用**其他模块**的类型不会被解析，typed ref 只用本模块内的 struct。
7. 生成后若报 "no Go files in <repo root>" 是 swag 扫根目录的无害提示，忽略。

## 六、验证清单（每个模块注解完跑一遍）

```bash
make swagger                                  # 无 ParseComment 报错；各模块 json 的 x-tagGroups 正确
go build ./... && go test ./...               # 全绿（含 TestSwagger_* 实例隔离测试）
curl -sf localhost:8080/swagger/<模块>/index.html   # 200（swagger_enabled=true 时）
curl -sf localhost:8080/swagger/<模块>/doc.json     # 只含本模块 tag
```

## 七、铺开状态

| 模块 | 端点 | 状态 |
|---|---|---|
| fishgame（app 11 + admin 13） | 24 | ✅ |
| system（admin 53 + app 3） | 56 | ✅ |
| scorecard（admin 5 + app 19） | 24 | ✅ |
| member（app 6 + admin 3） | 9 | ✅ |
| order（admin 3） | 3 | ✅ |
