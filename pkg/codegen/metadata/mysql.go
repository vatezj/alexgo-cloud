package metadata

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"alexGo-cloud/pkg/codegen/model"
)

type MySQLReader struct {
	db     *sql.DB
	schema string
}

func NewMySQLReader(db *sql.DB, schema string) *MySQLReader {
	return &MySQLReader{db: db, schema: schema}
}

func (r *MySQLReader) ListTables(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT table_name
FROM information_schema.tables
WHERE table_schema = ? AND table_type = 'BASE TABLE'
ORDER BY table_name`, r.schema)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", model.ErrMetadataUnreachable, err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("%w: %v", model.ErrMetadataUnreachable, err)
		}
		out = append(out, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%w: %v", model.ErrMetadataUnreachable, err)
	}
	return out, nil
}

func (r *MySQLReader) ReadTable(ctx context.Context, table string) (*TableMeta, error) {
	var comment string
	err := r.db.QueryRowContext(ctx, `
SELECT table_comment
FROM information_schema.tables
WHERE table_schema = ? AND table_name = ?`, r.schema, table).Scan(&comment)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("%w: %s.%s", model.ErrTableNotFound, r.schema, table)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", model.ErrMetadataUnreachable, err)
	}

	rows, err := r.db.QueryContext(ctx, `
SELECT column_name, data_type, column_type, column_comment, column_key, extra, is_nullable, ordinal_position
FROM information_schema.columns
WHERE table_schema = ? AND table_name = ?
ORDER BY ordinal_position`, r.schema, table)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", model.ErrMetadataUnreachable, err)
	}
	defer rows.Close()

	meta := &TableMeta{Schema: r.schema, Name: table, Comment: comment}
	for rows.Next() {
		var c ColumnMeta
		var isNullable string
		if err := rows.Scan(&c.Name, &c.DataType, &c.ColumnType, &c.Comment, &c.Key, &c.Extra, &isNullable, &c.Ordinal); err != nil {
			return nil, fmt.Errorf("%w: %v", model.ErrMetadataUnreachable, err)
		}
		c.Nullable = strings.EqualFold(isNullable, "YES")
		c.Comment = strings.Join(strings.Fields(c.Comment), " ") // 注释压平单行
		meta.Columns = append(meta.Columns, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%w: %v", model.ErrMetadataUnreachable, err)
	}
	if len(meta.Columns) == 0 {
		return nil, fmt.Errorf("%w: %s has no columns", model.ErrTableNotFound, table)
	}
	return meta, nil
}
