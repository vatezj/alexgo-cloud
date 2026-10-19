package newmodule

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	texttpl "text/template"
	"time"

	"alexGo-cloud/pkg/codegen/model"
	"alexGo-cloud/pkg/codegen/naming"
	"alexGo-cloud/pkg/codegen/postprocess"
)

//go:embed templates/*.tmpl
var shellFS embed.FS

// shellBind 壳层模板数据。
type shellBind struct {
	Module   string
	Entities []shellEntity
}

// shellEntity 单实体的模板视图。
type shellEntity struct {
	Entity string // Pascal：Product
	Snake  string // product / order_item
	Plural string // products / order_items（路由段，与 module_register 同源）
	Table  string // = Plural（示例实体表名）
}

func shellBindOf(o Options) shellBind {
	b := shellBind{Module: o.Name}
	for _, e := range o.Entities {
		entity := naming.EntityName(naming.Plural(e)) // product → products → Product
		b.Entities = append(b.Entities, shellEntity{
			Entity: entity,
			Snake:  naming.Snake(entity),
			Plural: naming.Plural(entity),
			Table:  naming.Plural(entity),
		})
	}
	return b
}

// nextMigrationVersion 目录内下一个迁移版本号：取 max(现有最大版本+1, 当日 000001)。
func nextMigrationVersion(dir string, now time.Time) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	maxV := 0
	for _, ent := range entries {
		name := ent.Name()
		if !strings.HasSuffix(name, ".up.sql") && !strings.HasSuffix(name, ".down.sql") {
			continue
		}
		head := strings.SplitN(name, "_", 2)[0]
		if v, err := strconv.Atoi(head); err == nil && v > maxV {
			maxV = v
		}
	}
	dateV, err := strconv.Atoi(now.Format("20060102") + "000001")
	if err != nil {
		return "", err
	}
	if dateV > maxV {
		return strconv.Itoa(dateV), nil
	}
	return strconv.Itoa(maxV + 1), nil
}

// shellFiles 渲染模块壳层文件（module.go / migration_source.go / 私有配置 / 每实体迁移对）。
func shellFiles(o Options) ([]model.GeneratedFile, error) {
	b := shellBindOf(o)
	var files []model.GeneratedFile

	renderGo := func(tplName, outPath string) error {
		raw, err := shellFS.ReadFile("templates/" + tplName)
		if err != nil {
			return err
		}
		tpl, err := texttpl.New(tplName).Parse(string(raw))
		if err != nil {
			return fmt.Errorf("解析模板 %s: %w", tplName, err)
		}
		var buf bytes.Buffer
		if err := tpl.Execute(&buf, b); err != nil {
			return fmt.Errorf("渲染 %s: %w", tplName, err)
		}
		out, err := postprocess.FormatGo(outPath, buf.Bytes())
		if err != nil {
			return fmt.Errorf("格式化 %s: %w", outPath, err)
		}
		files = append(files, model.GeneratedFile{Path: outPath, Content: out})
		return nil
	}

	if err := renderGo("module.go.tmpl", fmt.Sprintf("modules/%s/module.go", o.Name)); err != nil {
		return nil, err
	}
	if err := renderGo("migration_source.go.tmpl", fmt.Sprintf("modules/%s/migration_source.go", o.Name)); err != nil {
		return nil, err
	}

	// 模块私有配置（yaml，不走 FormatGo；占位符仍需渲染成真实模块名）
	raw, err := shellFS.ReadFile("templates/module_config.yaml.tmpl")
	if err != nil {
		return nil, err
	}
	cfgTpl, err := texttpl.New("module_config.yaml.tmpl").Parse(string(raw))
	if err != nil {
		return nil, fmt.Errorf("解析模板 module_config.yaml.tmpl: %w", err)
	}
	var cfgBuf bytes.Buffer
	if err := cfgTpl.Execute(&cfgBuf, b); err != nil {
		return nil, fmt.Errorf("渲染 module_config.yaml.tmpl: %w", err)
	}
	files = append(files, model.GeneratedFile{
		Path:    fmt.Sprintf("modules/%s/configs/config.yaml", o.Name),
		Content: cfgBuf.Bytes(),
	})

	// 实体迁移对：版本号在 modules/<name>/migrations 内逐个递增
	migDir := filepath.Join(o.Root, "modules", o.Name, "migrations")
	ver, err := nextMigrationVersion(migDir, o.Now)
	if err != nil {
		return nil, err
	}
	upTpl, _ := texttpl.New("up").Parse(mustRead(shellFS, "templates/entity_up.sql.tmpl"))
	downTpl, _ := texttpl.New("down").Parse(mustRead(shellFS, "templates/entity_down.sql.tmpl"))
	v, _ := strconv.Atoi(ver)
	for i, e := range b.Entities {
		var upBuf, downBuf bytes.Buffer
		if err := upTpl.Execute(&upBuf, e); err != nil {
			return nil, err
		}
		if err := downTpl.Execute(&downBuf, e); err != nil {
			return nil, err
		}
		version := strconv.Itoa(v + i)
		files = append(files,
			model.GeneratedFile{
				Path:    fmt.Sprintf("modules/%s/migrations/%s_%s.up.sql", o.Name, version, e.Table),
				Content: upBuf.Bytes(),
			},
			model.GeneratedFile{
				Path:    fmt.Sprintf("modules/%s/migrations/%s_%s.down.sql", o.Name, version, e.Table),
				Content: downBuf.Bytes(),
			},
		)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

func mustRead(fs embed.FS, name string) string {
	b, err := fs.ReadFile(name)
	if err != nil {
		panic(err)
	}
	return string(b)
}
