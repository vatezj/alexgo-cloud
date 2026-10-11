//go:build integration

package main

import (
	"context"
	"database/sql"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/go-sql-driver/mysql"

	"alexGo-cloud/pkg/codegen"
	"alexGo-cloud/pkg/codegen/builder"
	"alexGo-cloud/pkg/codegen/metadata"
	"alexGo-cloud/pkg/codegen/model"
	"alexGo-cloud/pkg/config"
)

// randSuffix 随机表名后缀：上次运行崩溃留下的残留表不干扰本次
// （DROP IF EXISTS + CREATE 会悄悄吃掉残留数据/结构变化）。
func randSuffix() string {
	b := make([]byte, 6)
	for i := range b {
		b[i] = "abcdefghijklmnopqrstuvwxyz0123456789"[rand.Intn(36)]
	}
	return string(b)
}

// TestSmoke_RealTableGoBuild 是 M1 验收：真实表 → 生成 → 编译 → 跑生成的单测 → 清理。
// 需要本地 MySQL（docker compose 起）。DB 不可达时 Skip。
func TestSmoke_RealTableGoBuild(t *testing.T) {
	repoRoot := findRepoRoot(t)
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.LoadGlobalConfig()
	if err != nil || cfg == nil || cfg.Database.DSN == "" {
		t.Skipf("no dsn in config: %v", err)
	}

	ctx := context.Background()
	db, err := sql.Open("mysql", cfg.Database.DSN)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	// 注意：defer 在函数返回时先于 t.Cleanup 执行，会把连接提前关掉导致
	// 后续 DROP 清理报 "database is closed"。用 t.Cleanup 关闭并最先注册，
	// 依赖 LIFO 保证它在 DROP 表清理之后才运行。
	t.Cleanup(func() { _ = db.Close() })
	if err := db.PingContext(ctx); err != nil {
		t.Skipf("mysql unreachable: %v (先 docker compose up mysql)", err)
	}

	// 1. 建两张冒烟表（同模块，随机后缀防残留污染）
	// 第二张表回归双表同包重声明（B1）
	smokeTable := "codegen_smoke_item_" + randSuffix()
	smokeTable2 := "codegen_smoke_note_" + randSuffix()
	for _, name := range []string{smokeTable, smokeTable2} {
		mustExec(t, db, "DROP TABLE IF EXISTS `"+name+"`")
		mustExec(t, db, "CREATE TABLE `"+name+"`"+` (
  id bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  name varchar(64) NOT NULL COMMENT '名称',
  price decimal(10,2) NOT NULL DEFAULT '0.00' COMMENT '价格',
  status tinyint NOT NULL DEFAULT '0' COMMENT '状态',
  remark varchar(255) DEFAULT NULL COMMENT '备注',
  password varchar(128) DEFAULT NULL COMMENT '密码',
  tenant_id bigint unsigned NOT NULL DEFAULT '0' COMMENT '租户',
  created_at datetime NOT NULL COMMENT '创建时间',
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='codegen smoke'`)
	}
	t.Cleanup(func() {
		mustExec(t, db, "DROP TABLE IF EXISTS "+smokeTable)
		mustExec(t, db, "DROP TABLE IF EXISTS "+smokeTable2)
	})

	// 2. 读元数据 + 构建 + 生成（两张表 → 同一模块）
	schema := schemaFromDSN(cfg.Database.DSN)
	if schema == "" {
		t.Fatal("cannot parse schema from dsn")
	}
	var tables []*model.Table
	for _, name := range []string{smokeTable, smokeTable2} {
		meta, err := metadata.NewMySQLReader(db, schema).ReadTable(ctx, name)
		if err != nil {
			t.Fatalf("ReadTable %s: %v", name, err)
		}
		tbl, err := builder.Build(meta, builder.Options{Module: "order"})
		if err != nil {
			t.Fatalf("Build %s: %v", name, err)
		}
		tables = append(tables, tbl)
	}
	files, err := codegen.Generate(tables, codegen.Options{UnitTestEnable: true})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(files) != 14 {
		t.Fatalf("files = %d, want 14", len(files))
	}

	// 3. 只记录本次新建的路径，cleanup 只删自己的文件
	var newPaths []string
	for _, f := range files {
		if _, statErr := os.Stat(f.Path); statErr != nil {
			newPaths = append(newPaths, f.Path)
		}
	}
	t.Cleanup(func() {
		for _, p := range newPaths {
			_ = os.Remove(p)
		}
	})
	created, skipped, err := writeFiles(".", files, false)
	if err != nil {
		t.Fatalf("writeFiles: %v", err)
	}
	if created != len(newPaths) || skipped != len(files)-len(newPaths) {
		t.Fatalf("created=%d skipped=%d, newPaths=%d", created, skipped, len(newPaths))
	}

	// 4. 编译整个 order 模块（含生成文件）
	out, err := exec.Command("go", "build", "./modules/order/...").CombinedOutput()
	if err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	// 5. 跑两个实体生成的单测 + vet（vet 覆盖 _test.go 同包编译，防重声明回归）
	pattern := "Test(" + tables[0].ClassName + "|" + tables[1].ClassName + ")Service_CRUD"
	out, err = exec.Command("go", "test", "./modules/order/service/",
		"-run", pattern, "-count=1").CombinedOutput()
	if err != nil {
		t.Fatalf("generated test failed: %v\n%s", err, out)
	}
	out, err = exec.Command("go", "vet", "./modules/order/...").CombinedOutput()
	if err != nil {
		t.Fatalf("go vet: %v\n%s", err, out)
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatal("go.mod not found from cwd")
	return ""
}

func mustExec(t *testing.T, db *sql.DB, query string) {
	t.Helper()
	if _, err := db.Exec(query); err != nil {
		t.Fatalf("exec %q: %v", strings.TrimSpace(query[:min(40, len(query))]), err)
	}
}

// schemaFromDSN 复用 main.go 中的同名实现（同包），此处不重复定义。
