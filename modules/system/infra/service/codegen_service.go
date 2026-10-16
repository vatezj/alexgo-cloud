package service

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"regexp"

	"alexGo-cloud/modules/system/infra/model"
	"alexGo-cloud/modules/system/infra/repository"
	"alexGo-cloud/pkg/codegen"
	"alexGo-cloud/pkg/codegen/builder"
	"alexGo-cloud/pkg/codegen/metadata"
	cgmodel "alexGo-cloud/pkg/codegen/model"
	errs "alexGo-cloud/pkg/errors"
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
	// ListDBTables 列出库内可导入的表名（导入弹窗下拉数据源）。
	ListDBTables(ctx context.Context) ([]string, error)
	// Preview 渲染选中配置表为生成文件（纯内存，spec §4/§6）。
	Preview(ctx context.Context, tableIDs []uint64) ([]cgmodel.GeneratedFile, error)
	// Download 与 Preview 同一份渲染结果打成 ZIP（spec §4）。
	Download(ctx context.Context, tableIDs []uint64) ([]byte, error)
}

type codegenService struct {
	reader         metadata.MetadataReader
	repo           repository.CodegenRepository
	opts           builder.Options
	unitTestEnable bool // config.Codegen.UnitTestEnable（CLI -no-tests 由调用方置 false）
}

