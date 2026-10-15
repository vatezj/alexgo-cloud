package types

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDateTimeUnmarshalJSON(t *testing.T) {
	cases := []struct {
		in      string
		wantErr bool
		want    string // "2006-01-02 15:04:05"
	}{
		{`"2026-01-02 15:04:05"`, false, "2026-01-02 15:04:05"},
		{`"2026-01-02"`, false, "2026-01-02 00:00:00"},
		{`"2026-01-02T15:04:05+08:00"`, false, "2026-01-02 15:04:05"},
		{`""`, true, ""},
		{`"not-a-time"`, true, ""},
		{`12345`, true, ""},
	}
	for _, c := range cases {
		var d DateTime
		err := json.Unmarshal([]byte(c.in), &d)
		if c.wantErr {
			if err == nil {
				t.Fatalf("input %s: want err, got %v", c.in, d)
			}
			continue
		}
		if err != nil {
			t.Fatalf("input %s: %v", c.in, err)
		}
		if got := d.Format("2006-01-02 15:04:05"); got != c.want {
			t.Errorf("input %s: got %s, want %s", c.in, got, c.want)
		}
	}
}

func TestDateTimeMarshalJSON(t *testing.T) {
	d := DateTime{Time: time.Date(2026, 1, 2, 15, 4, 5, 0, time.Local)}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `"2026-01-02 15:04:05"` {
		t.Fatalf("got %s", b)
	}
}

func TestDateTimeUnmarshalParam(t *testing.T) {
	var d DateTime
	if err := d.UnmarshalParam("2026-01-02 15:04:05"); err != nil {
		t.Fatal(err)
	}
	if err := d.UnmarshalParam(""); err == nil {
		t.Fatal("empty string should error")
	}
}
