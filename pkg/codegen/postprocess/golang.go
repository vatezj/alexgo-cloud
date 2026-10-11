// pkg/codegen/postprocess/golang.go
package postprocess

import (
	"fmt"
	"go/format"
	"strings"

	"alexGo-cloud/pkg/codegen/model"
)

// FormatGo 格式化 Go 产物；失败返回 ErrFormat（绝不静默放行）。
// 非 .go 路径（.vue/.ts/.sql 等前端/SQL 输出）原样直通——
// format.Source 对非 Go 源会报错，导致整批生成失败。
func FormatGo(path string, src []byte) ([]byte, error) {
	if !strings.HasSuffix(path, ".go") {
		return src, nil
	}
	out, err := format.Source(src)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", model.ErrFormat, path, err)
	}
	return out, nil
}
