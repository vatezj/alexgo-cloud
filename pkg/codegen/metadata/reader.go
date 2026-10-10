package metadata

import "context"

// ColumnMeta 是 information_schema.columns 的原始快照（不含任何推导）。
type ColumnMeta struct {
	Name       string
	DataType   string // varchar / bigint / datetime ...
	ColumnType string // varchar(64) unsigned / tinyint(1) ...
	Comment    string
	Key        string // PRI / UNI / MUL / ""
	Extra      string // auto_increment / ...
	Nullable   bool
	Ordinal    int
}

// TableMeta 是表级原始快照。
type TableMeta struct {
	Schema  string
	Name    string
	Comment string
	Columns []ColumnMeta
}

// MetadataReader 抽象库表元数据读取；M1 仅 MySQL 实现，接口保留扩展位。
type MetadataReader interface {
	ListTables(ctx context.Context) ([]string, error)
	ReadTable(ctx context.Context, table string) (*TableMeta, error)
}
