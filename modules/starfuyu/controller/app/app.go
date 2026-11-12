package app

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/starfuyu/model"
	"alexGo-cloud/modules/starfuyu/service"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/resp"
	"alexGo-cloud/pkg/tenant"
	"alexGo-cloud/pkg/token"
)

// AppController app 端 starfuyu 接口（spec §4.1，13 端点）：Bearer member 自鉴权，
// 不走 casbin——claims 与 modules/scorecard/controller/app/app.go 同构。
type AppController struct {
	fortunes  service.FortuneService
	profiles  service.ProfileService
	bazis     service.BaziService
	divs      service.DivinationService
	images    service.ImageService
	validator token.Validator
}

// NewAppController 装配 C 端控制器。token.Validator 的接口映射归入口
// （alexgo-server/cmd/main.go），模块内不重复 Provide（scorecard 同款）。
func NewAppController(fortunes service.FortuneService, profiles service.ProfileService,
	bazis service.BaziService, divs service.DivinationService, images service.ImageService,
	validator token.Validator) *AppController {
	return &AppController{
		fortunes: fortunes, profiles: profiles, bazis: bazis,
		divs: divs, images: images, validator: validator,
	}
}

// claims 解析 Bearer 并校验 member 身份；成功后把 token 内租户写回请求 ctx
// （spec §4.3，与 scorecard app.go claims 同一 API）——TenantMiddleware 按
// X-Tenant-ID 头解析的值在此被覆盖，伪造头无法跨租户；失败已写响应，ok=false
// 直接 return。全部鉴权 handler 先过本函数再取 ctx.Request.Context()。
func (c *AppController) claims(ctx *gin.Context) (*token.Claims, bool) {
	raw := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(ctx.GetHeader("Authorization")), "Bearer "))
	if raw == "" {
		resp.Fail(ctx, errs.New(errs.CodeUnauthorized, nil))
		return nil, false
	}
	cl, err := c.validator.Validate(ctx.Request.Context(), raw)
	if err != nil {
		resp.Fail(ctx, errs.New(errs.CodeUnauthorized, nil))
		return nil, false
	}
	if cl.UserType != token.UserTypeMember {
		resp.Fail(ctx, errs.New(errs.CodeForbidden, nil))
		return nil, false
	}
	ctx.Request = ctx.Request.WithContext(tenant.WithTenantID(ctx.Request.Context(), cl.TenantID))
	return cl, true
}

// dateParam 校验 :date 为 YYYY-MM-DD（spec §4.1：非法 10001）；通过返回原串。
func dateParam(ctx *gin.Context) (string, bool) {
	raw := ctx.Param("date")
	if _, err := time.Parse("2006-01-02", raw); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return "", false
	}
	return raw, true
}

// baziBirthReq 计算/更新八字入参（spec §4.1：birth_date 必填 + 格式校验在 service，
// 此处不加 binding tag，避免把 10002 提前改写成 10001）。
type baziBirthReq struct {
	BirthDate  string `json:"birth_date"`
	BirthTime  string `json:"birth_time"`
	BirthPlace string `json:"birth_place"`
}

// ---- 运势 ----

// FortuneToday 今日全部 12 星座运势；无数据 → 空列表而非报错（凌晨任务未跑时
// 客户端可兜底，spec §4.1）。内容表读恒 tid=0（service 内钉）。
func (c *AppController) FortuneToday(ctx *gin.Context) {
	if _, ok := c.claims(ctx); !ok {
		return
	}
	list, err := c.fortunes.GetByDate(ctx.Request.Context(), service.TodaySH())
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, list)
}

// FortuneByDate 指定日期运势（YYYY-MM-DD 校验 10001；无数据空列表）。
func (c *AppController) FortuneByDate(ctx *gin.Context) {
	if _, ok := c.claims(ctx); !ok {
		return
	}
	date, ok := dateParam(ctx)
	if !ok {
		return
	}
	list, err := c.fortunes.GetByDate(ctx.Request.Context(), date)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, list)
}

// FortuneMyToday 我的今日运势（需 profile 星座，未设 → 30505）。
func (c *AppController) FortuneMyToday(ctx *gin.Context) {
	cl, ok := c.claims(ctx)
	if !ok {
		return
	}
	c.myFortune(ctx, cl.UserID, service.TodaySH())
}

// FortuneMyByDate 我的指定日期运势（同上）。
func (c *AppController) FortuneMyByDate(ctx *gin.Context) {
	cl, ok := c.claims(ctx)
	if !ok {
		return
	}
	date, ok := dateParam(ctx)
	if !ok {
		return
	}
	c.myFortune(ctx, cl.UserID, date)
}

// myFortune fortune/my/* 组合（brief 钉死）：GetMy → nil/空星座 → 30505；
// 有则按星座 GetOne（miss → 30501 透传）。
func (c *AppController) myFortune(ctx *gin.Context, userID uint64, date string) {
	p, err := c.profiles.GetMy(ctx.Request.Context(), userID)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	if p == nil || p.Constellation == "" {
		resp.Fail(ctx, errs.New(errs.CodeStarfuyuConstellationMissing, nil))
		return
	}
	fortune, err := c.fortunes.GetOne(ctx.Request.Context(), date, p.Constellation)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, fortune)
}

