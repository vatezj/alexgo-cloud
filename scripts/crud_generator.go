package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
)

// 这是 alexGo-cloud 的 CRUD 生成器（模板驱动）。
//
// 输入：
// - 遍历 modules/**.proto
// - 从 proto 中识别 service XxxService → Entity=Xxx
// - （轻量模式）尝试从 message Xxx { ... } 中提取基础字段类型（string/int32/uint64...）
//
// 输出：
// - 使用 scripts/templates/crud/*.tpl 生成 model/repository/service/controller/admin/controller/app 及 module 注册片段
// - 默认不覆盖已存在文件（保证“生成器不会破坏手写代码”）
//
// 用法：
// - 全量扫描：go run ./scripts/crud_generator.go
// - 指定模块：go run ./scripts/crud_generator.go --module=order
// - 指定实体：go run ./scripts/crud_generator.go --module=order --service=Order
// - 强制覆盖：go run ./scripts/crud_generator.go --force
//
// 生产建议：
// - 若要精确字段映射（proto AST、注释、tag、枚举、关联、分页等），可升级为 protoc-gen-* 插件解析 FileDescriptorSet。
type Field struct {
	Name     string
	GoName   string
	GoType   string
	JSONName string
}

type CrudData struct {
	Module      string
	ServiceName string
	ProtoPkg    string
	LowerName   string
	Fields      []Field
}

const templatesDir = "scripts/templates/crud"

var (
	reService = regexp.MustCompile(`(?m)^\s*service\s+([A-Za-z0-9_]+)\s*\{`)
	rePackage = regexp.MustCompile(`(?m)^\s*package\s+([A-Za-z0-9_.]+)\s*;`)
	reMessage = regexp.MustCompile(`(?ms)^\s*message\s+%s\s*\{(.*?)^\s*\}`)
	reField   = regexp.MustCompile(`(?m)^\s*(repeated\s+)?([A-Za-z0-9_.]+)\s+([a-zA-Z0-9_]+)\s*=\s*\d+\s*;`)
	reGoType  = regexp.MustCompile(`(?m)^\s*type\s+([A-Za-z0-9_]+)\s+(struct|interface)\s*\{`)
	reGoFunc  = regexp.MustCompile(`(?m)^\s*func\s+([A-Za-z0-9_]+)\s*\(`)
)

func main() {
	moduleFlag := flag.String("module", "", "target module name (e.g. system)")
	serviceFlag := flag.String("service", "", "target service/entity name without Service suffix (e.g. User)")
	force := flag.Bool("force", false, "overwrite generated files if exist")
	flag.Parse()

	var specs []CrudData
	_ = filepath.WalkDir("modules", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".proto") {
			return nil
		}
		// gRPC 契约（如 api/rpc/token.proto）不是 CRUD 实体——跳过，
		// 否则 make proto/generate-crud 会为 TokenService 生成无业务语义的脚手架。
		if strings.Contains(filepath.ToSlash(path), "/rpc/") {
			return nil
		}

		mod := moduleName(path)
		if *moduleFlag != "" && mod != *moduleFlag {
			return nil
		}
		b, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil
		}
		content := string(b)
		protoPkg := ""
		if m := rePackage.FindStringSubmatch(content); len(m) == 2 {
			protoPkg = m[1]
		}

		for _, m := range reService.FindAllStringSubmatch(content, -1) {
			svc := m[1]
			entity := strings.TrimSuffix(svc, "Service")
			if entity == "" || entity == svc {
				continue
			}
			if *serviceFlag != "" && entity != *serviceFlag {
				continue
			}
			fields := parseMessageFields(content, entity)
			specs = append(specs, CrudData{
				Module:      mod,
				ServiceName: entity,
				ProtoPkg:    protoPkg,
				LowerName:   strings.ToLower(entity),
				Fields:      fields,
			})
		}
		return nil
	})

	seen := map[string]bool{}
	for _, s := range specs {
		key := s.Module + ":" + s.ServiceName
		if seen[key] {
			continue
		}
		seen[key] = true
		if err := generate(s, *force); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
		}
	}
}

func moduleName(protoPath string) string {
	p := filepath.ToSlash(protoPath)
	parts := strings.Split(p, "/")
	for i := 0; i < len(parts)-1; i++ {
		if parts[i] == "modules" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

func hasTypeInModule(module, typeName string) bool {
	if module == "" {
		return true
	}
	root := filepath.Join("modules", module)
	found := false
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if found || err != nil {
			return nil
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		b, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil
		}
		for _, m := range reGoType.FindAllStringSubmatch(string(b), -1) {
			if m[1] == typeName {
				found = true
				return nil
			}
		}
		return nil
	})
	return found
}

