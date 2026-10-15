package model

import (
	"strings"
	"time"
)

// Oauth2Client 接入方注册（D4：无 tenant_id/deleted，硬删除）。
type Oauth2Client struct {
	ID           uint64    `gorm:"primaryKey" json:"id"`
	ClientID     string    `gorm:"size:64;uniqueIndex" json:"client_id"`
	ClientSecret string    `gorm:"size:128" json:"-"`
	ClientName   string    `gorm:"size:128" json:"client_name"`
	Website      string    `gorm:"size:255" json:"website"`
	GrantTypes   string    `gorm:"size:255" json:"grant_types"`    // 逗号分隔
	RedirectURIs string    `gorm:"type:text" json:"redirect_uris"` // 换行分隔，精确匹配
	AccountTypes string    `gorm:"size:64" json:"account_types"`   // 逗号分隔 system,member,client
	Scopes       string    `gorm:"size:64" json:"scopes"`
	Activity     int       `json:"activity"` // 1 启用 0 停用
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (Oauth2Client) TableName() string { return "oauth2_client" }

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// AllowsGrant grant 白名单精确匹配。
func (c *Oauth2Client) AllowsGrant(g string) bool {
	for _, x := range splitCSV(c.GrantTypes) {
		if x == g {
			return true
		}
	}
	return false
}

// AllowsAccountType 账号类型白名单（D10 防跨类型冒用）。
func (c *Oauth2Client) AllowsAccountType(t string) bool {
	for _, x := range splitCSV(c.AccountTypes) {
		if x == t {
			return true
		}
	}
	return false
}

// AllowsRedirect redirect_uri 精确匹配（多条换行分隔）。
func (c *Oauth2Client) AllowsRedirect(u string) bool {
	for _, line := range strings.Split(c.RedirectURIs, "\n") {
		if strings.TrimSpace(line) == u && u != "" {
			return true
		}
	}
	return false
}
