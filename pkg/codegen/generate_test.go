// pkg/codegen/generate_test.go
package codegen_test

import (
	"bytes"
	"errors"
	"flag"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"alexGo-cloud/pkg/codegen"
	"alexGo-cloud/pkg/codegen/builder"
	"alexGo-cloud/pkg/codegen/metadata"
	"alexGo-cloud/pkg/codegen/model"
)

var update = flag.Bool("update", false, "rewrite golden files")

func fixtureMeta() *metadata.TableMeta {
	return &metadata.TableMeta{
		Schema:  "alexgo",
		Name:    "codegen_demo_items",
		Comment: "codegen demo",
		Columns: []metadata.ColumnMeta{
			{Name: "id", DataType: "bigint", ColumnType: "bigint unsigned", Key: "PRI", Extra: "auto_increment", Comment: "主键", Ordinal: 1},
			{Name: "name", DataType: "varchar", ColumnType: "varchar(64)", Comment: "名称", Ordinal: 2},
			{Name: "price", DataType: "decimal", ColumnType: "decimal(10,2)", Comment: "价格", Ordinal: 3},
			{Name: "status", DataType: "tinyint", ColumnType: "tinyint(1)", Comment: "状态", Ordinal: 4},
			{Name: "remark", DataType: "varchar", ColumnType: "varchar(255)", Nullable: true, Comment: "备注", Ordinal: 5},
			{Name: "password", DataType: "varchar", ColumnType: "varchar(128)", Comment: "密码", Ordinal: 6},
			{Name: "created_at", DataType: "datetime", ColumnType: "datetime", Comment: "创建时间", Ordinal: 7},
			{Name: "tenant_id", DataType: "bigint", ColumnType: "bigint unsigned", Comment: "租户", Ordinal: 8},
		},
	}
}

func buildFixture(t *testing.T) *model.Table {
	t.Helper()
	tbl, err := builder.Build(fixtureMeta(), builder.Options{Module: "order"})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return tbl
}

// buildSecondFixture 与 buildFixture 同模块（order），表名不同 → 实体名不同。
func buildSecondFixture(t *testing.T) *model.Table {
	t.Helper()
	meta := fixtureMeta()
	meta.Name = "codegen_demo_notes"
	meta.Comment = "codegen demo note"
	tbl, err := builder.Build(meta, builder.Options{Module: "order"})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return tbl
}

func TestGenerate_Golden(t *testing.T) {
	files, err := codegen.Generate([]*model.Table{buildFixture(t)}, codegen.Options{UnitTestEnable: true})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(files) != 7 {
		t.Fatalf("files = %d, want 7: %+v", len(files), pathsOf(files))
	}
	for _, f := range files {
		golden := filepath.Join("testdata", "golden", strings.ReplaceAll(f.Path, "/", "_"))
		if *update {
			if err := os.MkdirAll("testdata/golden", 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(golden, f.Content, 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(golden)
		if err != nil {
			t.Fatalf("read golden %s: %v (run: go test ./pkg/codegen -run Golden -update)", golden, err)
		}
		if !bytes.Equal(f.Content, want) {
			t.Errorf("mismatch %s\n--- got ---\n%s\n--- want ---\n%s", f.Path, f.Content, want)
		}
	}
	// 回归钉：敏感字段必须是 json:"-"
	var modelFile []byte
	for _, f := range files {
		if strings.HasSuffix(f.Path, "/model/codegen_demo_item.go") {
			modelFile = f.Content
		}
	}
	if !bytes.Contains(modelFile, []byte(`json:"-"`)) {
		t.Error("model output missing sensitive json:\"-\"")
	}
}

func TestGenerate_NoUnitTest(t *testing.T) {
	files, err := codegen.Generate([]*model.Table{buildFixture(t)}, codegen.Options{UnitTestEnable: false})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(files) != 6 {
		t.Fatalf("files = %d, want 6 (no service_test)", len(files))
	}
	for _, f := range files {
		if strings.HasSuffix(f.Path, "_test.go") {
			t.Errorf("unexpected test file %s", f.Path)
		}
	}
}

func TestGenerate_AggregatesErrors(t *testing.T) {
	badNoCols := &model.Table{TableName: "t1", Module: "m", ClassName: "T1",
		TemplateType: model.TemplateTypeSingle, FrontType: model.FrontTypeVben5Antd}
	badType := buildFixture(t)
	badType.TemplateType = model.TemplateTypeTree // M1 未实现 → ErrTemplateMissing
	_, err := codegen.Generate([]*model.Table{badNoCols, badType}, codegen.Options{UnitTestEnable: true})
	if err == nil {
		t.Fatal("Generate should fail")
	}
	if !errors.Is(err, model.ErrColumnInvalid) {
		t.Errorf("err missing ErrColumnInvalid: %v", err)
	}
	if !errors.Is(err, model.ErrTemplateMissing) {
		t.Errorf("err missing ErrTemplateMissing: %v", err)
	}
}

func TestGenerate_Collide(t *testing.T) {
	a := buildFixture(t)
	b := buildFixture(t) // 同表两次 → 同路径
	_, err := codegen.Generate([]*model.Table{a, b}, codegen.Options{UnitTestEnable: true})
	if !errors.Is(err, model.ErrPathCollide) {
		t.Fatalf("err = %v, want ErrPathCollide", err)
	}
}

func TestGenerate_SortedOutput(t *testing.T) {
	files, err := codegen.Generate([]*model.Table{buildFixture(t)}, codegen.Options{UnitTestEnable: true})
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(files); i++ {
		if files[i-1].Path >= files[i].Path {
			t.Fatalf("unsorted: %s >= %s", files[i-1].Path, files[i].Path)
		}
	}
}

// 回归钉（B1）：同模块双表生成的两份 service_test 同属 package service，
// 包级 var 必须按实体限定（err<Entity>RecordNotFound），否则重声明
// → 生成代码 go vet/test 失败。
func TestGenerate_TwoTablesSameModule_NoDuplicateTopLevelVars(t *testing.T) {
	files, err := codegen.Generate([]*model.Table{buildFixture(t), buildSecondFixture(t)}, codegen.Options{UnitTestEnable: true})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(files) != 14 {
		t.Fatalf("files = %d, want 14: %+v", len(files), pathsOf(files))
	}
	var testFiles []model.GeneratedFile
	for _, f := range files {
		if strings.HasSuffix(f.Path, "_test.go") {
			testFiles = append(testFiles, f)
		}
	}
	if len(testFiles) != 2 {
		t.Fatalf("service_test files = %d, want 2: %+v", len(testFiles), pathsOf(files))
	}
	seen := map[string]string{} // 顶层 var 名 → 所在文件
	for _, f := range testFiles {
		file, err := parser.ParseFile(token.NewFileSet(), f.Path, f.Content, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", f.Path, err)
		}
		for _, decl := range file.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.VAR {
				continue
			}
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, name := range vs.Names {
					if prev, dup := seen[name.Name]; dup {
						t.Errorf("duplicate top-level var %s across same-package files %s and %s", name.Name, prev, f.Path)
					}
					seen[name.Name] = f.Path
				}
			}
		}
	}
}

func pathsOf(files []model.GeneratedFile) []string {
	out := make([]string, len(files))
	for i, f := range files {
		out[i] = f.Path
	}
	return out
}
