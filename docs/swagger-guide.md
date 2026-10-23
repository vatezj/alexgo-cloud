# Swagger API 文档规范（swaggo）

alexGo-cloud 后端 API 文档基于 swaggo 生成，UI 按 **后台管理 / App 端** 两大侧分组，每侧内部再按**模块**分 tag。

- 入口：`GET /swagger/index.html`（与 `/health` 同级，不在 `/api` 下）
- 开关：`server.swagger_enabled`（默认 `true`；对外生产环境显式置 `false` 关闭，避免接口形状外泄）
- 生成：`make swagger`（= `swag init` → `scripts/swagger_inject_taggroups.py` 注入 `x-tagGroups`）
- 产物：`docs/swagger/{docs.go,swagger.json,swagger.yaml}`，**入库提交**（运行时读 `docs.go` 编译进二进制的 docTemplate，改完必须重新生成并提交）

## 一、tag 命名规则（核心约定）

```
<模块名>-<侧>     侧 ∈ {admin, app}
```

- admin 侧（后台管理，走 `/api/admin/**`，pkg/resp 信封）→ tag 后缀 `-admin`，如 `fishgame-admin`、`system-admin`
- app 侧（客户端契约，走 `/api/v1/**` 或 `/api/public/**`，裸 JSON）→ tag 后缀 `-app`，如 `fishgame-app`

`scripts/swagger_inject_taggroups.py` 在每次 `make swagger` 后自动收集所有 operation 的 tag，
按后缀归入侧栏两组：`后台管理`（所有 `*-admin`）/ `App 端`（所有 `*-app`）。
**只要 tag 遵循后缀命名，新模块无需改任何脚本代码。**

### 侧栏分组是怎么来的

Swagger UI 只认 spec 顶层的 `x-tagGroups` 扩展字段。swag 不会生成它，
所以生成后由注入脚本补写到 **swagger.json 和 docs.go 两处**（运行时走 docs.go 的 docTemplate，
只改 json 不生效——脚本会同时改，勿手动只改一个）。

## 二、通用注解（已就位，勿动）

`alexgo-server/cmd/main.go` 顶部 package 注释定义了全局项：

- `@BasePath /api` —— `@Router` 路径**相对**于 `/api` 写
- `@securityDefinitions.apikey Bearer` —— admin 侧，`Authorization: Bearer <token>`
- `@securityDefinitions.apikey X-Session` —— app 侧，`X-Session: <session>` 自鉴权头

两侧响应约定不同（写进 `@Description` 让读文档的人一眼看到）：

| 侧 | 响应形状 |
|---|---|
| admin | 统一 pkg/resp 信封 `{code, msg, data}` |
| app | 裸 JSON，无信封：成功直出 payload / 失败 `{"error":"..."}` |

## 三、注解模板

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
//	@Router			/v1/<资源> [post]
func (c *XxxController) Xxx(ctx *gin.Context) { ... }
```

## 四、swag 解析坑（踩过的，勿再踩）

1. **`@Success` 行末尾禁止挂示例 JSON** —— swag 会把示例和类型粘成一个 token，
   报 `invalid type: any/object/interface{} {`。形状写进 `@Description`，或用 `Example` 放 `@Param` 行。
2. **类型写法**：`any` 不认、裸 `object` 当 success 类型不认 → 用 `map[string]interface{}`（admin 信封）
   或 `map[string]string`（app 裸错误体）；结构体直接写 `service.XxxInput` / `service.XxxOut`（会进 definitions）。
3. **`@Param` 行的 `Example({...})` 是合法的**，JSON 示例放这里没问题。
4. **`@Router` 不写 `/api` 前缀**（BasePath 已带）。
5. 生成后若报 "no Go files in <repo root>" 是 swag 扫根目录的无害提示，忽略。

## 五、验证清单（每个模块注解完跑一遍）

```bash
make swagger                                  # 无 ParseComment 报错；x-tagGroups 含新 tag
go build ./... && go test ./...               # 全绿
curl -sf localhost:<port>/swagger/index.html  # 200（swagger_enabled=true 时）
```

## 六、铺开计划（先打样后铺开，打样已完成）

| 批次 | 模块 | 规模 | 状态 |
|---|---|---|---|
| 打样 | fishgame（app 11 + admin 13） | 24 ops | ✅ |
| 打样 | system 在线会话组（3） | 3 ops | ✅ |
| 待铺 | system 其余（auth/tenant/menu/dict/online-user/file…） | ~61 ops | 按本规范逐组补 |
| 待铺 | infra、member、oauth2、scorecard | 各模块路由数而定 | 按本规范逐模块补 |

铺开时按模块一提交（`docs(swagger): <module> 端点注解`），每提交过一遍第五节清单。
