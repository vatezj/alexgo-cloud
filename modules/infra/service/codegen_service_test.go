package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/modules/infra/model"
	"alexGo-cloud/modules/infra/repository"
	"alexGo-cloud/pkg/codegen/builder"
	"alexGo-cloud/pkg/codegen/metadata"
	cgmodelErr "alexGo-cloud/pkg/codegen/model"
)

// fakeReader 是内存 metadata.MetadataReader：支持外部 mutate 后 Sync。
type fakeReader struct {
	tables map[string]*metadata.TableMeta
	order  []string
}

func (f *fakeReader) ListTables(_ context.Context) ([]string, error) {
	return append([]string(nil), f.order...), nil
}

func (f *fakeReader) ReadTable(_ context.Context, name string) (*metadata.TableMeta, error) {
	t, ok := f.tables[name]
	if !ok {
		return nil, cgmodelErr.ErrTableNotFound
	}
	// 返回副本，防调用方改内部态
	cp := *t
	cp.Columns = append([]metadata.ColumnMeta(nil), t.Columns...)
	return &cp, nil
}

// col 构造元数据列：dataType 传完整类型（如 "varchar(255)"），
// DataType 取基准类型（builder 按 DataType 映射），ColumnType 存完整类型。
func col(name, dataType, key, extra string) metadata.ColumnMeta {
	base := dataType
	if i := strings.Index(dataType, "("); i > 0 {
		base = dataType[:i]
	}
	return metadata.ColumnMeta{
		Name: name, DataType: base, ColumnType: dataType, Key: key, Extra: extra,
	}
}

func newFixtureReader() *fakeReader {
	return &fakeReader{
		tables: map[string]*metadata.TableMeta{
			"order_items": {
				Schema: "alexgo", Name: "order_items", Comment: "订单明细",
				Columns: []metadata.ColumnMeta{
					{Name: "id", DataType: "bigint", ColumnType: "bigint unsigned", Key: "PRI", Extra: "auto_increment", Ordinal: 1},
					{Name: "name", DataType: "varchar", ColumnType: "varchar(64)", Comment: "名称", Ordinal: 2},
					{Name: "qty", DataType: "int", ColumnType: "int", Comment: "数量", Ordinal: 3},
				},
			},
		},
		order: []string{"order_items"},
	}
}

func newSvc(t *testing.T) (CodegenService, repository.CodegenRepository, *fakeReader) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.CodegenTable{}, &model.CodegenColumn{}); err != nil {
		t.Fatal(err)
	}
	repo := repository.NewCodegenRepository(db)
	r := newFixtureReader()
	svc := NewCodegenService(r, repo, builder.Options{Module: "infra"})
	return svc, repo, r
}

// 导入 happy path：快照 3 列、主键标记、类名生成。
func TestImportTable_HappyPath(t *testing.T) {
	svc, _, _ := newSvc(t)
	got, err := svc.ImportTable(context.Background(), "order_items")
	if err != nil {
		t.Fatalf("ImportTable: %v", err)
	}
	if got.Name != "order_items" || got.ClassName == "" {
		t.Errorf("got %+v", got)
	}
	_, cols, err := svc.GetTable(context.Background(), got.ID)
	if err != nil || len(cols) != 3 {
		t.Fatalf("cols=%d err=%v", len(cols), err)
	}
	if !cols[0].IsPK {
		t.Error("id 应标记 is_pk")
	}
	// 重复导入拒绝
	if _, err := svc.ImportTable(context.Background(), "order_items"); err == nil {
		t.Error("重复导入应报错")
	}
}

// 四类拒绝（Review Focus #3）：每类 errors.Is 精确断言 + 0 新行。
func TestImportTable_Rejections(t *testing.T) {
	cases := []struct {
		name    string
		table   string // reader 中的表名
		mutate  func(*fakeReader)
		wantErr error
	}{
		{
			name:    "库中不存在",
			table:   "ghost_table",
			mutate:  func(r *fakeReader) {},
			wantErr: cgmodelErr.ErrTableNotFound,
		},
		{
			name:  "无主键",
			table: "no_pk",
			mutate: func(r *fakeReader) {
				r.tables["no_pk"] = &metadata.TableMeta{Name: "no_pk", Columns: []metadata.ColumnMeta{{Name: "a", DataType: "int", ColumnType: "int"}}}
				r.order = append(r.order, "no_pk")
			},
			wantErr: cgmodelErr.ErrColumnInvalid,
		},
		{
			name:  "复合主键",
			table: "composite_pk",
			mutate: func(r *fakeReader) {
				r.tables["composite_pk"] = &metadata.TableMeta{Name: "composite_pk", Columns: []metadata.ColumnMeta{
					{Name: "a", DataType: "int", ColumnType: "int", Key: "PRI"},
					{Name: "b", DataType: "int", ColumnType: "int", Key: "PRI"},
				}}
				r.order = append(r.order, "composite_pk")
			},
			wantErr: cgmodelErr.ErrColumnInvalid,
		},
		{
			name:  "主键非 id",
			table: "uuid_pk",
			mutate: func(r *fakeReader) {
				r.tables["uuid_pk"] = &metadata.TableMeta{Name: "uuid_pk", Columns: []metadata.ColumnMeta{
					{Name: "order_no", DataType: "varchar", ColumnType: "varchar(32)", Key: "PRI"},
				}}
				r.order = append(r.order, "uuid_pk")
			},
			wantErr: cgmodelErr.ErrColumnInvalid,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, repo, reader := newSvc(t)
			tc.mutate(reader)
			_, err := svc.ImportTable(context.Background(), tc.table)
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("err = %v, want errors.Is(%v)", err, tc.wantErr)
			}
			_, total, listErr := repo.ListTables(context.Background(), 1, 100)
			if listErr != nil || total != 0 {
				t.Errorf("拒绝后仍有落库: total=%d err=%v", total, listErr)
			}
		})
	}
}

