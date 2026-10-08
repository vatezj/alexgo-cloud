//go:build ignore

package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/go-sql-driver/mysql"

	"alexGo-cloud/pkg/config"
)

type Column struct {
	Name       string
	Type       string
	ColumnType string
	Nullable   bool
	Key        string
	Extra      string
	Comment    string
}

type Table struct {
	Schema  string
	Name    string
	Module  string
	Entity  string
	Plural  string
	Columns []Field

	Imports struct {
		Time bool
		JSON bool
	}
}

type Field struct {
	ColumnName string
	GoName     string
	GoType     string
	JSONTag    string
	GormTag    string
}

func main() {
	module := flag.String("module", "", "target module name (e.g. system)")
	tables := flag.String("tables", "", "comma-separated table names (optional)")
	dsnFlag := flag.String("dsn", "", "mysql dsn (optional; default: config/env)")
	schemaFlag := flag.String("schema", "", "mysql schema/database (optional)")
	outDir := flag.String("out", "", "output dir (optional; default: modules/<module>)")
	force := flag.Bool("force", false, "overwrite files if exist")
	flag.Parse()

	if *module == "" {
		fmt.Fprintln(os.Stderr, "missing --module")
		os.Exit(2)
	}

	cfg, _ := config.LoadGlobalConfig()
	dsn := *dsnFlag
	if dsn == "" && cfg != nil {
		dsn = cfg.Database.DSN
	}
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "missing dsn: pass --dsn or set DB_DSN or config database.dsn")
		os.Exit(2)
	}

	schema := *schemaFlag
	if schema == "" {
		if parsed, err := mysql.ParseDSN(dsn); err == nil && parsed.DBName != "" {
			schema = parsed.DBName
		}
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	defer db.Close()

	if schema == "" {
		_ = db.QueryRow("SELECT DATABASE()").Scan(&schema)
	}
	if schema == "" {
		fmt.Fprintln(os.Stderr, "cannot determine schema/database; pass --schema or include db name in dsn")
		os.Exit(2)
	}

	targetOut := *outDir
	if targetOut == "" {
		targetOut = filepath.Join("modules", *module)
	}

	var tableList []string
	if *tables != "" {
		for _, t := range strings.Split(*tables, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tableList = append(tableList, t)
			}
		}
	} else {
		tableList, err = listTables(db, schema)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	}

	sort.Strings(tableList)

	for _, tbl := range tableList {
		t, err := inspectTable(db, schema, *module, tbl)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			continue
		}
		if err := generateTable(targetOut, t, *force); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
		}
	}
}