// FortuneImage 该星座当日运势图：Ensure 现场生成（幂等）→ 返回公开读 URL
// （brief 钉死响应 {url}；:constellation/:date 路由序与 Ensure(date,…) 入参序相反）。
func (c *AppController) FortuneImage(ctx *gin.Context) {
	if _, ok := c.claims(ctx); !ok {
		return
	}
	key, err := c.images.Ensure(ctx.Request.Context(), ctx.Param("date"), ctx.Param("constellation"))
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, gin.H{"url": "/api/public/starfuyu/image/" + key})
}

// ---- 八字 ----

// BaziCalculate 计算八字：DeepSeek 排盘并覆盖保存我的八字；限流命中 service 上抛 30503。
func (c *AppController) BaziCalculate(ctx *gin.Context) {
	c.baziUpsert(ctx, c.bazis.Calculate)
}

// BaziUpdateMy 更新我的八字（同 calculate，同样限流）。
func (c *AppController) BaziUpdateMy(ctx *gin.Context) {
	c.baziUpsert(ctx, c.bazis.UpdateMy)
}

// baziUpsert calculate/update 公共骨架：claims → 绑定 → 服务调用 → 信封
// （两个 service 方法签名相同，方法值直传，scorecard roomAction 同款去重）。
func (c *AppController) baziUpsert(ctx *gin.Context,
	do func(context.Context, uint64, service.BaziReq) (*model.Bazi, error)) {
	cl, ok := c.claims(ctx)
	if !ok {
		return
	}
	var body baziBirthReq
	if err := ctx.ShouldBindJSON(&body); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	row, err := do(ctx.Request.Context(), cl.UserID, service.BaziReq{
		BirthDate:  body.BirthDate,
		BirthTime:  body.BirthTime,
		BirthPlace: body.BirthPlace,
	})
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, row)
}

// BaziMy 我的八字；无 → 30504（service 上抛）。
func (c *AppController) BaziMy(ctx *gin.Context) {
	cl, ok := c.claims(ctx)
	if !ok {
		return
	}
	bazi, err := c.bazis.GetMy(ctx.Request.Context(), cl.UserID)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, bazi)
}

// ---- 求签 ----

// DivinationSign 指定签号签文（1–100 越界 10001 由 service 校验）。
// 公开端点：唯一不挂 claims 的路由，签文按号缓存零 LLM（决定 15）。
func (c *AppController) DivinationSign(ctx *gin.Context) {
	number, err := strconv.Atoi(ctx.Param("number"))
	if err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	content, err := c.divs.GetSign(ctx.Request.Context(), number)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, gin.H{"sign_number": number, "sign_content": content})
}

// DivinationDraw 求签落库（限流命中 30503 由 service 上抛）；question/category 均可空。
func (c *AppController) DivinationDraw(ctx *gin.Context) {
	cl, ok := c.claims(ctx)
	if !ok {
		return
	}
	var body struct {
		Question string `json:"question"`
		Category string `json:"category"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	row, err := c.divs.Draw(ctx.Request.Context(), cl.UserID, body.Question, body.Category)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, row)
}

// DivinationHistory 我的求签历史（page/size 分页；size≤100 由 PageReq binding 挡，
// 超限 → 10001——service 契约注明「size≤100 归 controller」）。
func (c *AppController) DivinationHistory(ctx *gin.Context) {
	cl, ok := c.claims(ctx)
	if !ok {
		return
	}
	var req resp.PageReq
	if err := ctx.ShouldBindQuery(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	req.Defaults() // page=1 size=10
	list, total, err := c.divs.History(ctx.Request.Context(), cl.UserID, req.Page, req.Size)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.Page(ctx, list, total, req)
}

// ---- 星座偏好 ----

// ProfileGet 我的星座偏好；未设 → constellation:"" 不报错（brief 钉死）。
func (c *AppController) ProfileGet(ctx *gin.Context) {
	cl, ok := c.claims(ctx)
	if !ok {
		return
	}
	p, err := c.profiles.GetMy(ctx.Request.Context(), cl.UserID)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	if p == nil {
		resp.OK(ctx, gin.H{"constellation": ""})
		return
	}
	resp.OK(ctx, gin.H{"constellation": p.Constellation})
}

// ProfilePut 设置/更新星座（12 星座枚举校验 10001 由 service.SetMy 执行）。
func (c *AppController) ProfilePut(ctx *gin.Context) {
	cl, ok := c.claims(ctx)
	if !ok {
		return
	}
	var body struct {
		Constellation string `json:"constellation"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	p, err := c.profiles.SetMy(ctx.Request.Context(), cl.UserID, body.Constellation)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, gin.H{"constellation": p.Constellation})
}
