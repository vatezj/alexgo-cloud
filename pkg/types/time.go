// Package types 提供跨模块共享的 API 值类型。
package types

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// DateTime 是 API 层时间参数/响应值：JSON 与 query 一律 "2006-01-02 15:04:05"
// （本地时区，与 MySQL parseTime=True&loc=Local 一致）。
// Unmarshal 宽松：兼容 "2006-01-02" 与 RFC3339。
type DateTime struct {
	time.Time
}

var timeLayouts = []string{
	"2006-01-02 15:04:05",
	"2006-01-02",
	time.RFC3339,
}

func parseLoose(s string) (time.Time, error) {
	for _, layout := range timeLayouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid datetime %q", s)
}

func (d *DateTime) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	if strings.TrimSpace(s) == "" {
		return fmt.Errorf("empty datetime")
	}
	t, err := parseLoose(s)
	if err != nil {
		return err
	}
	d.Time = t
	return nil
}

func (d *DateTime) UnmarshalParam(s string) error {
	if strings.TrimSpace(s) == "" {
		return fmt.Errorf("empty datetime")
	}
	t, err := parseLoose(s)
	if err != nil {
		return err
	}
	d.Time = t
	return nil
}

func (d DateTime) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Format("2006-01-02 15:04:05"))
}
