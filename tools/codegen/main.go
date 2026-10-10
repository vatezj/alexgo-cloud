// tools/codegen/main.go
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"alexGo-cloud/pkg/codegen"
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
	flag.Parse()

	if *module == "" {
		fmt.Fprintln(os.Stderr, "missing --module")
		flag.Usage()
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
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

	reader := metadata.NewMySQLReader(db, schema)

	tableList, err := parseTables(*tables)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if len(tableList) == 0 {
		tableList, err = reader.ListTables(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
	}

	// 读取 + 构建：聚合所有错误，任一失败即不生成。
	var errs []error
	var tablesToGen []*model.Table
	for _, name := range tableList {
		meta, merr := reader.ReadTable(ctx, name)
		if merr != nil {
			errs = append(errs, merr)
			continue
		}
		tbl, berr := builder.Build(meta, builder.Options{Module: *module})
		if berr != nil {
			errs = append(errs, berr)
			continue
		}
		tablesToGen = append(tablesToGen, tbl)
	}
	if len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "%v\n", e)
		}
		fmt.Fprintf(os.Stderr, "共 %d 个错误，未生成任何文件\n", len(errs))
		os.Exit(1)
	}

	unitTest := true
	if cfg != nil {
		unitTest = cfg.Codegen.UnitTestEnable
	}
	if *noTests {
		unitTest = false
	}

	files, err := codegen.Generate(tablesToGen, codegen.Options{UnitTestEnable: unitTest})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
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

func schemaFromDSN(dsn string) string {
	// user:pass@tcp(host:port)/dbname?params
	slash := strings.LastIndex(dsn, "/")
	if slash < 0 {
		return ""
	}
	rest := dsn[slash+1:]
	if q := strings.IndexAny(rest, "?"); q >= 0 {
		rest = rest[:q]
	}
	return rest
}
