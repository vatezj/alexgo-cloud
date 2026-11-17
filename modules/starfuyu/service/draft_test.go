package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"alexGo-cloud/modules/starfuyu/model"
)

// draftFortunes FortuneService 替身：记录 Fetch 日期与调用序。
type draftFortunes struct {
	events *[]string
	date   string
}

func (s *draftFortunes) Fetch(_ context.Context, date string, _ bool) (bool, error) {
	*s.events = append(*s.events, "fetch:"+date)
	return false, nil
}
func (s *draftFortunes) GetByDate(context.Context, string) ([]*model.Fortune, error) {
	return nil, nil
}
func (s *draftFortunes) GetOne(context.Context, string, string) (*model.Fortune, error) {
	return nil, nil
}
func (s *draftFortunes) ListAdmin(context.Context, string, string, int, int) ([]*model.Fortune, int64, error) {
	return nil, 0, nil
}

// draftArticles ArticleService 替身：记录 UploadToMP 日期。
type draftArticles struct {
	events *[]string
}

func (s *draftArticles) BuildHTML(context.Context, string, string) (string, error) { return "", nil }
func (s *draftArticles) UploadToMP(_ context.Context, date string) ([]UploadedArticle, error) {
	*s.events = append(*s.events, "upload:"+date)
	return []UploadedArticle{{Title: "t", MediaID: "m"}}, nil
}

// TestCoverBGForDate 单双选底图：日期天数为单数 → 粉色版，双数 → 橙色版；
// 非法日期报错（用户裁定 2026-11-17）。
func TestCoverBGForDate(t *testing.T) {
	cases := map[string]string{
		"2026-11-17": "cover_bg.png",   // 17 单 → 粉
		"2026-11-18": "cover_bg_2.png", // 18 双 → 橙
		"2026-11-19": "cover_bg.png",   // 19 单 → 粉
		"2026-12-31": "cover_bg.png",   // 31 单 → 粉
		"2027-01-02": "cover_bg_2.png", // 2 双 → 橙
	}
	for date, want := range cases {
		got, err := coverBGForDate(date)
		if err != nil {
			t.Fatalf("coverBGForDate(%q) err = %v", date, err)
		}
		if got != want {
			t.Errorf("coverBGForDate(%q) = %q, want %q", date, got, want)
		}
	}
	if _, err := coverBGForDate("2026/11/18"); err == nil {
		t.Errorf("非法日期应报错")
	}
}

// TestTomorrowSH 上海日历日 +1：固定时刻含跨年边界。
func TestTomorrowSH(t *testing.T) {
	cases := map[string]string{
		"2026-11-17 17:10:00": "2026-11-18",
		"2026-11-17 00:30:00": "2026-11-18",
		"2026-12-31 23:00:00": "2027-01-01",
		"2026-01-01 05:00:00": "2026-01-02",
	}
	for in, want := range cases {
		tm, err := time.ParseInLocation("2006-01-02 15:04:05", in, shLocation())
		if err != nil {
			t.Fatal(err)
		}
		if got := TomorrowSH(tm); got != want {
			t.Errorf("TomorrowSH(%s) = %q, want %q", in, got, want)
		}
	}
}

// TestUploadDraft_Pipeline 顺序 fetch→cover→upload 同一日期；底图按单双；
// 封面输出落在 coverDir/{date}.png。
func TestUploadDraft_Pipeline(t *testing.T) {
	var events []string
	coverDir := t.TempDir()
	r := &draftRunner{
		fortunes: &draftFortunes{events: &events},
		articles: &draftArticles{events: &events},
		coverDir: coverDir,
		runCover: func(_ context.Context, date, bgName, outPath string) error {
			events = append(events, "cover:"+date+":"+bgName)
			if !strings.HasSuffix(outPath, date+".png") || !strings.HasPrefix(outPath, coverDir) {
				t.Errorf("封面输出路径不符: %s (coverDir=%s date=%s)", outPath, coverDir, date)
			}
			return nil
		},
	}

	if err := r.UploadDraft(context.Background(), "2026-11-18"); err != nil {
		t.Fatalf("UploadDraft err = %v, want nil", err)
	}
	want := []string{
		"fetch:2026-11-18",
		"cover:2026-11-18:cover_bg_2.png", // 18 双数 → 橙
		"upload:2026-11-18",
	}
	if len(events) != len(want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
	for i := range want {
		if events[i] != want[i] {
			t.Errorf("events[%d] = %q, want %q", i, events[i], want[i])
		}
	}
}

// TestUploadDraft_CoverFailsNoUpload 封面失败 → 错误上抛且 UploadToMP 不被调用（不发半成品）。
func TestUploadDraft_CoverFailsNoUpload(t *testing.T) {
	var events []string
	r := &draftRunner{
		fortunes: &draftFortunes{events: &events},
		articles: &draftArticles{events: &events},
		coverDir: t.TempDir(),
		runCover: func(context.Context, string, string, string) error {
			return errors.New("python 炸了")
		},
	}
	err := r.UploadDraft(context.Background(), "2026-11-18")
	if err == nil || !strings.Contains(err.Error(), "python 炸了") {
		t.Fatalf("封面错误应上抛, got %v", err)
	}
	for _, e := range events {
		if strings.HasPrefix(e, "upload:") {
			t.Fatalf("封面失败不得上传草稿, events=%v", events)
		}
	}
}
