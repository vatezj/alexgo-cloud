package controller

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/taokouling/service"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/resp"
)

// Converter 消费端接口（测试桩）：Parse/Convert 与 ConversionService 逐字同签名。
type Converter interface {
	Parse(ctx context.Context, password, partner, clientIP string) (string, error)
	Convert(ctx context.Context, password, partner, clientIP string) (service.ConvertResult, error)
}

// AppController C 端双端点控制器（spec §4 parse/convert）。
type AppController struct {
	svc     Converter
	limiter *service.RateLimiter
}

// NewAppController 装配 C 端控制器：svc 为编排服务（桩可换），limiter 按 IP 前置限流。
func NewAppController(svc Converter, limiter *service.RateLimiter) *AppController {
	return &AppController{svc: svc, limiter: limiter}
}

// parseReq 两端点共用入参：password 必填（gin binding 10001），partner 选填。
type parseReq struct {
	Password string `json:"password" binding:"required"`
	Partner  string `json:"partner"`
}

// rateLimited 429 信封（形状与 resp 同构 {code,msg,data}；status 429 不走 resp.Fail——
// 其按码段把 307xx 映射为 400）。msg 取 errs 注册文案，cause 不外泄。
func rateLimited(c *gin.Context) {
	c.JSON(http.StatusTooManyRequests, gin.H{
		"code": int(errs.CodeTaoKoulingRateLimited),
		"msg":  errs.New(errs.CodeTaoKoulingRateLimited, nil).Msg(),
		"data": nil,
	})
}

// Parse 轻量链路：限流前置（命中直接 429，不触 service/不落日志）→ 绑参 →
// svc.Parse（service 自钉 tenant，此处透传 ctx.Request.Context()）→ itemID。
//
//	@Summary	淘口令解析（C 端）
//	@Tags		taokouling-app
//	@Param		body	body		object	true	"{password, partner?}"
//	@Success	200		{object}	map[string]interface{}
//	@Router		/app/taokouling/parse [post]
func (c *AppController) Parse(ctx *gin.Context) {
	if !c.limiter.Allow(ctx.ClientIP()) {
		rateLimited(ctx)
		return
	}
	var req parseReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	itemID, err := c.svc.Parse(ctx.Request.Context(), req.Password, req.Partner, ctx.ClientIP())
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, gin.H{"item_id": itemID})
}

// Convert 完整链路：限流前置 → 绑参 → svc.Convert → ConvertResult（降级仍 200，data.degraded=true）。
//
//	@Summary	淘口令转换（C 端，含降级）
//	@Tags		taokouling-app
//	@Param		body	body		object	true	"{password, partner?}"
//	@Success	200		{object}	map[string]interface{}
//	@Router		/app/taokouling/convert [post]
func (c *AppController) Convert(ctx *gin.Context) {
	if !c.limiter.Allow(ctx.ClientIP()) {
		rateLimited(ctx)
		return
	}
	var req parseReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	result, err := c.svc.Convert(ctx.Request.Context(), req.Password, req.Partner, ctx.ClientIP())
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, result)
}
