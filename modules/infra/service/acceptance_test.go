package service

import (
	"archive/zip"
	"bytes"
	"context"
	"strings"
	"testing"

	"alexGo-cloud/modules/infra/model"
)

// TestM2_Acceptance_ImportEditSync 是 spec §8 M2 验收链：
// 导入 → 改配置（query 开关+操作符）→ 库表加列删列改注释 → 同步 → diff 计数。
func TestM2_Acceptance_ImportEditSync(t *testing.T) {
	svc, _, reader := newSvc(t)
	ctx := context.Background()

	// 1) 导入
	tbl, err := svc.ImportTable(ctx, "order_items")
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	_, cols, _ := svc.GetTable(ctx, tbl.ID)
	if len(cols) != 3 {
		t.Fatalf("import cols=%d, want 3", len(cols))
	}

	// 2) 改配置：name 列 query 开、between
	var nameID uint64
	for _, c := range cols {
		if c.Name == "name" {
			nameID = c.ID
		}
	}
	if err := svc.SaveColumns(ctx, tbl.ID, []*model.CodegenColumn{{
		ID: nameID, TableID: tbl.ID, Name: "name",
		QueryEnable: true, QueryOperation: "between", HTMLType: "Select",
	}}); err != nil {
		t.Fatalf("save config: %v", err)
	}

	// 3) 库表变更：+remark 列、删 qty 列、改表注释
	m := reader.tables["order_items"]
	m.Columns = append(m.Columns[:2], col("remark", "varchar(255)", "", "")) // [id,name,remark]
	m.Comment = "订单明细v2"
	reader.tables["order_items"] = m

	// 4) 同步 → diff（§9.4）：Added=1(remark), Deprecated=1(qty),
	//    name 只改了配置→快照无变化→unchanged（配置变更不计 updated）, Unchanged=2(id,name)
	res, err := svc.Sync(ctx, tbl.ID)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if res.Added != 1 {
		t.Errorf("added=%d, want 1", res.Added)
	}
	if res.Deprecated != 1 {
		t.Errorf("deprecated=%d, want 1", res.Deprecated)
	}
	if res.Updated != 0 {
		t.Errorf("updated=%d, want 0（仅配置变化，快照未变）", res.Updated)
	}
	if res.Unchanged != 2 {
		t.Errorf("unchanged=%d, want 2 (id,name)", res.Unchanged)
	}

	// 5) 配置字段跨同步存活 + qty 行仍在（deprecated）+ 表注释刷新
	t2, cols2, err := svc.GetTable(ctx, tbl.ID)
	if err != nil {
		t.Fatal(err)
	}
	if t2.TableComment != "订单明细v2" {
		t.Errorf("table_comment=%q", t2.TableComment)
	}
	if len(cols2) != 4 {
		t.Fatalf("cols=%d, want 4 (id,name,qty,remark)", len(cols2))
	}
	for _, c := range cols2 {
		switch c.Name {
		case "name":
			if !c.QueryEnable || c.QueryOperation != "between" || c.HTMLType != "Select" {
				t.Errorf("配置丢失: %+v", c)
			}
		case "qty":
			if !c.Deprecated {
				t.Error("qty 应 deprecated")
			}
		case "remark":
			if c.ListEnable || c.FormEnable || c.QueryEnable {
				t.Errorf("新列开关应全关: %+v", c)
			}
		}
	}

	// 6) 再同步一次：全 unchanged、计数归零（幂等）
	res2, err := svc.Sync(ctx, tbl.ID)
	if err != nil {
		t.Fatal(err)
	}
	if res2.Added != 0 || res2.Updated != 0 || res2.Deprecated != 0 {
		t.Errorf("二次同步非幂等: %+v", res2)
	}
}

// TestM3_Acceptance_PreviewDownload 是 spec §8 M3 验收链（服务层）：
// 导入 → 预览（引擎纯内存渲染）→ ZIP（同一份渲染结果打包）。
func TestM3_Acceptance_PreviewDownload(t *testing.T) {
	svc, _, _ := newSvc(t)
	ctx := context.Background()

	tbl, err := svc.ImportTable(ctx, "order_items")
	if err != nil {
		t.Fatalf("import: %v", err)
	}

	files, err := svc.Preview(ctx, []uint64{tbl.ID})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if len(files) < 4 {
		t.Fatalf("files=%d, want >=4", len(files))
	}
	var paths []string
	for _, f := range files {
		// M4 起 preview 含 front 产物（apps/web-antd/ 前缀），server 仍落 modules/infra/。
		if !strings.HasPrefix(f.Path, "modules/infra/") &&
			!strings.HasPrefix(f.Path, "apps/web-antd/") {
			t.Errorf("path=%q, want modules/infra/ 或 apps/web-antd/ 前缀", f.Path)
		}
		if len(f.Content) == 0 {
			t.Errorf("%s content 为空", f.Path)
		}
		paths = append(paths, f.Path)
	}
	// unitTestEnable=true（newSvc 传入）→ 单测骨架必须在（不硬编码文件总数，模板增减不误伤）
	hasTest := false
	for _, p := range paths {
		if strings.HasSuffix(p, "_test.go") {
			hasTest = true
		}
	}
	if !hasTest {
		t.Errorf("paths=%v, 缺 *_test.go 骨架", paths)
	}

	zipBytes, err := svc.Download(ctx, []uint64{tbl.ID})
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	if len(zipBytes) < 4 || string(zipBytes[:2]) != "PK" {
		t.Fatalf("zip magic 错误（前 4 字节 %q）", zipBytes[:min(4, len(zipBytes))])
	}
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatalf("zip open: %v", err)
	}
	zipPaths := map[string]bool{}
	for _, zf := range zr.File {
		zipPaths[zf.Name] = true
	}
	if len(zr.File) != len(paths) {
		t.Errorf("zip entries=%d, want %d", len(zr.File), len(paths))
	}
	for _, p := range paths {
		if !zipPaths[p] {
			t.Errorf("zip 缺 entry: %s", p)
		}
	}
}
