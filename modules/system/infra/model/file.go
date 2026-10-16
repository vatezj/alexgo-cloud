package model

import "time"

// SystemFile 文件元数据（子项目 B / P1-1 spec §2）。
// RelPath 只做盘内定位、绝不外泄给客户端；PublicKey 才是对外读键——
// 32 hex 恒生成（含 private 行）：避开 unique 索引空串冲突，是否放行只看 Public。
type SystemFile struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:255" json:"name"` // 原始文件名：只进元数据/Disposition，不进盘路径
	Ext       string    `gorm:"size:32" json:"ext"`   // 小写扩展名、无点
	Mime      string    `gorm:"size:128" json:"mime"` // http.DetectContentType 探测值
	Size      int64     `json:"size"`
	RelPath   string    `gorm:"column:rel_path;size:255;uniqueIndex" json:"rel_path"`
	PublicKey string    `gorm:"column:public_key;size:32;uniqueIndex" json:"public_key"`
	Public    bool      `json:"public"`
	BizType   string    `gorm:"column:biz_type;size:64" json:"biz_type"` // notice/general，仅归类
	TenantID  uint64    `gorm:"column:tenant_id;index" json:"tenant_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (SystemFile) TableName() string { return "system_files" }