// Sync 前置：未导入的表 → ErrTableNotFound（不静默建行）。
func TestSync_NotImported_Rejected(t *testing.T) {
	svc, _, _ := newSvc(t)
	if _, err := svc.Sync(context.Background(), 4242); !errors.Is(err, cgmodelErr.ErrTableNotFound) {
		t.Errorf("err = %v, want ErrTableNotFound", err)
	}
}

// 评审 Important#1：部分 body 更新不得清空行。
// gorm Save 全量覆盖含零值——PUT {"remark":"x"} 曾把 table_name/module/
// class_name/created_at 全部清零，table_name 一空 Sync 永久失效。
// 服务端必须合并进既有行，且 table_name 不接受 body 修改（Sync 定位键）。
func TestUpdateTable_PartialBodyKeepsRow(t *testing.T) {
	svc, _, _ := newSvc(t)
	ctx := context.Background()
	imported, err := svc.ImportTable(ctx, "order_items")
	if err != nil {
		t.Fatal(err)
	}

	// 模拟前端只带 id + remark 的部分 body
	if err := svc.UpdateTable(ctx, &model.CodegenTable{ID: imported.ID, Remark: "edited"}); err != nil {
		t.Fatalf("UpdateTable: %v", err)
	}

	got, _, err := svc.GetTable(ctx, imported.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "order_items" {
		t.Errorf("table_name 被清: %q", got.Name)
	}
	if got.Module != "infra" || got.ClassName != imported.ClassName || got.BusinessName != imported.BusinessName {
		t.Errorf("配置字段被清: module=%q class=%q business=%q", got.Module, got.ClassName, got.BusinessName)
	}
	if got.CreatedAt.IsZero() {
		t.Error("created_at 被清零")
	}
	if got.Remark != "edited" {
		t.Errorf("remark = %q, want edited", got.Remark)
	}

	// table_name 即使随 body 传来也不得改写（Sync 用它定位物理表）
	if err := svc.UpdateTable(ctx, &model.CodegenTable{ID: imported.ID, Name: "other_table", Remark: "r2"}); err != nil {
		t.Fatalf("UpdateTable rename attempt: %v", err)
	}
	got, _, _ = svc.GetTable(ctx, imported.ID)
	if got.Name != "order_items" {
		t.Errorf("table_name 被 body 改写: %q", got.Name)
	}
}

// 评审 Important#2：坏配置不得入库（Focus #3 的"非法 module / 类名冲突"两缺口）。
func TestUpdateTable_RejectsBadConfig(t *testing.T) {
	svc, _, _ := newSvc(t)
	ctx := context.Background()
	a, err := svc.ImportTable(ctx, "order_items")
	if err != nil {
		t.Fatal(err)
	}

	// ① 非法 module（路径穿越/大写/空）——module 拼进生成物路径
	bad := *a
	bad.Module = "../../etc"
	if err := svc.UpdateTable(ctx, &bad); !errors.Is(err, cgmodelErr.ErrTableInvalid) {
		t.Errorf("module 路径穿越: err = %v, want ErrTableInvalid", err)
	}
	// ② 非法 class_name（Go 导出标识符之外）
	bad = *a
	bad.ClassName = "not-a-class"
	if err := svc.UpdateTable(ctx, &bad); !errors.Is(err, cgmodelErr.ErrTableInvalid) {
		t.Errorf("class_name 非法: err = %v, want ErrTableInvalid", err)
	}
	// ③ 类名冲突：两张表同 module 同 class_name → 生成物同包重声明
	b2 := *a
	b2.ID = a.ID // 同行不冲突
	b2.ClassName = a.ClassName
	if err := svc.UpdateTable(ctx, &b2); err != nil {
		t.Errorf("同类名改回自身应允许: %v", err)
	}
	// ④ 入库回读确认坏 module 没落库
	got, _, _ := svc.GetTable(ctx, a.ID)
	if got.Module == "../../etc" || got.ClassName == "not-a-class" {
		t.Errorf("坏配置落库: module=%q class=%q", got.Module, got.ClassName)
	}
}

// 类名冲突检测需要第二行：直接用 repo 造（模拟另一张已导入表）。
func TestUpdateTable_RejectsClassConflict(t *testing.T) {
	svc, repo, _ := newSvc(t)
	ctx := context.Background()
	a, err := svc.ImportTable(ctx, "order_items")
	if err != nil {
		t.Fatal(err)
	}
	other := &model.CodegenTable{Name: "order_notes", Module: "infra",
		BusinessName: "order_note", ClassName: "OrderNote"}
	if err := repo.CreateTable(ctx, other, nil); err != nil {
		t.Fatal(err)
	}
	bad := *a
	bad.ClassName = "OrderNote" // 与 other 冲突
	if err := svc.UpdateTable(ctx, &bad); !errors.Is(err, cgmodelErr.ErrTableInvalid) {
		t.Errorf("类名冲突: err = %v, want ErrTableInvalid", err)
	}
}

// 重复导入必须包哨兵（评审：当前裸 fmt.Errorf，前端无法 errors.Is 归类）。
func TestImportTable_DuplicateSentinel(t *testing.T) {
	svc, _, _ := newSvc(t)
	ctx := context.Background()
	if _, err := svc.ImportTable(ctx, "order_items"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ImportTable(ctx, "order_items"); !errors.Is(err, cgmodelErr.ErrTableInvalid) {
		t.Errorf("重复导入: err = %v, want ErrTableInvalid", err)
	}
}