func listTables(db *sql.DB, schema string) ([]string, error) {
	rows, err := db.Query(`
SELECT table_name
FROM information_schema.tables
WHERE table_schema = ? AND table_type = 'BASE TABLE'`, schema)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

func inspectTable(db *sql.DB, schema, module, table string) (*Table, error) {
	rows, err := db.Query(`
SELECT column_name, data_type, column_type, is_nullable, column_key, extra, column_comment
FROM information_schema.columns
WHERE table_schema = ? AND table_name = ?
ORDER BY ordinal_position`, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []Column
	for rows.Next() {
		var c Column
		var isNullable string
		if err := rows.Scan(&c.Name, &c.Type, &c.ColumnType, &isNullable, &c.Key, &c.Extra, &c.Comment); err != nil {
			return nil, err
		}
		c.Nullable = strings.EqualFold(isNullable, "YES")
		cols = append(cols, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(cols) == 0 {
		return nil, fmt.Errorf("no columns for table %s.%s", schema, table)
	}

	entity := toCamel(table)
	entity = strings.TrimSuffix(entity, "s")

	t := &Table{
		Schema: schema,
		Name:   table,
		Module: module,
		Entity: entity,
		Plural: pluralize(entity),
	}

	for _, c := range cols {
		f := Field{
			ColumnName: c.Name,
			GoName:     toCamel(c.Name),
		}
		f.JSONTag = c.Name
		if looksSensitive(c.Name) {
			f.JSONTag = "-"
		}
		f.GoType, t.Imports.Time, t.Imports.JSON = mysqlTypeToGo(c, t.Imports.Time, t.Imports.JSON)
		f.GormTag = buildGormTag(c)
		t.Columns = append(t.Columns, f)
	}

	return t, nil
}

func buildGormTag(c Column) string {
	var parts []string
	parts = append(parts, "column:"+c.Name)
	if c.Key == "PRI" {
		parts = append(parts, "primaryKey")
	}
	if strings.Contains(strings.ToLower(c.Extra), "auto_increment") {
		parts = append(parts, "autoIncrement")
	}
	return strings.Join(parts, ";")
}

func looksSensitive(col string) bool {
	col = strings.ToLower(col)
	return strings.Contains(col, "password") || strings.Contains(col, "secret") || strings.Contains(col, "token")
}

func mysqlTypeToGo(c Column, needTime, needJSON bool) (string, bool, bool) {
	ct := strings.ToLower(c.ColumnType)
	t := strings.ToLower(c.Type)
	unsigned := strings.Contains(ct, "unsigned")

	var base string
	switch t {
	case "bigint":
		if unsigned {
			base = "uint64"
		} else {
			base = "int64"
		}
	case "int", "integer", "mediumint":
		if unsigned {
			base = "uint32"
		} else {
			base = "int"
		}
	case "smallint", "tinyint":
		if unsigned {
			base = "uint16"
		} else {
			base = "int"
		}
	case "float", "double":
		base = "float64"
	case "decimal", "numeric":
		base = "string"
	case "char", "varchar", "text", "mediumtext", "longtext", "tinytext":
		base = "string"
	case "json":
		needJSON = true
		base = "json.RawMessage"
	case "datetime", "timestamp", "date", "time":
		needTime = true
		base = "time.Time"
	case "blob", "tinyblob", "mediumblob", "longblob", "binary", "varbinary":
		base = "[]byte"
	default:
		base = "string"
	}

	if c.Nullable && base != "[]byte" && base != "json.RawMessage" && base != "string" {
		return "*" + base, needTime, needJSON
	}
	if c.Nullable && base == "time.Time" {
		return "*time.Time", needTime, needJSON
	}
	return base, needTime, needJSON
}

func generateTable(outRoot string, t *Table, force bool) error {
	targets := []struct {
		path string
		tpl  string
	}{
		{filepath.Join(outRoot, "model", strings.ToLower(t.Entity)+"_dbgen.go"), "model.go.tpl"},
		{filepath.Join(outRoot, "repository", strings.ToLower(t.Entity)+"_dbgen.go"), "repository.go.tpl"},
		{filepath.Join(outRoot, "service", strings.ToLower(t.Entity)+"_dbgen.go"), "service.go.tpl"},
		{filepath.Join(outRoot, "controller", "admin", strings.ToLower(t.Entity)+"_dbgen.go"), "controller_admin.go.tpl"},
		{filepath.Join(outRoot, "module_register_"+strings.ToLower(t.Entity)+"_dbgen.go"), "module_register.go.tpl"},
	}

	funcs := template.FuncMap{
		"lower": strings.ToLower,
	}

	for _, target := range targets {
		b, err := renderTemplate(filepath.Join("scripts", "templates", "db_crud", target.tpl), t, funcs)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target.path), 0o755); err != nil {
			return err
		}
		if !force {
			if _, err := os.Stat(target.path); err == nil {
				continue
			}
		}
		if err := os.WriteFile(target.path, b, 0o644); err != nil {
			return err
		}
	}

	return nil
}

func renderTemplate(path string, data any, funcs template.FuncMap) ([]byte, error) {
	tpl, err := template.New(filepath.Base(path)).Funcs(funcs).ParseFiles(path)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return nil, err
	}
	src := buf.Bytes()
	if out, err := format.Source(src); err == nil {
		return out, nil
	}
	return src, nil
}

func pluralize(s string) string {
	if s == "" {
		return s
	}
	if strings.HasSuffix(s, "s") {
		return s + "es"
	}
	return s + "s"
}

var reNonWord = regexp.MustCompile(`[^a-zA-Z0-9]+`)

func toCamel(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	parts := reNonWord.Split(s, -1)
	for i := range parts {
		if parts[i] == "" {
			continue
		}
		parts[i] = strings.ToUpper(parts[i][:1]) + strings.ToLower(parts[i][1:])
	}
	out := strings.Join(parts, "")
	if out == "Id" {
		return "ID"
	}
	out = strings.ReplaceAll(out, "Id", "ID")
	out = strings.ReplaceAll(out, "Url", "URL")
	out = strings.ReplaceAll(out, "Json", "JSON")
	return out
}

func init() {
	_ = json.RawMessage{}
	_ = time.Time{}
}
