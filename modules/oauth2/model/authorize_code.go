package model

import "time"

// Oauth2AuthorizeCode 授权码（D4：租户列 Go 名 OwnerTenantID 规避租户插件注入；
// username 列供 jwt 模式授权码流回填 claims）。
type Oauth2AuthorizeCode struct {
	ID                  uint64    `gorm:"primaryKey" json:"id"`
	Code                string    `gorm:"size:64;uniqueIndex" json:"code"`
	ClientID            string    `gorm:"size:64;index" json:"client_id"`
	UserID              uint64    `json:"user_id"`
	Username            string    `gorm:"size:64" json:"username"`
	AccountType         string    `gorm:"size:16" json:"account_type"`
	OwnerTenantID       uint64    `gorm:"column:tenant_id" json:"owner_tenant_id"`
	RedirectURI         string    `gorm:"type:text" json:"redirect_uri"`
	Scopes              string    `gorm:"size:64" json:"scopes"`
	CodeChallenge       string    `gorm:"size:128" json:"code_challenge"`
	CodeChallengeMethod string    `gorm:"size:16" json:"code_challenge_method"`
	ExpiresAt           time.Time `json:"expires_at"`
	Revoked             bool      `json:"revoked"`
	CreatedAt           time.Time `json:"created_at"`
}

func (Oauth2AuthorizeCode) TableName() string { return "oauth2_authorize_code" }
