package social

import "time"

// 平台枚举（字符串，可扩展：douyin / alipay 后续加值）。
const (
	PlatformWechatMP  = "wechat_mp"  // 微信小程序 code2session
	PlatformWechatWeb = "wechat_web" // 微信网页授权
)

// SocialApp 租户级第三方登录渠道配置（每租户每平台一份）。
// Secret 明文存储（调用平台 API 必须明文）；json:"-" 保证任何 admin 响应不回显。
type SocialApp struct {
	ID         uint64    `gorm:"primaryKey" json:"id"`
	TenantID   uint64    `gorm:"column:tenant_id;uniqueIndex:uk_social_app_tp" json:"tenant_id"`
	Platform   string    `gorm:"size:32;uniqueIndex:uk_social_app_tp" json:"platform"`
	AppID      string    `gorm:"column:appid;size:64" json:"appid"`
	Secret     string    `gorm:"size:128" json:"-"`
	Status     int       `json:"status"` // 1 启用 0 停用
	CreateTime time.Time `gorm:"column:create_time" json:"create_time"`
	UpdateTime time.Time `gorm:"column:update_time" json:"update_time"`
}

func (SocialApp) TableName() string { return "system_social_app" }

// MemberIdentity 第三方身份绑定行（(tenant,platform,openid) 唯一）。
type MemberIdentity struct {
	ID           uint64    `gorm:"primaryKey" json:"id"`
	TenantID     uint64    `gorm:"column:tenant_id;uniqueIndex:uk_identity_tpo;index:idx_identity_tu" json:"tenant_id"`
	MemberUserID uint64    `gorm:"column:member_user_id" json:"member_user_id"`
	Platform     string    `gorm:"size:32;uniqueIndex:uk_identity_tpo" json:"platform"`
	OpenID       string    `gorm:"column:openid;size:128;uniqueIndex:uk_identity_tpo" json:"openid"`
	UnionID      string    `gorm:"column:unionid;size:128;index:idx_identity_tu" json:"unionid"` // 可空；空串不参与同人匹配
	CreateTime   time.Time `gorm:"column:create_time" json:"create_time"`
}

func (MemberIdentity) TableName() string { return "system_member_identity" }
