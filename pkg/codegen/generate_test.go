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
	"regexp"
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

// treeFixtureMeta：树表 fixture（spec §10.1 固定约定 parent_id + name）。
// 表名 codegen_demo_menus → 实体 CodegenDemoMenu、snake codegen_demo_menu，
// 与单表 fixture（codegen_demo_item）不同名，可同包共存。
func treeFixtureMeta() *metadata.TableMeta {
	return &metadata.TableMeta{
		Schema:  "alexgo",
		Name:    "codegen_demo_menus",
		Comment: "codegen demo tree",
		Columns: []metadata.ColumnMeta{
			{Name: "id", DataType: "bigint", ColumnType: "bigint unsigned", Key: "PRI", Extra: "auto_increment", Comment: "主键", Ordinal: 1},
			{Name: "parent_id", DataType: "bigint", ColumnType: "bigint unsigned", Comment: "父级ID", Ordinal: 2},
			{Name: "name", DataType: "varchar", ColumnType: "varchar(64)", Comment: "名称", Ordinal: 3},
			{Name: "remark", DataType: "varchar", ColumnType: "varchar(255)", Nullable: true, Comment: "备注", Ordinal: 4},
			{Name: "created_at", DataType: "datetime", ColumnType: "datetime", Comment: "创建时间", Ordinal: 5},
			{Name: "tenant_id", DataType: "bigint", ColumnType: "bigint unsigned", Comment: "租户", Ordinal: 6},
		},
	}
}

// buildTreeFixture 树表 fixture（TemplateType=2）。
func buildTreeFixture(t *testing.T) *model.Table {
	t.Helper()
	tbl, err := builder.Build(treeFixtureMeta(), builder.Options{Module: "order", TemplateType: model.TemplateTypeTree})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return tbl
}

// dropColumns 返回去掉指定列的 meta 拷贝（不改原 meta）。
func dropColumns(meta *metadata.TableMeta, names ...string) *metadata.TableMeta {
	drop := map[string]bool{}
	for _, n := range names {
		drop[n] = true
	}
	out := *meta
	out.Columns = out.Columns[:0:0]
	for _, c := range meta.Columns {
		if !drop[c.Name] {
			out.Columns = append(out.Columns, c)
		}
	}
	return &out
}

// TestGenerate_TreeConvention 钉 spec §10.1：树表缺 parent_id/name 必须报
// ErrColumnInvalid，且校验先于 Render——否则树族未实现的 ErrTemplateMissing
// 会掩盖「缺列」这个真因（Review Focus ①）。
func TestGenerate_TreeConvention(t *testing.T) {
	cases := []struct {
		name    string
		meta    *metadata.TableMeta
		wantMsg string
	}{
		{"缺 parent_id", dropColumns(treeFixtureMeta(), "parent_id"), "parent_id"},
		{"缺 name", dropColumns(treeFixtureMeta(), "name"), "name"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tbl, err := builder.Build(tc.meta, builder.Options{Module: "order", TemplateType: model.TemplateTypeTree})
			if err != nil {
				t.Fatalf("Build: %v", err)
			}
			_, err = codegen.Generate([]*model.Table{tbl}, codegen.Options{UnitTestEnable: true})
			if !errors.Is(err, model.ErrColumnInvalid) {
				t.Fatalf("err = %v, want ErrColumnInvalid", err)
			}
			if errors.Is(err, model.ErrTemplateMissing) {
				t.Fatalf("校验必须先于 Render，ErrTemplateMissing 掩盖了缺列: %v", err)
			}
			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Fatalf("err = %q, 应点名缺列 %q", err, tc.wantMsg)
			}
		})
	}
}

