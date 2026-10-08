package auth

type Claims struct {
	UserID   uint64 `json:"user_id"`
	Username string `json:"username"`
	TenantID uint64 `json:"tenant_id"`
	UserType int    `json:"user_type"` // 1管理员 2会员（token 模式填充；jwt 模式为 1）
	DeptID   uint64 `json:"dept_id"`   // 仅管理员
}
