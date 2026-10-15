// tools/codegen/main.go
package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"

	inframodel "alexGo-cloud/modules/system/infra/model"
	"alexGo-cloud/modules/system/infra/repository"
	"alexGo-cloud/modules/system/infra/service"
	"alexGo-cloud/pkg/codegen/builder"
	"alexGo-cloud/pkg/codegen/metadata"
	"alexGo-cloud/pkg/codegen/model"
	"alexGo-cloud/pkg/config"
)

func main() {
	module := flag.String("module", "", "target module name (e.g. order)")
	tables := flag.String("tables", "", "comma-separated table names (empty = all in schema)")
	dsnFlag := flag.String("dsn", "", "mysql dsn (default: config database.dsn / DB_DSN)")
	schemaFlag := flag.String("schema", "", "mysql schema (default: from dsn)")
	out := flag.String("out", ".", "output root dir (repo root)")
	force := flag.Bool("force", false, "overwrite existing files")
	dryRun := flag.Bool("dry-run", false, "print planned files, do not write")
	noTests := flag.Bool("no-tests", false, "skip generating unit test skeletons")
	importFlag := flag.Bool("import", false, "先现场读元数据导入配置表（需 -module/-tables 或 -tables 留空=全库表），再从配置表生成")
	flag.Parse()

	cfg, _ := config.LoadGlobalConfig()
	dsn := *dsnFlag
	if dsn == "" && cfg != nil {
		dsn = cfg.Database.DSN
	}
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "missing dsn: pass --dsn or set DB_DSN or config database.dsn")
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 原样保留（从旧位置上移）：单测骨架开关
	unitTest := true
	if cfg != nil {
		unitTest = cfg.Codegen.UnitTestEnable
	}
	if *noTests {
		unitTest = false
	}

	// 配置表所在库（与元数据同 DSN）；gorm 只用于 config 读写
	gdb, err := gorm.Open(gormmysql.Open(dsn), &gorm.Config{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "open config db: %v\n", err)
		os.Exit(1)
	}
	repo := repository.NewCodegenRepository(gdb)

	// -import：现场读元数据写入配置表（已导入跳过，幂等），随后统一走配置模式
	if *importFlag {
		if *module == "" {
			fmt.Fprintln(os.Stderr, "-import 需要 --module（导入行要写目标模块），如 --module=order --tables=t1 --import")
			os.Exit(2)
		}
		tableList, err := parseTables(*tables)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		sdb, err := sql.Open("mysql", dsn)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		defer sdb.Close()
		if err := sdb.PingContext(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", model.ErrMetadataUnreachable, err)
			os.Exit(1)
		}
		schema := *schemaFlag
		if schema == "" {
			schema = schemaFromDSN(dsn)
		}
		if schema == "" {
			fmt.Fprintln(os.Stderr, "cannot determine schema; pass --schema or include db name in dsn")
			os.Exit(2)
		}
		reader := metadata.NewMySQLReader(sdb, schema)
		if len(tableList) == 0 {
			// 与旧 dbgen 语义对齐：-tables 留空 = schema 全部表
			tableList, err = reader.ListTables(ctx)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
		}
		svc := service.NewCodegenService(reader, repo, builder.Options{Module: *module}, unitTest)
		done, err := repo.ImportedTableNames(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		imported := make(map[string]bool, len(done))
		for _, n := range done {
			imported[n] = true
		}
		var importErrs []error
		skipped := 0
		for _, name := range tableList {
			if imported[name] {
				skipped++
				continue
			}
			if _, err := svc.ImportTable(ctx, name); err != nil {
				importErrs = append(importErrs, fmt.Errorf("import %s: %w", name, err))
			}
		}
		if len(importErrs) > 0 {
			for _, e := range importErrs {
				fmt.Fprintf(os.Stderr, "%v\n", e)
			}
			fmt.Fprintf(os.Stderr, "导入失败 %d 个，未生成任何文件\n", len(importErrs))
			os.Exit(1)
		}
		fmt.Printf("导入 %d / 跳过 %d（已导入）\n", len(tableList)-skipped, skipped)
	}

	// 配置模式：过滤已导入表 → Preview（与后台同一渲染路径）
	all, err := repo.ListAllTables(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	selected, err := filterConfigTables(all, *module, *tables)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(2)
	}
	ids := make([]uint64, 0, len(selected))
	for _, t := range selected {
		ids = append(ids, t.ID)
	}
	// 配置模式不读元数据（显式 nopReader，防 nil 悬置）；生成路径与后台 preview 完全同源
	svc := service.NewCodegenService(nopReader{}, repo, builder.Options{}, unitTest)
	files, err := svc.Preview(ctx, ids)
	if err != nil {
		fmt.Fprintln(os.Stderr, err) // errors.Join 多行 = 逐条打印（spec §6）
		fmt.Fprintln(os.Stderr, "未生成任何文件")
		os.Exit(1)
	}

	if *dryRun {
		for _, f := range files {
			fmt.Println(f.Path)
		}
		fmt.Printf("dry-run：%d 个文件\n", len(files))
		return
	}
	created, skipped, err := writeFiles(*out, files, *force)
	if err != nil {
		fmt.Fprintf(os.Stderr, "写盘失败: %v（已写 %d，跳过 %d）\n", err, created, skipped)
		os.Exit(1)
	}
	fmt.Printf("生成 %d / 跳过 %d（已存在）/ 失败 0\n", created, skipped)
}

