package newmodule

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"alexGo-cloud/pkg/codegen/model"
)

// FileChange 一个文件的变更（create：Before=nil）。
type FileChange struct {
	Path   string
	Before []byte
	After  []byte
}

// ChangeSet 全部变更的内存集合——全有或全无的原子单位。
type ChangeSet struct {
	// Root 落盘根目录（Plan 从 o.Root 带入）。全部 Path 相对此根，
	// Apply 据此拼出实际写入路径；空视为 "."（与 CLI Root="." 同语义）。
	Root    string
	Creates []FileChange
	Patches []FileChange
}

// Plan 纯内存构建：壳层 + 实体 + 菜单种子渲染，三个既有文件读入并编辑。
func Plan(o Options) (*ChangeSet, error) {
	if err := o.Validate(); err != nil {
		return nil, err
	}
	cs := &ChangeSet{Root: o.Root}

	addCreates := func(files []model.GeneratedFile) {
		for _, f := range files {
			cs.Creates = append(cs.Creates, FileChange{Path: f.Path, After: f.Content})
		}
	}

	shell, err := shellFiles(o)
	if err != nil {
		return nil, err
	}
	addCreates(shell)

	ents, err := entityFiles(o)
	if err != nil {
		return nil, err
	}
	addCreates(ents)

	menu, err := menuSeedFiles(&o)
	if err != nil {
		return nil, err
	}
	addCreates(menu)

	// 三个既有文件
	patch := func(rel string, edit func(src []byte) ([]byte, error)) error {
		full := filepath.Join(o.Root, rel)
		src, err := os.ReadFile(full)
		if err != nil {
			return fmt.Errorf("读 %s: %w", rel, err)
		}
		after, err := edit(src)
		if err != nil {
			return fmt.Errorf("编辑 %s: %w", rel, err)
		}
		cs.Patches = append(cs.Patches, FileChange{Path: rel, Before: src, After: after})
		return nil
	}

	if err := patch("alexgo-server/cmd/main.go", func(src []byte) ([]byte, error) {
		return editMainGo(src, o.Name)
	}); err != nil {
		return nil, err
	}
	if err := patch("alexgo-server/configs/config.yaml", func(src []byte) ([]byte, error) {
		return editConfigYAML(src, o.Name)
	}); err != nil {
		return nil, err
	}
	if err := patch("modules/system/service/permission_routes.go", func(src []byte) ([]byte, error) {
		return editPermissionRoutes(src, o.Name, o.Entities)
	}); err != nil {
		return nil, err
	}
	return cs, nil
}

// Apply 落盘：先 creates 后 patches；任一失败回滚已做部分。成功返回 undo。
// 所有 Path 相对 cs.Root（Plan 携带），失败时连同失败点目录链一并清理——
// 失败路径也记入 writtenCreates，让 pruneEmptyDirs 沿其祖代上行，
// 占位残留（如测试注入的同名文件）与已建空目录都被清掉。
func Apply(cs *ChangeSet) (func(), error) {
	root := cs.Root
	if root == "" {
		root = "."
	}
	var writtenCreates []string
	var writtenPatches []FileChange

	removeCreated := func() {
		for i := len(writtenCreates) - 1; i >= 0; i-- {
			_ = os.Remove(writtenCreates[i])
		}
		pruneEmptyDirs(writtenCreates)
	}
	rollback := func() {
		removeCreated()
		for _, p := range writtenPatches {
			_ = os.WriteFile(p.Path, p.Before, 0o644)
		}
	}

	for _, c := range cs.Creates {
		dst := filepath.Join(root, c.Path)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			writtenCreates = append(writtenCreates, dst)
			rollback()
			return nil, fmt.Errorf("创建目录 %s: %w", filepath.Dir(c.Path), err)
		}
		if err := os.WriteFile(dst, c.After, 0o644); err != nil {
			writtenCreates = append(writtenCreates, dst)
			rollback()
			return nil, fmt.Errorf("写 %s: %w", c.Path, err)
		}
		writtenCreates = append(writtenCreates, dst)
	}
	for _, p := range cs.Patches {
		dst := filepath.Join(root, p.Path)
		// 写前先记账：WriteFile 中途失败（如 ENOSPC 半截写）时 rollback 仍回写原文，
		// 回滚语义闭合；完全没写成功的情形回写原文是幂等无害的。
		writtenPatches = append(writtenPatches, FileChange{Path: dst, Before: p.Before, After: p.After})
		if err := os.WriteFile(dst, p.After, 0o644); err != nil {
			rollback()
			return nil, fmt.Errorf("写 %s: %w", p.Path, err)
		}
	}

	undo := func() {
		removeCreated()
		for _, p := range writtenPatches {
			_ = os.WriteFile(p.Path, p.Before, 0o644)
		}
	}
	return undo, nil
}

// pruneEmptyDirs 删除创建文件所在的空目录（深者先删，逐级向上，删不动即止）。
// undo/rollback 后 modules/<name> 整树不留残渣；碰到非空的上层（如 modules/）
// 的 os.Remove 自然失败停止，不会误删无关目录。
func pruneEmptyDirs(files []string) {
	dirset := map[string]bool{}
	for _, f := range files {
		for d := filepath.Dir(f); d != "." && d != "/" && !dirset[d]; d = filepath.Dir(d) {
			dirset[d] = true
		}
	}
	dirs := make([]string, 0, len(dirset))
	for d := range dirset {
		dirs = append(dirs, d)
	}
	// 按路径深度倒序（长路径先删，保证子目录先清）
	sort.Slice(dirs, func(i, j int) bool {
		return strings.Count(dirs[i], "/") > strings.Count(dirs[j], "/")
	})
	for _, d := range dirs {
		_ = os.Remove(d) // 非空则失败，忽略
	}
}

// Run = Plan → Apply（DryRun 则止步）→ 可选 go build 验证（失败 undo）。
func Run(o Options) (*ChangeSet, error) {
	cs, err := Plan(o)
	if err != nil {
		return nil, err
	}
	if o.DryRun {
		return cs, nil
	}
	undo, err := Apply(cs)
	if err != nil {
		return nil, err
	}
	if o.VerifyBuild {
		cmd := exec.Command("go", "build", "./...")
		cmd.Dir = o.Root
		if out, err := cmd.CombinedOutput(); err != nil {
			undo()
			return nil, fmt.Errorf("go build ./... 失败，已回滚:\n%s", out)
		}
	}
	return cs, nil
}
