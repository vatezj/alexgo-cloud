package model

import "time"

// CodegenTable 代码生成表配置（快照+配置）。系统级：无 tenant_id、无 deleted（spec §1/§9.3）。
// 注意：表名字段叫 Name 而非 TableName——后者与 gorm 约定方法 TableName() 冲突。
type CodegenTable struct {
	ID            uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Name          string    `gorm:"column:table_name;uniqueIndex:uk_codegen_table_name" json:"table_name"`
	TableComment  string    `gorm:"column:table_comment" json:"table_comment"`
	Module        string    `gorm:"column:module" json:"module"`
	BusinessName  string    `gorm:"column:business_name" json:"business_name"`
	ClassName     string    `gorm:"column:class_name" json:"class_name"`
	TemplateType  int       `gorm:"column:template_type" json:"template_type"`
	FrontType     int       `gorm:"column:front_type" json:"front_type"`
	ParentTableID uint64    `gorm:"column:parent_table_id" json:"parent_table_id"`
	Remark        string    `gorm:"column:remark" json:"remark"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (CodegenTable) TableName() string { return "codegen_table" }

// CodegenColumn 字段配置：type/comment/go_type/json_name/主键空性 = 快照；
// html_type/开关/query_operation/dict_type/sort_order = 用户配置（同步不覆盖，§9.4）。
type CodegenColumn struct {
	ID             uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	TableID        uint64    `gorm:"column:table_id;uniqueIndex:uk_codegen_column_table_name" json:"table_id"`
	Name           string    `gorm:"column:name;uniqueIndex:uk_codegen_column_table_name" json:"name"`
	Type           string    `gorm:"column:type" json:"type"`
	Comment        string    `gorm:"column:comment" json:"comment"`
	GoType         string    `gorm:"column:go_type" json:"go_type"`
	JSONName       string    `gorm:"column:json_name" json:"json_name"`
	IsPK           bool      `gorm:"column:is_pk" json:"is_pk"`
	AutoIncrement  bool      `gorm:"column:auto_increment" json:"auto_increment"`
	Nullable       bool      `gorm:"column:nullable" json:"nullable"`
	HTMLType       string    `gorm:"column:html_type" json:"html_type"`
	ListEnable     bool      `gorm:"column:list_enable" json:"list_enable"`
	FormEnable     bool      `gorm:"column:form_enable" json:"form_enable"`
	QueryEnable    bool      `gorm:"column:query_enable" json:"query_enable"`
	QueryOperation string    `gorm:"column:query_operation" json:"query_operation"`
	ListRequired   bool      `gorm:"column:list_required" json:"list_required"`
	FormRequired   bool      `gorm:"column:form_required" json:"form_required"`
	DictType       string    `gorm:"column:dict_type" json:"dict_type"`
	SortOrder      int       `gorm:"column:sort_order" json:"sort_order"`
	Deprecated     bool      `gorm:"column:deprecated" json:"deprecated"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (CodegenColumn) TableName() string { return "codegen_column" }

// SyncDiff 是 service 对比元数据算出的差异（纯数据，由仓库事务落库）。
type SyncDiff struct {
	Added      []*CodegenColumn // 新列：开关全关（§9.4）
	Refresh    []*CodegenColumn // 快照刷新或 deprecated 恢复的既有行（含 ID）
	Deprecated []uint64         // 消失列的行 id（置 deprecated=1）
}

// SyncResult 是同步结果计数（service 算，controller 直接序列化）。
type SyncResult struct {
	Added      int `json:"added"`
	Updated    int `json:"updated"`
	Deprecated int `json:"deprecated"`
	Unchanged  int `json:"unchanged"`
}
