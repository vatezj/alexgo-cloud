# OAuth2 统一登录——兼容层下线评估

- 日期：2026-10-11
- 关联：spec `docs/superpowers/specs/2026-10-11-unified-login-design.md` §4.4（老端点降级为兼容层）、§9 D11；实施计划 Task 12（切换统一登录）、Task 14（前端 SSO）、Task 17（本文档 + 死代码清理）
- 结论先行：**服务端兼容层保留**（仓库外消费者不可证迁移）；**前端死代码 `loginApi` 本任务删除**。

## 1. 现状：兼容端点覆盖表

Task 12 已把三端老登录端点内部切到统一登录（`UnifiedLogin.Login`），响应契约逐字不变；
「下线」指整体删除以下端点注册。

| 老端点 | 注册点 | 现状（Task 12 之后） |
|---|---|---|
| `POST /api/auth/login` | system `controller/vben/controller.go:42`（挂 /api 根组） | 内部走统一登录（account_type=system、client=alexgo-admin），vben 契约不变；同组兄弟 `/auth/logout`、`/auth/codes` |
| `POST /api/app/system/auth/login` | system `module.go:218`（appGroup `/app/system`） | 同上（D11 勘误后的实际路径）；兄弟 `/auth/refresh`、`/auth/logout` |
| `POST /api/app/member/auth/login` | member `module.go:56`（appGroup `/app/member`） | 走统一登录（account_type=member、client=alexgo-app）；兄弟 `register/refresh/logout`（55-58 行） |

## 2. 消费方盘点（仓库内证据）

```bash
$ grep -rn "loginApi" apps/web-antd/src        # Task 14 之后、本任务删除之前
apps/web-antd/src/api/core/auth.ts:24:export async function loginApi(...)
$ grep -rn '"/auth/login"' modules/ --include='*.go'
modules/member/module.go:56:	appGroup.POST("/auth/login", m.authCtrl.Login)
modules/system/module.go:218:	appGroup.POST("/auth/login", m.appCtrl.Login)
modules/system/controller/vben/controller.go:42:	r.POST("/auth/login", ctrl.Login)
```

- 仓库内唯一前端登录入口 web-antd：Task 14 后走 `POST /api/oauth2/authorize/login` + `/callback`（PKCE），`loginApi` 零调用 → 死代码（本任务删除）。
- **仓库外消费者不可见**：原生 app 端、第三方 H5/小程序仍打 `POST /api/app/*/auth/login` 等端点——代码库内无法证明其已迁移到 `POST /api/oauth2/token`。

## 3. 下线条件（全部满足才动服务端）

- [ ] 仓库内零调用（本任务达成：`grep -rn "loginApi" apps/web-antd/src` 无输出）
- [ ] app 端发版确认改用 `POST /api/oauth2/token`（grant_type=password + client alexgo-app）
- [ ] 老端点 QPS 观测归零 ≥ 2 周（含 member refresh/logout）
- [ ] member 外部接入方（H5/小程序）盘点完毕并书面确认迁移
- [ ] 下线方式：先返回 `410 Gone`（`{"error":"gone"}`）观察一周再删路由——绝不直接 404/静默消失
- [ ] 下线前对老端点加 4xx/5xx 告警，异常即撤销下线

## 4. 风险

- 贸然删除：外部 app/member 登录与刷新瞬间全挂；仓库内**无任何编译错误或测试失败**会提示（消费方在库外）。
- 保留成本≈0：兼容层已是薄壳——校验/审计/签发单点在统一登录（D13 恰一次审计），端点无独立业务逻辑，无双轨漂移风险。

## 5. 结论

服务端兼容层**保留**，按 §3 清单推进下线；本任务只落地仓库内可证的部分：
删除 web-antd 的 `loginApi` 与 `AuthApi.LoginParams`/`AuthApi.LoginResult`
（`refreshTokenApi` 仍被 `apps/web-antd/src/api/request.ts` 消费，保留）。
