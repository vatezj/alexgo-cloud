package model

import "time"

type Dept struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	ParentID  uint64    `json:"parent_id"`
	Name      string    `json:"name"`
	Sort      int       `json:"sort"`
	Status    int       `json:"status"`
	TenantID  uint64    `json:"tenant_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Dept) TableName() string { return "depts" }

type Post struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	Sort      int       `json:"sort"`
	Status    int       `json:"status"`
	TenantID  uint64    `json:"tenant_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Post) TableName() string { return "posts" }

type DictType struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	Type      string    `json:"type"`
	Name      string    `json:"name"`
	Status    int       `json:"status"`
	Remark    string    `json:"remark"`
	TenantID  uint64    `json:"tenant_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (DictType) TableName() string { return "dict_types" }

type DictData struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	TypeID    uint64    `json:"type_id"`
	Label     string    `json:"label"`
	Value     string    `json:"value"`
	Sort      int       `json:"sort"`
	Status    int       `json:"status"`
	Remark    string    `json:"remark"`
	TenantID  uint64    `json:"tenant_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (DictData) TableName() string { return "dict_datas" }

type SystemConfig struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	Name      string    `json:"name"`
	Remark    string    `json:"remark"`
	TenantID  uint64    `json:"tenant_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (SystemConfig) TableName() string { return "system_configs" }

type SystemNotice struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Status    int       `json:"status"`
	TenantID  uint64    `json:"tenant_id"`
	CreatedAt time.Time `json:"created_at"`
	// Attachments 附件 JSON 数组文本（元素 {id,name,size,url}，url=/api/public/infra/file/<key>）。
	// 照 DataScopeDeptIDs 的 string 惯例（spec §2）：后端原样存取不解析——
	// 公告内容本就是 admin 受信富文本。size 4096 防 MySQL 默认 varchar(191) 截断。
	Attachments string    `gorm:"column:attachments;size:4096" json:"attachments"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (SystemNotice) TableName() string { return "system_notices" }

// SystemNoticeRead 公告已读回执：每租户每用户对每公告一行。
// 三列组合 uniqueIndex 同时服务两处：生产建表由上面迁移的 uk 落地、
// 测试 AutoMigrate 由本标签落地——缺任一处，clause.OnConflict 无冲突目标可依，
// 幂等写会退化成重复插行（NoticeRead_Idempotent 测试钉住）。
type SystemNoticeRead struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	TenantID  uint64    `gorm:"uniqueIndex:uk_notice_reads_tenant_user_notice" json:"tenant_id"`
	UserID    uint64    `gorm:"uniqueIndex:uk_notice_reads_tenant_user_notice" json:"user_id"`
	NoticeID  uint64    `gorm:"uniqueIndex:uk_notice_reads_tenant_user_notice" json:"notice_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (SystemNoticeRead) TableName() string { return "system_notice_reads" }