func generate(d CrudData, force bool) error {
	if d.Module == "" || d.ServiceName == "" {
		return nil
	}

	// 实体级门控：模块内已存在该实体类型（手写或已生成）时，整体跳过该实体的全部输出。
	//
	// 为什么不能只靠逐输出的符号检查：手写代码的命名与模板假设不一致
	// （例如不存在 Admin{Entity}Controller、请求结构体散落在别的文件），
	// 逐输出检查会“部分生成”，产生 redeclared 与调用不存在方法的编译错误。
	// 要么全跳过、要么全生成，才有一致性。
	// 需要重新生成时：先删除对应 *_gen.go（手写类型不应删除，此类实体本就该跳过）。
	if !force && hasTypeInModule(d.Module, d.ServiceName) {
		fmt.Printf("skip %s.%s: type %s already exists in modules/%s (delete *_gen.go to regenerate)\n",
			d.Module, d.ServiceName, d.ServiceName, d.Module)
		return nil
	}

	type output struct {
		targetPath string
		tplName    string
		existsIn   string
		symbolKind string
		symbolName string
	}

	outputs := []output{
		{
			targetPath: filepath.Join("modules", d.Module, "model", d.LowerName+"_gen.go"),
			tplName:    "model.go.tpl",
			existsIn:   filepath.Join("modules", d.Module, "model"),
			symbolKind: "type",
			symbolName: d.ServiceName,
		},
		{
			targetPath: filepath.Join("modules", d.Module, "repository", d.LowerName+"_gen.go"),
			tplName:    "repository.go.tpl",
			existsIn:   filepath.Join("modules", d.Module, "repository"),
			symbolKind: "type",
			symbolName: d.ServiceName + "Repository",
		},
		{
			targetPath: filepath.Join("modules", d.Module, "service", d.LowerName+"_gen.go"),
			tplName:    "service.go.tpl",
			existsIn:   filepath.Join("modules", d.Module, "service"),
			symbolKind: "type",
			symbolName: d.ServiceName + "Service",
		},
		{
			targetPath: filepath.Join("modules", d.Module, "controller", "admin", d.LowerName+"_gen.go"),
			tplName:    "controller_admin.go.tpl",
			existsIn:   filepath.Join("modules", d.Module, "controller", "admin"),
			symbolKind: "type",
			symbolName: "Admin" + d.ServiceName + "Controller",
		},
		{
			targetPath: filepath.Join("modules", d.Module, "controller", "app", d.LowerName+"_gen.go"),
			tplName:    "controller_app.go.tpl",
			existsIn:   filepath.Join("modules", d.Module, "controller", "app"),
			symbolKind: "type",
			symbolName: "App" + d.ServiceName + "Controller",
		},
		{
			targetPath: filepath.Join("modules", d.Module, "module_register_"+d.LowerName+"_gen.go"),
			tplName:    "module_register.go.tpl",
			existsIn:   filepath.Join("modules", d.Module),
			symbolKind: "func",
			symbolName: "Register" + d.ServiceName + "Routes",
		},
	}

	funcMap := template.FuncMap{
		"lower":  strings.ToLower,
		"plural": pluralize,
	}

	for _, out := range outputs {
		if out.existsIn != "" && out.symbolName != "" {
			if symbolExists(out.existsIn, out.symbolKind, out.symbolName) {
				continue
			}
		}
		tplPath := filepath.Join(templatesDir, out.tplName)
		tpl, err := template.New(out.tplName).Funcs(funcMap).ParseFiles(tplPath)
		if err != nil {
			return err
		}
		var buf bytes.Buffer
		if err := tpl.Execute(&buf, d); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(out.targetPath), 0o755); err != nil {
			return err
		}
		if err := writeFile(out.targetPath, buf.Bytes(), force); err != nil {
			return err
		}
	}

	fmt.Printf("CRUD generated for %s.%s\n", d.Module, d.ServiceName)
	return nil
}

func writeFile(path string, content []byte, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
	}
	return os.WriteFile(path, content, 0o644)
}

func pluralize(s string) string {
	if s == "" {
		return s
	}
	if strings.HasSuffix(s, "s") {
		return s + "es"
	}
	return s + "s"
}

func symbolExists(dir, kind, name string) bool {
	if dir == "" || name == "" {
		return false
	}
	found := false
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if found || err != nil {
			return nil
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		b, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil
		}
		s := string(b)
		switch kind {
		case "type":
			for _, m := range reGoType.FindAllStringSubmatch(s, -1) {
				if m[1] == name {
					found = true
					return nil
				}
			}
		case "func":
			for _, m := range reGoFunc.FindAllStringSubmatch(s, -1) {
				if m[1] == name {
					found = true
					return nil
				}
			}
		}
		return nil
	})
	return found
}

func parseMessageFields(protoContent, msgName string) []Field {
	if msgName == "" {
		return nil
	}
	re := regexp.MustCompile(fmt.Sprintf(reMessage.String(), regexp.QuoteMeta(msgName)))
	m := re.FindStringSubmatch(protoContent)
	if len(m) != 2 {
		return nil
	}
	body := m[1]
	var fields []Field
	for _, fm := range reField.FindAllStringSubmatch(body, -1) {
		typ := fm[2]
		name := fm[3]
		goType := protoTypeToGo(typ, fm[1] != "")
		if goType == "" {
			continue
		}
		goName := toCamel(name)
		fields = append(fields, Field{
			Name:     name,
			GoName:   goName,
			GoType:   goType,
			JSONName: name,
		})
	}
	return fields
}

func protoTypeToGo(t string, repeated bool) string {
	var base string
	switch t {
	case "string":
		base = "string"
	case "bool":
		base = "bool"
	case "int32":
		base = "int32"
	case "int64":
		base = "int64"
	case "uint32":
		base = "uint32"
	case "uint64":
		base = "uint64"
	default:
		return ""
	}
	if repeated {
		return "[]" + base
	}
	return base
}

func toCamel(s string) string {
	if s == "" {
		return s
	}
	parts := strings.Split(s, "_")
	for i := range parts {
		p := parts[i]
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	out := strings.Join(parts, "")
	if out == "Id" {
		return "ID"
	}
	if strings.HasSuffix(out, "Id") {
		return strings.TrimSuffix(out, "Id") + "ID"
	}
	return out
}
