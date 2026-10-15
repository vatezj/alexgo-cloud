package resp

import "github.com/gin-gonic/gin"

// PageReq 是统一分页入参（query: page/size）。绑定后必须调 Defaults()。
type PageReq struct {
	Page int `form:"page" binding:"omitempty,min=1"`
	Size int `form:"size" binding:"omitempty,min=1,max=100"`
}

// Defaults 补缺省值：page=1，size=10；已有非零值不覆盖。
func (p *PageReq) Defaults() {
	if p.Page <= 0 {
		p.Page = 1
	}
	if p.Size <= 0 {
		p.Size = 10
	}
}

// Page 写分页信封。req 须是 Defaults() 之后的值。
func Page[T any](c *gin.Context, list []T, total int64, req PageReq) {
	write(c, 200, 0, "ok", gin.H{
		"list":  list,
		"total": total,
		"page":  req.Page,
		"size":  req.Size,
	})
}
