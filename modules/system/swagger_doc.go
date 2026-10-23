// 本文件是 system 模块 Swagger spec 的 swaginit 入口（`make swagger` 以 -g 指向它）。
// 只承载 @title/@BasePath/@securityDefinitions 等通用注解，勿在此写业务代码。
// tag 命名 = 模块-侧（system-admin / system-app），详见 docs/swagger-guide.md。

// @title			alexGo-cloud system API
// @version		1.0
// @description	alexGo-cloud 后端 system 模块 API 文档（swaggo 生成，`make swagger` 刷新）。
// @description	admin 侧（/admin/**）统一 pkg/resp 信封 {code,msg,data}；app 契约侧为裸 JSON 无信封。
// @BasePath		/api
// @securityDefinitions.apikey	Bearer
// @in				header
// @name			Authorization
// @description	admin 侧统一登录令牌，格式 `Bearer <token>`。
// @securityDefinitions.apikey	X-Session
// @in				header
// @name			X-Session
// @description	app 契约侧自鉴权会话令牌（登录接口签发）。
package system
