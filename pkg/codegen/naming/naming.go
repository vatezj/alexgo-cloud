// pkg/codegen/naming/naming.go
package naming

import (
	"regexp"
	"strings"

	"github.com/jinzhu/inflection"
)

var (
	nonWord = regexp.MustCompile(`[^a-zA-Z0-9]+`)
	// 切驼峰：先匹配整段缩写（后不跟小写，如 HTTP），再匹配"大写+小写"单词（如 Request）。
	// 原设计是 `[A-Z]+(?![a-z])|[A-Z][a-z]*|[a-z]+|[0-9]+` 单趟切词，但 Go 的 regexp
	// 是 RE2，不支持负向先行断言 (?!...)，单趟写法会在 init 时 panic。等价改写为
	// 多步插入分隔符再按 nonWord 切分：
	//   1) camelAcronym：缩写尾 + 词首大写（HTTPRequest → HTTP_Request）；
	//      不能用 `[A-Z]+(?:[a-z]+|[0-9]+)?`——贪心会把 HTTPRequest 合成一坨。
	//   2) camelCase：小写/数字 → 大写（UserID → User_ID、Version2 → Version_2）；
	//   3) camelLetterDigit / camelDigitLetter：字母 ↔ 数字边界（UTF8 → UTF_8）。
	camelAcronym     = regexp.MustCompile(`([A-Z])([A-Z][a-z])`)
	camelCase        = regexp.MustCompile(`([a-z0-9])([A-Z])`)
	camelLetterDigit = regexp.MustCompile(`([a-zA-Z])([0-9])`)
	camelDigitLetter = regexp.MustCompile(`([0-9])([a-zA-Z])`)
)

// acronym 表内缩写保持全大写；表外单词走首字母大写。
var acronym = map[string]string{
	"id": "ID", "url": "URL", "json": "JSON", "api": "API",
	"http": "HTTP", "sql": "SQL", "xml": "XML", "uid": "UID",
}

// EntityName 表名 → 类名：按 _ 切词，末词单数化，再驼峰。
// order_items → OrderItem；addresses → Address（旧 dbgen 的 TrimSuffix "s" 会错成 Addres）。
func EntityName(table string) string {
	if table == "" {
		return ""
	}
	parts := nonWord.Split(strings.ToLower(table), -1)
	out := make([]string, 0, len(parts))
	for i, p := range parts {
		if p == "" {
			continue
		}
		if i == len(parts)-1 {
			p = inflection.Singular(p)
		}
		out = append(out, capitalize(p))
	}
	return strings.Join(out, "")
}

// ToGoName 列名 → Go 字段名：user_id → UserID；identity → Identity。
func ToGoName(col string) string {
	if col == "" {
		return ""
	}
	parts := nonWord.Split(col, -1)
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		low := strings.ToLower(p)
		if a, ok := acronym[low]; ok {
			b.WriteString(a)
			continue
		}
		b.WriteString(capitalize(low))
	}
	return b.String()
}

// Snake 驼峰 → 下蛇：OrderItem → order_item；UserID → user_id。
func Snake(s string) string {
	if s == "" {
		return ""
	}
	// 顺序：先切缩写尾，再切小写→大写，最后切字母↔数字；随后交给 nonWord 兜底。
	s = camelAcronym.ReplaceAllString(s, "${1}_${2}")
	s = camelCase.ReplaceAllString(s, "${1}_${2}")
	s = camelLetterDigit.ReplaceAllString(s, "${1}_${2}")
	s = camelDigitLetter.ReplaceAllString(s, "${1}_${2}")
	raw := nonWord.Split(strings.ToLower(s), -1)
	parts := make([]string, 0, len(raw))
	for _, p := range raw {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, "_")
}

// Plural 实体名 → 路径用复数下蛇：OrderItem → order_items。
func Plural(entity string) string {
	if entity == "" {
		return ""
	}
	return inflection.Plural(Snake(entity))
}

func IsSensitive(col string) bool {
	low := strings.ToLower(col)
	return strings.Contains(low, "password") || strings.Contains(low, "secret") || strings.Contains(low, "token")
}

// JSONName 列名 → json tag；敏感字段返回 "-"（不输出到 JSON）。
func JSONName(col string) string {
	if IsSensitive(col) {
		return "-"
	}
	return col
}

func capitalize(s string) string {
	return strings.ToUpper(s[:1]) + s[1:]
}