// nopReader：配置模式不读元数据的显式占位（M3 终审跟进票④）。
// 若生成路径意外触达 reader，立即报错而不是 nil 解引用 panic。
type nopReader struct{}

func (nopReader) ListTables(context.Context) ([]string, error) {
	return nil, errors.New("codegen CLI: 配置模式不读元数据（nopReader.ListTables 被意外调用）")
}

func (nopReader) ReadTable(context.Context, string) (*metadata.TableMeta, error) {
	return nil, errors.New("codegen CLI: 配置模式不读元数据（nopReader.ReadTable 被意外调用）")
}

func writeFiles(root string, files []model.GeneratedFile, force bool) (created, skipped int, err error) {
	for _, f := range files {
		dst := filepath.Join(root, filepath.FromSlash(f.Path))
		if !force {
			if _, statErr := os.Stat(dst); statErr == nil {
				skipped++
				continue
			}
		}
		if mkErr := os.MkdirAll(filepath.Dir(dst), 0o755); mkErr != nil {
			return created, skipped, mkErr
		}
		if wErr := os.WriteFile(dst, f.Content, 0o644); wErr != nil {
			return created, skipped, wErr
		}
		created++
	}
	return created, skipped, nil
}

func parseTables(s string) ([]string, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	var out []string
	for _, t := range strings.Split(s, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			out = append(out, t)
		}
	}
	return out, nil
}

// filterConfigTables 配置模式选表：module/tables 都是过滤器（空=不过滤）。
// 无匹配返回 error（带修复建议），绝不静默生成 0 个文件当成功。
func filterConfigTables(tables []*inframodel.CodegenTable, module, names string) ([]*inframodel.CodegenTable, error) {
	want, err := parseTables(names)
	if err != nil {
		return nil, err
	}
	wantSet := make(map[string]bool, len(want))
	for _, w := range want {
		wantSet[w] = true
	}
	var out []*inframodel.CodegenTable
	for _, t := range tables {
		if t == nil {
			continue
		}
		if module != "" && t.Module != module {
			continue
		}
		if len(wantSet) > 0 && !wantSet[t.Name] {
			continue
		}
		out = append(out, t)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf(
			"没有匹配的已导入表（--module=%q --tables=%q）：先在后台页面导入，或加 --import 现场导入",
			module, names,
		)
	}
	return out, nil
}

// schemaFromDSN 返回 DSN 中的库名；无库名时返回 ""。
// 用驱动的 ParseDSN 而非手写切串：unix socket、密码含特殊字符、query 含 / 均正确。
func schemaFromDSN(dsn string) string {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return ""
	}
	return cfg.DBName
}