func TestGenerate_Golden(t *testing.T) {
	files, err := codegen.Generate([]*model.Table{buildFixture(t)}, codegen.Options{UnitTestEnable: true})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(files) != 14 {
		t.Fatalf("files = %d, want 14: %+v", len(files), pathsOf(files))
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
	if len(files) != 12 {
		t.Fatalf("files = %d, want 12 (no service_test)", len(files))
	}
	for _, f := range files {
		if strings.HasSuffix(f.Path, "_test.go") {
			t.Errorf("unexpected test file %s", f.Path)
		}
		if strings.HasSuffix(f.Path, ".test.ts") {
			t.Errorf("unexpected front test file %s", f.Path)
		}
	}
}

func TestGenerate_AggregatesErrors(t *testing.T) {
	badNoCols := &model.Table{TableName: "t1", Module: "m", ClassName: "T1",
		TemplateType: model.TemplateTypeSingle, FrontType: model.FrontTypeVben5Antd}
	badType := buildFixture(t)
	badType.TemplateType = model.TemplateTypeMainSub // M6 未实现 → ErrTemplateMissing（M5 起 tree 已交付，不能再当缺模板载体）
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
	if len(files) != 28 {
		t.Fatalf("files = %d, want 28: %+v", len(files), pathsOf(files))
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

// 回归钉（M4 Review Focus ④）：JSONName="-" 的列不得出现在任何 front 产物中。
func TestGenerate_FrontOmitsHiddenColumns(t *testing.T) {
	files, err := codegen.Generate([]*model.Table{buildFixture(t)}, codegen.Options{UnitTestEnable: true})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	found := 0
	for _, f := range files {
		if !strings.Contains(f.Path, "apps/web-antd/") {
			continue
		}
		found++
		for _, hidden := range []string{"password", "tenant_id"} {
			if strings.Contains(string(f.Content), hidden) {
				t.Errorf("%s leaks %q", f.Path, hidden)
			}
		}
	}
	if found < 3 {
		t.Fatalf("front files = %d, want >= 3", found)
	}
}

// 回归钉（M4 Review Focus ③）：front 产物不得残留未翻译的 Go 动作。
// [[ = front delims 残留；{{ 后接 . - 或 Go 关键字 = 用错 delims 的 Go 动作。
// Vue 插值 {{ f.value }}（空格+标识符）与 TS 对象字面量 { ... } 不在命中集。
func TestGenerate_FrontNoDelimResidue(t *testing.T) {
	files, err := codegen.Generate([]*model.Table{buildFixture(t)}, codegen.Options{UnitTestEnable: true})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	re := regexp.MustCompile(`\[\[|\{\{\s*([.\-]|(range|if|else|end|with|define|template)\b)`)
	for _, f := range files {
		if !strings.Contains(f.Path, "apps/web-antd/") {
			continue
		}
		if loc := re.FindIndex(f.Content); loc != nil {
			off := loc[0]
			lo, hi := off-40, off+60
			if lo < 0 {
				lo = 0
			}
			if hi > len(f.Content) {
				hi = len(f.Content)
			}
			t.Errorf("%s residue at %d: …%s…", f.Path, off, f.Content[lo:hi])
		}
	}
}

// TestGenerate_FrontFixturesToDisk 把 fixture 的 front 产物写进 web-antd 真实路径，
// 供 vue-tsc/vitest 做类型门禁（T5/T6 防波堤）。默认 skip：
//
//	CODEGEN_FRONT_FIXTURE=1 go test ./pkg/codegen -run FrontFixtures
//
// 写盘方负责随后删除两个目录（见计划 T5 Step 5/T6 Step 5）。
func TestGenerate_FrontFixturesToDisk(t *testing.T) {
	if os.Getenv("CODEGEN_FRONT_FIXTURE") != "1" {
		t.Skip("set CODEGEN_FRONT_FIXTURE=1 to write web-antd fixtures")
	}
	files, err := codegen.Generate([]*model.Table{buildFixture(t)}, codegen.Options{UnitTestEnable: true})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	written := 0
	for _, f := range files {
		if !strings.HasPrefix(f.Path, "apps/web-antd/") {
			continue
		}
		dst := filepath.Join("..", "..", filepath.FromSlash(f.Path))
		if mkErr := os.MkdirAll(filepath.Dir(dst), 0o755); mkErr != nil {
			t.Fatalf("mkdir %s: %v", dst, mkErr)
		}
		if wErr := os.WriteFile(dst, f.Content, 0o644); wErr != nil {
			t.Fatalf("write %s: %v", dst, wErr)
		}
		written++
	}
	if written < 6 {
		t.Fatalf("written front files = %d, want >= 6", written)
	}
	t.Logf("wrote %d fixture files under apps/web-antd/src (clean up after typecheck)", written)
}

func pathsOf(files []model.GeneratedFile) []string {
	out := make([]string, len(files))
	for i, f := range files {
		out[i] = f.Path
	}
	return out
}
