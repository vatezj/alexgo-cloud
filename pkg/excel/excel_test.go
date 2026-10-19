package excel

import (
	"bytes"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"

	errs "alexGo-cloud/pkg/errors"
)

type row struct {
	Name string
	Age  int
	When *time.Time
}

func threeCols() []Column[row] {
	return []Column[row]{
		{Title: "姓名", Cell: func(r row) any { return r.Name }},
		{Title: "年龄", Cell: func(r row) any { return r.Age }},
		{Title: "时间", Cell: func(r row) any { return TimeRFC3339(r.When) }},
	}
}

// cell 安全取格：excelize GetRows 会裁剪行尾空格，越界回退空串。
func cell(rows [][]string, r, c int) string {
	if r >= len(rows) || c >= len(rows[r]) {
		return ""
	}
	return rows[r][c]
}

// 往返：Build 出的 xlsx 用 excelize 读回，表头+数据行逐格比对（含 nil 时间→空串）。
func TestBuild_RoundTrip(t *testing.T) {
	when := time.Date(2026, 10, 19, 8, 30, 0, 0, time.UTC)
	data, err := Build(Spec[row]{
		Sheet:    "测试",
		FileName: "t.xlsx",
		Columns:  threeCols(),
		Rows: []row{
			{Name: "张三", Age: 30, When: &when},
			{Name: "李四", Age: 25, When: nil},
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	defer f.Close()
	rows, err := f.GetRows("测试")
	if err != nil {
		t.Fatalf("GetRows: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3 (header + 2)", len(rows))
	}
	for i, w := range []string{"姓名", "年龄", "时间"} {
		if cell(rows, 0, i) != w {
			t.Errorf("header[%d] = %q, want %q", i, cell(rows, 0, i), w)
		}
	}
	if cell(rows, 1, 0) != "张三" || cell(rows, 1, 1) != "30" || cell(rows, 1, 2) != "2026-10-19T08:30:00Z" {
		t.Errorf("row1 = %v", rows[1])
	}
	if cell(rows, 2, 0) != "李四" || cell(rows, 2, 2) != "" {
		t.Errorf("row2 (nil time) = %v", rows[2])
	}
}

// 空 Rows 合法：仅表头 xlsx。
func TestBuild_EmptyRows_HeaderOnly(t *testing.T) {
	data, err := Build(Spec[row]{Sheet: "测试", FileName: "t.xlsx", Columns: threeCols()})
	if err != nil {
		t.Fatalf("Build empty: %v", err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	defer f.Close()
	rows, err := f.GetRows("测试")
	if err != nil {
		t.Fatalf("GetRows: %v", err)
	}
	if len(rows) != 1 || cell(rows, 0, 0) != "姓名" {
		t.Errorf("empty rows sheet = %v, want header only", rows)
	}
}

// 超限：Build 在产出任何字节之前拒绝（RF②）——返回 30013 且零字节。
func TestBuild_TooManyRows(t *testing.T) {
	rows := make([]row, MaxExportRows+1)
	data, err := Build(Spec[row]{Sheet: "测试", FileName: "t.xlsx", Columns: threeCols(), Rows: rows})
	if !errs.Is(err, errs.CodeExportTooManyRows) {
		t.Fatalf("err = %v, want 30013", err)
	}
	if data != nil {
		t.Errorf("data must be nil on error, got %d bytes", len(data))
	}
}

// 空列定义拒绝（不 panic；Column[T] 泛型下「列数不一致」结构上不可表示，此为等价防护）。
func TestBuild_NoColumns(t *testing.T) {
	if _, err := Build(Spec[row]{Rows: []row{{Name: "x"}}}); !errs.Is(err, errs.CodeParamInvalid) {
		t.Fatalf("err = %v, want 10001", err)
	}
}

// FileName / Disposition / TimeRFC3339 纯函数形状。
func TestHelpers(t *testing.T) {
	name := FileName("users")
	if len(name) < len("users_") || name[:6] != "users_" || name[len(name)-5:] != ".xlsx" {
		t.Errorf("FileName = %q", name)
	}
	if got := Disposition("users_20261019_153045.xlsx"); got != "attachment; filename=users_20261019_153045.xlsx" {
		t.Errorf("Disposition = %q", got)
	}
	if TimeRFC3339(nil) != "" {
		t.Error("TimeRFC3339(nil) must be empty")
	}
	when := time.Date(2026, 10, 19, 8, 30, 0, 0, time.UTC)
	if got := TimeRFC3339(&when); got != "2026-10-19T08:30:00Z" {
		t.Errorf("TimeRFC3339 = %q", got)
	}
}
