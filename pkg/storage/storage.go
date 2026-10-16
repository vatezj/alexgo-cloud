// Package storage 纯 IO 驱动层：只按相对路径读写删，零 DB 零策略。
// relPath 由调用方（service）生成——驱动不得把任何用户输入解析进盘路径。
// 将来 S3/OSS 驱动同样只实现 Driver 三个方法。
package storage

import (
	"context"
	"errors"
	"io"
)

// ErrNotFound 目标相对路径不存在（Open/Delete 未命中）。
var ErrNotFound = errors.New("storage: file not found")

// Driver 存储驱动合同。ctx 供远端驱动做请求取消；本地驱动在入口检查一次。
type Driver interface {
	// Save 以独占创建写入；目标已存在必须报错而非覆盖。
	Save(ctx context.Context, relPath string, r io.Reader) error
	// Open 打开读取流；不存在返回 ErrNotFound。
	Open(ctx context.Context, relPath string) (io.ReadCloser, error)
	// Delete 删除文件；不存在返回 ErrNotFound。
	Delete(ctx context.Context, relPath string) error
}
