package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"alexGo-cloud/modules/infra/model"
	"alexGo-cloud/modules/infra/repository"
	"alexGo-cloud/pkg/codegen/builder"
	"alexGo-cloud/pkg/codegen/metadata"
	cgmodel "alexGo-cloud/pkg/codegen/model"
)

// CodegenService admin 业务层（spec §4）。
type CodegenService interface {
	ListTables(ctx context.Context, page, size int) ([]*model.CodegenTable, int64, error)
	GetTable(ctx context.Context, id uint64) (*model.CodegenTable, []*model.CodegenColumn, error)
	ImportTable(ctx context.Context, tableName string) (*model.CodegenTable, error)
	UpdateTable(ctx context.Context, t *model.CodegenTable) error
	DeleteTable(ctx context.Context, id uint64) error
	SaveColumns(ctx context.Context, tableID uint64, cols []*model.CodegenColumn) error
	ListColumns(ctx context.Context, tableID uint64) ([]*model.CodegenColumn, error)
	// Sync 从元数据源重读表并计算+落库差异（spec §9.4）。
	Sync(ctx context.Context, tableID uint64) (*model.SyncResult, error)
}

type codegenService struct {
	reader metadata.MetadataReader
	repo   repository.CodegenRepository
	opts   builder.Options
}

func NewCodegenService(reader metadata.MetadataReader, repo repository.CodegenRepository, opts builder.Options) CodegenService {
	return &codegenService{reader: reader, repo: repo, opts: opts}
}

func (s *codegenService) ListTables(ctx context.Context, page, size int) ([]*model.CodegenTable, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 200 {
		size = 10
	}
	return s.repo.ListTables(ctx, page, size)
}

func (s *codegenService) GetTable(ctx context.Context, id uint64) (*model.CodegenTable, []*model.CodegenColumn, error) {
	t, err := s.repo.GetTable(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	cols, err := s.repo.ListColumns(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	return t, cols, nil
}

// ImportTable 元数据 → builder 构建期校验（含 §9.1 单列 id 主键）→ 落配置。
// 校验失败时 repo 一行不写：Build 在前，Create 在后。
func (s *codegenService) ImportTable(ctx context.Context, tableName string) (*model.CodegenTable, error) {
	if err := validateIdent(tableName); err != nil {
		return nil, err
	}
	// 1) 去重：已导入直接拒绝
	names, err := s.repo.ImportedTableNames(ctx)
	if err != nil {
		return nil, err
	}
	for _, n := range names {
		if n == tableName {
			return nil, fmt.Errorf("table %q already imported", tableName)
		}
	}
	// 2) 读元数据（不存在 → ErrTableNotFound，原样上抛给 400）
	meta, err := s.reader.ReadTable(ctx, tableName)
	if err != nil {
		return nil, err
	}
	// 3) 构建期校验：无列/主键规则 → ErrColumnInvalid 系
	tbl, err := builder.Build(meta, s.opts)
	if err != nil {
		return nil, err
	}
	// 4) 快照落库。快照字段按 meta 序（ordinal 即配置序），
	//    GoType/JSONName/主键标记等从构建产物按列名联表取。
	t := &model.CodegenTable{
		Name:         meta.Name,
		TableComment: meta.Comment,
		Module:       s.opts.Module,
		BusinessName: tbl.BusinessName,
		ClassName:    tbl.ClassName,
		TemplateType: int(tbl.TemplateType),
		FrontType:    int(tbl.FrontType),
	}
	built := builtByName(tbl.Columns)
	cols := make([]*model.CodegenColumn, 0, len(meta.Columns))
	for i, cm := range meta.Columns {
		b := built[cm.Name]
		if b == nil {
			// Build 校验保证 meta/built 列集一致，此处纯防御。
			return nil, fmt.Errorf("%w: %s: column %q missing in build output", cgmodel.ErrColumnInvalid, meta.Name, cm.Name)
		}
		cols = append(cols, &model.CodegenColumn{
			Name:          cm.Name,
			Type:          cm.ColumnType,
			Comment:       b.Comment,
			GoType:        b.GoType,
			JSONName:      b.JSONName,
			IsPK:          b.IsPK,
			AutoIncrement: b.AutoIncrement,
			Nullable:      b.Nullable,
			HTMLType:      defaultHTMLType(b.IsPK, b.GoType),
			// 导入初值：主键/自增不进表单；query 开关默认关（需显式打开）。
			ListEnable:     !b.IsPK && !b.AutoIncrement,
			FormEnable:     !b.IsPK && !b.AutoIncrement,
			QueryEnable:    false,
			QueryOperation: "eq",
			SortOrder:      i,
		})
	}
	if err := s.repo.CreateTable(ctx, t, cols); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *codegenService) UpdateTable(ctx context.Context, t *model.CodegenTable) error {
	if _, err := s.repo.GetTable(ctx, t.ID); err != nil {
		return err
	}
	return s.repo.UpdateTable(ctx, t)
}

func (s *codegenService) DeleteTable(ctx context.Context, id uint64) error {
	return s.repo.DeleteTable(ctx, id)
}

func (s *codegenService) SaveColumns(ctx context.Context, tableID uint64, cols []*model.CodegenColumn) error {
	if len(cols) == 0 {
		return errors.New("no columns to save")
	}
	return s.repo.SaveColumns(ctx, tableID, cols)
}

func (s *codegenService) ListColumns(ctx context.Context, tableID uint64) ([]*model.CodegenColumn, error) {
	return s.repo.ListColumns(ctx, tableID)
}

// Sync：未导入的表 → ErrTableNotFound（不静默建行）。
// diff 分类与落库在 Task 6 补全；此处先钉前置校验语义。
func (s *codegenService) Sync(ctx context.Context, tableID uint64) (*model.SyncResult, error) {
	if _, err := s.repo.GetTable(ctx, tableID); err != nil {
		return nil, fmt.Errorf("%w: table %d not imported", cgmodel.ErrTableNotFound, tableID)
	}
	return nil, errors.New("sync diff not implemented yet")
}

var identRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// validateIdent 表名只允许小写 snake——它是拼进 SQL/文件名的输入面。
func validateIdent(name string) error {
	if !identRe.MatchString(name) || len(name) > 128 {
		return fmt.Errorf("invalid table name %q", name)
	}
	return nil
}

// builtByName 按列名索引构建产物，供 meta ↔ built 联表。
func builtByName(cols []cgmodel.Column) map[string]*cgmodel.Column {
	m := make(map[string]*cgmodel.Column, len(cols))
	for i := range cols {
		m[cols[i].Name] = &cols[i]
	}
	return m
}

func defaultHTMLType(isPK bool, goType string) string {
	// 主键/自增不进表单；类型粗映射，前端可再改（配置字段，同步不覆盖）。
	switch {
	case isPK:
		return "Input"
	case goType == "bool":
		return "Switch"
	case goType == "time.Time":
		return "DatePicker"
	case goType == "int" || goType == "int64" || goType == "uint64" || goType == "float64":
		return "InputNumber"
	default:
		return "Input"
	}
}