func NewCodegenService(
	reader metadata.MetadataReader,
	repo repository.CodegenRepository,
	opts builder.Options,
	unitTestEnable bool,
) CodegenService {
	return &codegenService{reader: reader, repo: repo, opts: opts, unitTestEnable: unitTestEnable}
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
			return nil, errs.New(errs.CodeCodegenTableTaken, nil)
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

// UpdateTable 白名单合并进既有行（评审 Important#1）：
// gorm Save 全量覆盖含零值，部分 body 会把 table_name/module/created_at 清零——
// table_name 一空 Sync 永久失效（它定位物理表）。故 body 只放行可编辑配置字段，
// 快照/系统字段（table_name、created_at、table_comment）一律取库中现值。
func (s *codegenService) UpdateTable(ctx context.Context, t *model.CodegenTable) error {
	existing, err := s.repo.GetTable(ctx, t.ID)
	if err != nil {
		return err
	}
	if t.Module != "" {
		existing.Module = t.Module
	}
	if t.BusinessName != "" {
		existing.BusinessName = t.BusinessName
	}
	if t.ClassName != "" {
		existing.ClassName = t.ClassName
	}
	if t.TemplateType != 0 {
		existing.TemplateType = t.TemplateType
	}
	if t.FrontType != 0 {
		existing.FrontType = t.FrontType
	}
	if t.Remark != "" {
		existing.Remark = t.Remark
	}
	if t.ParentTableID != 0 {
		existing.ParentTableID = t.ParentTableID
	}
	if err := s.validateConfig(ctx, existing); err != nil {
		return err
	}
	return s.repo.UpdateTable(ctx, existing)
}

// validateConfig 更新前校验（评审 Important#2）：module/class_name 拼进生成物
// 路径与包名——路径穿越、非法标识符、类名冲突必须在入库前拒绝（Focus #3），
// 否则 M3 生成期才爆，坏配置已污染库。
func (s *codegenService) validateConfig(ctx context.Context, t *model.CodegenTable) error {
	if !identRe.MatchString(t.Module) || len(t.Module) > 128 {
		return fmt.Errorf("%w: invalid module %q", cgmodel.ErrTableInvalid, t.Module)
	}
	if !goExportedRe.MatchString(t.ClassName) || len(t.ClassName) > 128 {
		return fmt.Errorf("%w: invalid class_name %q", cgmodel.ErrTableInvalid, t.ClassName)
	}
	if !identRe.MatchString(t.BusinessName) || len(t.BusinessName) > 128 {
		return fmt.Errorf("%w: invalid business_name %q", cgmodel.ErrTableInvalid, t.BusinessName)
	}
	all, err := s.repo.ListAllTables(ctx)
	if err != nil {
		return err
	}
	for _, o := range all {
		if o.ID != t.ID && o.Module == t.Module && o.ClassName == t.ClassName {
			return fmt.Errorf("%w: class %s.%s already used by table %q",
				cgmodel.ErrTableInvalid, t.Module, t.ClassName, o.Name)
		}
	}
	return nil
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

// ListDBTables 直读 information_schema（元数据读取器），供导入弹窗下拉。
func (s *codegenService) ListDBTables(ctx context.Context) ([]string, error) {
	return s.reader.ListTables(ctx)
}

// Preview 渲染选中配置表（spec §4：POST preview；纯内存，引擎失败无副作用）。
func (s *codegenService) Preview(ctx context.Context, tableIDs []uint64) ([]cgmodel.GeneratedFile, error) {
	return s.generate(ctx, tableIDs)
}

// Download 与 Preview 同一份渲染逻辑打包 ZIP（spec §4：POST download）。
func (s *codegenService) Download(ctx context.Context, tableIDs []uint64) ([]byte, error) {
	files, err := s.generate(ctx, tableIDs)
	if err != nil {
		return nil, err
	}
	return zipFiles(files)
}

// generate 配置行 → 映射 → 引擎。逐表聚合错误（spec §6：一轮改配置能解决的不逼跑多次）。
func (s *codegenService) generate(ctx context.Context, tableIDs []uint64) ([]cgmodel.GeneratedFile, error) {
	if len(tableIDs) == 0 {
		return nil, fmt.Errorf("%w: empty table_ids", cgmodel.ErrTableInvalid)
	}
	var errs []error
	tables := make([]*cgmodel.Table, 0, len(tableIDs))
	for _, id := range tableIDs {
		t, cols, err := s.GetTable(ctx, id)
		if err != nil {
			errs = append(errs, fmt.Errorf("table %d: %w", id, err))
			continue
		}
		tbl, err := TableFromConfig(t, cols)
		if err != nil {
			errs = append(errs, fmt.Errorf("table %s: %w", t.Name, err))
			continue
		}
		tables = append(tables, tbl)
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return codegen.Generate(tables, codegen.Options{UnitTestEnable: s.unitTestEnable})
}

// zipFiles 纯内存打包；entry 名 = GeneratedFile.Path（引擎产出的仓库相对路径，恒正斜杠）。
func zipFiles(files []cgmodel.GeneratedFile) ([]byte, error) {
	buf := &bytes.Buffer{}
	w := zip.NewWriter(buf)
	for _, f := range files {
		fw, err := w.Create(f.Path)
		if err != nil {
			return nil, err
		}
		if _, err := fw.Write(f.Content); err != nil {
			return nil, err
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Sync 重读元数据并按 spec §9.4 分类：新列全关插入、消失列标 deprecated、
// 快照刷新不碰配置、重现恢复。整个落库走 repo.ApplySync 单事务。
func (s *codegenService) Sync(ctx context.Context, tableID uint64) (*model.SyncResult, error) {
	t, err := s.repo.GetTable(ctx, tableID)
	if err != nil {
		return nil, fmt.Errorf("%w: table %d not imported", cgmodel.ErrTableNotFound, tableID)
	}
	meta, err := s.reader.ReadTable(ctx, t.Name)
	if err != nil {
		return nil, err
	}
	tbl, err := builder.Build(meta, s.opts)
	if err != nil {
		return nil, err
	}
	existing, err := s.repo.ListColumns(ctx, tableID)
	if err != nil {
		return nil, err
	}
	byName := make(map[string]*model.CodegenColumn, len(existing))
	for _, c := range existing {
		byName[c.Name] = c
	}
	// 快照 Type 取元数据 ColumnType（model.Column 无 DBType，按列名联表）。
	metaByName := make(map[string]*metadata.ColumnMeta, len(meta.Columns))
	for i := range meta.Columns {
		metaByName[meta.Columns[i].Name] = &meta.Columns[i]
	}

	diff := model.SyncDiff{}
	res := &model.SyncResult{}
	maxSort := -1
	for _, c := range existing {
		if c.SortOrder > maxSort {
			maxSort = c.SortOrder
		}
	}

	seen := make(map[string]bool, len(tbl.Columns))
	for _, gc := range tbl.Columns {
		seen[gc.Name] = true
		cm := metaByName[gc.Name]
		if cm == nil {
			return nil, fmt.Errorf("%w: %s: column %q missing in metadata", cgmodel.ErrColumnInvalid, meta.Name, gc.Name)
		}
		cur, ok := byName[gc.Name]
		if !ok {
			// 新列：全部开关默认关闭（§9.4），sort 接在既有最大之后
			maxSort++
			diff.Added = append(diff.Added, &model.CodegenColumn{
				Name: gc.Name, Type: cm.ColumnType, Comment: gc.Comment,
				GoType: gc.GoType, JSONName: gc.JSONName,
				IsPK: gc.IsPK, AutoIncrement: gc.AutoIncrement, Nullable: gc.Nullable,
				HTMLType: "", ListEnable: false, FormEnable: false, QueryEnable: false,
				QueryOperation: "eq", SortOrder: maxSort, Deprecated: false,
			})
			res.Added++
			continue
		}
		// 快照对比：命中差异或从 deprecated 复活 → Refresh（配置字段不进 diff）
		changed := cur.Type != cm.ColumnType || cur.Comment != gc.Comment ||
			cur.GoType != gc.GoType || cur.JSONName != gc.JSONName ||
			cur.IsPK != gc.IsPK || cur.AutoIncrement != gc.AutoIncrement ||
			cur.Nullable != gc.Nullable
		if changed || cur.Deprecated {
			refresh := *cur
			refresh.Type = cm.ColumnType
			refresh.Comment = gc.Comment
			refresh.GoType = gc.GoType
			refresh.JSONName = gc.JSONName
			refresh.IsPK = gc.IsPK
			refresh.AutoIncrement = gc.AutoIncrement
			refresh.Nullable = gc.Nullable
			refresh.Deprecated = false
			diff.Refresh = append(diff.Refresh, &refresh)
			res.Updated++
		} else {
			res.Unchanged++
		}
	}
	for _, c := range existing {
		if !seen[c.Name] {
			// §9.4：只对 deprecated=0 的行标 deprecated；已标过的本轮无变化。
			if c.Deprecated {
				res.Unchanged++
				continue
			}
			diff.Deprecated = append(diff.Deprecated, c.ID)
			res.Deprecated++
		}
	}

	if len(diff.Added) > 0 || len(diff.Refresh) > 0 || len(diff.Deprecated) > 0 ||
		t.TableComment != meta.Comment {
		if err := s.repo.ApplySync(ctx, tableID, meta.Comment, diff); err != nil {
			return nil, err
		}
	}
	return res, nil
}

var identRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// goExportedRe：class_name 是生成物的 Go 类型名，必须是导出标识符——
// 含 `-`/`.`/小写开头的值会让生成物编译失败。
var goExportedRe = regexp.MustCompile(`^[A-Z][A-Za-z0-9_]*$`)

// validateIdent 表名只允许小写 snake——它是拼进 SQL/文件名的输入面。
func validateIdent(name string) error {
	if !identRe.MatchString(name) || len(name) > 128 {
		return errs.New(errs.CodeParamInvalid, fmt.Errorf("invalid table name %q", name))
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
