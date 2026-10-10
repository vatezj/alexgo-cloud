// pkg/codegen/builder/typing_test.go
package builder_test

import (
	"errors"
	"testing"

	"alexGo-cloud/pkg/codegen/builder"
	"alexGo-cloud/pkg/codegen/metadata"
	"alexGo-cloud/pkg/codegen/model"
)

func col(dataType, columnType string, nullable bool) metadata.ColumnMeta {
	return metadata.ColumnMeta{Name: "c", DataType: dataType, ColumnType: columnType, Nullable: nullable}
}

func TestMySQLTypeToGo(t *testing.T) {
	cases := []struct {
		name    string
		col     metadata.ColumnMeta
		want    string
		wantErr bool
	}{
		{"bigint unsigned", col("bigint", "bigint unsigned", false), "uint64", false},
		{"bigint", col("bigint", "bigint", false), "int64", false},
		{"int", col("int", "int", false), "int", false},
		{"int nullable", col("int", "int", true), "*int", false},
		{"varchar", col("varchar", "varchar(64)", false), "string", false},
		{"varchar nullable stays string", col("varchar", "varchar(64)", true), "string", false},
		{"decimal", col("decimal", "decimal(10,2)", false), "string", false},
		{"datetime", col("datetime", "datetime", false), "time.Time", false},
		{"datetime nullable", col("datetime", "datetime", true), "*time.Time", false},
		{"timestamp nullable", col("timestamp", "timestamp", true), "*time.Time", false},
		{"json", col("json", "json", false), "json.RawMessage", false},
		{"blob nullable", col("blob", "blob", true), "[]byte", false},
		{"enum", col("enum", "enum('a','b')", false), "string", false},
		{"set", col("set", "set('a')", false), "string", false},
		{"geometry", col("geometry", "geometry", false), "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := builder.MySQLTypeToGo(c.col)
			if c.wantErr {
				if !errors.Is(err, model.ErrTypeMappingUnknown) {
					t.Fatalf("err = %v, want ErrTypeMappingUnknown", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v", err)
			}
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestMySQLTypeToHTML(t *testing.T) {
	cases := []struct {
		name  string
		col   metadata.ColumnMeta
		goTyp string
		want  model.HTMLType
	}{
		{"varchar → Input", col("varchar", "varchar(64)", false), "string", model.HTMLInput},
		{"text → Textarea", col("text", "text", false), "string", model.HTMLTextarea},
		{"tinyint(1) → Switch", col("tinyint", "tinyint(1)", false), "int", model.HTMLSwitch},
		{"tinyint(4) → InputNumber", col("tinyint", "tinyint(4)", false), "int", model.HTMLInputNumber},
		{"int → InputNumber", col("int", "int", false), "int", model.HTMLInputNumber},
		{"nullable int → InputNumber", col("int", "int", true), "*int", model.HTMLInputNumber},
		{"decimal → InputNumber", col("decimal", "decimal(10,2)", false), "string", model.HTMLInputNumber},
		{"datetime → DatePicker", col("datetime", "datetime", false), "time.Time", model.HTMLDatePicker},
		{"json → Textarea", col("json", "json", false), "json.RawMessage", model.HTMLTextarea},
	}
	for _, c := range cases {
		if got := builder.MySQLTypeToHTML(c.col, c.goTyp); got != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}
}

func TestBuildGormTag(t *testing.T) {
	pk := col("bigint", "bigint unsigned", false)
	pk.Name = "id"
	pk.Key = "PRI"
	pk.Extra = "auto_increment"
	if got, want := builder.BuildGormTag(pk), "column:id;primaryKey;autoIncrement"; got != want {
		t.Errorf("BuildGormTag(pk) = %q, want %q", got, want)
	}
	plain := col("varchar", "varchar(64)", false)
	plain.Name = "name"
	if got, want := builder.BuildGormTag(plain), "column:name"; got != want {
		t.Errorf("BuildGormTag(name) = %q, want %q", got, want)
	}
}
