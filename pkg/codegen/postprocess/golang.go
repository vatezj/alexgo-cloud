// pkg/codegen/postprocess/golang.go
package postprocess

import (
	"fmt"
	"go/format"

	"alexGo-cloud/pkg/codegen/model"
)

// FormatGo 格式化 Go 产物；失败返回 ErrFormat（绝不静默放行）。
func FormatGo(path string, src []byte) ([]byte, error) {
	out, err := format.Source(src)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", model.ErrFormat, path, err)
	}
	return out, nil
}
