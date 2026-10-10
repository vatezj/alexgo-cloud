package model

type Column struct {
	Name    string // 列名，如 user_id
	Comment string // 列注释（已压平为单行）

	GoName   string // Go 字段名，如 UserID
	GoType   string // Go 类型，如 uint64、*time.Time
	JSONName string // json tag；敏感字段为 "-"
	GormTag  string // gorm tag 内容，如 column:user_id
	HTMLType HTMLType

	IsPK          bool
	AutoIncrement bool
	Nullable      bool
	ListEnable    bool
	FormEnable    bool
	QueryEnable   bool
	QueryOp       string // eq / like / between
	ListRequired  bool
	FormRequired  bool
	DictType      string // 预留字典类型
	SortOrder     int
	Deprecated    bool // 同步时列已消失
}
