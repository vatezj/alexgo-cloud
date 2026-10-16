package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// LocalDriver 本地盘驱动：root 下按相对路径读写删。
// 防御：构造时 root 转绝对路径；resolve 拒穿越/拒绝对路径 + 前缀约束双保险；
// Save 用 O_CREATE|O_EXCL 独占创建，碰撞即报错不覆盖。
type LocalDriver struct {
	root string // 绝对路径、无尾分隔符
}

// NewLocalDriver 构造驱动并 MkdirAll 盘根（0o755）。
func NewLocalDriver(root string) (*LocalDriver, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("storage: abs root: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("storage: mkdir root %q: %w", abs, err)
	}
	return &LocalDriver{root: abs}, nil
}

// resolve 把 relPath 解析为 root 内绝对路径。
// 第一保险：绝对路径注入直接拒；Clean 后仍以 ".." 开头（穿越 root）直接拒。
// 第二保险：结果必须以 root+分隔符开头（防实现细节变化）。
// 穿越语义上"被拒"而非锚定进 root——spec §9 要求 "../ 穿越被拒"。
func (d *LocalDriver) resolve(relPath string) (string, error) {
	if strings.TrimSpace(relPath) == "" {
		return "", fmt.Errorf("storage: empty rel path")
	}
	if filepath.IsAbs(relPath) {
		return "", fmt.Errorf("storage: absolute rel path: %q", relPath)
	}
	clean := filepath.Clean(relPath)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("storage: path escapes root: %q", relPath)
	}
	full := filepath.Join(d.root, clean)
	if !strings.HasPrefix(full, d.root+string(os.PathSeparator)) {
		return "", fmt.Errorf("storage: path escapes root: %q", relPath)
	}
	return full, nil
}

// Save 独占创建写入；写入中途失败删除半截文件，删除失败并入返回错误。
func (d *LocalDriver) Save(ctx context.Context, relPath string, r io.Reader) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	full, err := d.resolve(relPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return fmt.Errorf("storage: mkdir for %q: %w", relPath, err)
	}
	f, err := os.OpenFile(full, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("storage: create %q: %w", relPath, err)
	}
	if _, err := io.Copy(f, r); err != nil {
		_ = f.Close()
		writeErr := fmt.Errorf("storage: write %q: %w", relPath, err)
		if rmErr := os.Remove(full); rmErr != nil {
			// 删除半截失败必须可见：否则残留半截文件会让同 relPath 重试永久撞 O_EXCL。
			return errors.Join(writeErr, fmt.Errorf("storage: remove partial %q: %w", relPath, rmErr))
		}
		return writeErr
	}
	return f.Close()
}

// Open 打开读取流；不存在映射 ErrNotFound。
func (d *LocalDriver) Open(ctx context.Context, relPath string) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	full, err := d.resolve(relPath)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(full)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("storage: open %q: %w", relPath, err)
	}
	return f, nil
}

// Delete 删除文件；不存在映射 ErrNotFound。
func (d *LocalDriver) Delete(ctx context.Context, relPath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	full, err := d.resolve(relPath)
	if err != nil {
		return err
	}
	if err := os.Remove(full); err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound
		}
		return fmt.Errorf("storage: remove %q: %w", relPath, err)
	}
	return nil
}
