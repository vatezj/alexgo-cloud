// Package excel 导出 xlsx 的薄共享层：表头+数据行构建、行数上限、下载文件名与
// Content-Disposition。列映射由各域 controller 私有提供（Column 闭包），
// 本包不感知任何业务模型。
package excel

import (
	"errors"
	"mime"
	"time"

	"github.com/xuri/excelize/v2"

	errs "alexGo-cloud/pkg/errors"
)

// MaxExportRows 导出行数保险丝：租户内全量可能很大，超限直接 30013 拒绝。
const MaxExportRows = 50000

// MIME xlsx 的 Content-Type（下载流豁免信封的响应体类型）。
const MIME = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

// Column 一列：标题 + 从行取值的闭包（返回 excelize 可写的 string/int/int64/
// float64/bool；时间请走 TimeRFC3339 转字符串）。
type Column[T any] struct {
	Title string
	Cell  func(T) any
}

// Spec 一次导出的完整描述。
type Spec[T any] struct {
	Sheet    string      // sheet 名（如 "用户"）
	FileName string      // 下载文件名（用 FileName() 生成）
	Columns  []Column[T] // 列（表头 + 取值闭包）
	Rows     []T         // 数据行
}

// Build 先完整构建 xlsx 到内存再返回字节。
// 契约：返回非 nil error 时一个字节都没产出——调用方发信封是安全的
// （Review Focus ② 先查后写；信封豁免只发生在调用方 ctx.Data 写字节之后，
// 与 codegen download 同款）。excelize 行数据本就驻留内存，Build 非流式是
// 事实形态而非妥协（决策②「不经 P1-1 落盘」不受影响）。
func Build[T any](spec Spec[T]) ([]byte, error) {
	if len(spec.Rows) > MaxExportRows {
		return nil, errs.New(errs.CodeExportTooManyRows, nil)
	}
	if len(spec.Columns) == 0 {
		return nil, errs.New(errs.CodeParamInvalid, errors.New("excel: no columns"))
	}

	f := excelize.NewFile()
	defer f.Close()
	sheet := spec.Sheet
	if sheet == "" {
		sheet = "Sheet1"
	}
	// NewFile 默认建 Sheet1；改名到目标 sheet。
	f.SetSheetName("Sheet1", sheet)

	for i, col := range spec.Columns {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(sheet, cell, col.Title); err != nil {
			return nil, err
		}
	}
	for r, row := range spec.Rows {
		for c, col := range spec.Columns {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+2)
			if err := f.SetCellValue(sheet, cell, col.Cell(row)); err != nil {
				return nil, err
			}
		}
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// FileName 带时间戳的下载文件名：<base>_<yyyyMMdd_HHmmss>.xlsx。
func FileName(base string) string {
	return base + "_" + time.Now().Format("20060102_150405") + ".xlsx"
}

// Disposition RFC5987 Content-Disposition：mime.FormatMediaType 对 ASCII 名内联
// filename=、非 ASCII 自动升级 filename*=utf-8''…（file_controller serve 段先例）。
func Disposition(fileName string) string {
	return mime.FormatMediaType("attachment", map[string]string{"filename": fileName})
}

// TimeRFC3339 时间列取值：RFC3339；nil → 空串（LoginDate 等可空时间不 panic）。
func TimeRFC3339(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}
