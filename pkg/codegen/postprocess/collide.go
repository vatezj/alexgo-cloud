// pkg/codegen/postprocess/collide.go
package postprocess

import (
	"fmt"

	"alexGo-cloud/pkg/codegen/model"
)

// CheckCollide 校验输出路径无重复（同一次生成内两模板映射到同一文件）。
func CheckCollide(files []model.GeneratedFile) error {
	seen := make(map[string]bool, len(files))
	for _, f := range files {
		if seen[f.Path] {
			return fmt.Errorf("%w: duplicate path %s", model.ErrPathCollide, f.Path)
		}
		seen[f.Path] = true
	}
	return nil
}
