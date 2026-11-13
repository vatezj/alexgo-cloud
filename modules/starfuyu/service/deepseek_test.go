package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"alexGo-cloud/pkg/config"
)

// newTestDeepSeek 假 DeepSeek（httptest）：捕获请求体到 captured（可 nil），
// content 作为 choices[0].message.content 返回。cfg 按 brief：APIKey="k"、BaseURL=srv.URL。
func newTestDeepSeek(t *testing.T, content string, captured *[]byte) FortuneFetcher {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("读取请求体失败: %v", err)
			return
		}
		if captured != nil {
			*captured = body
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": content}},
			},
		})
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{}
	cfg.Starfuyu.Deepseek.APIKey = "k"
	cfg.Starfuyu.Deepseek.BaseURL = srv.URL
	return NewDeepSeekFetcher(cfg)
}

// TestFetchFortunes_PromptAndShareFields prompt 含分享卡字段名；
// 十二行完整 JSON 往返后分享卡字段带值、daily_todos 恰 3 条。
func TestFetchFortunes_PromptAndShareFields(t *testing.T) {
	const date = "2026-11-13"
	payload, err := json.Marshal(twelveRows(date, "深"))
	if err != nil {
		t.Fatalf("marshal 十二行失败: %v", err)
	}
	var captured []byte
	f := newTestDeepSeek(t, string(payload), &captured)

	rows, err := f.FetchFortunes(context.Background(), date)
	if err != nil {
		t.Fatalf("FetchFortunes err = %v, want nil", err)
	}
	if len(rows) != 12 {
		t.Fatalf("rows len = %d, want 12", len(rows))
	}

	var req deepSeekRequest
	if err := json.Unmarshal(captured, &req); err != nil {
		t.Fatalf("解析请求体失败: %v", err)
	}
	if len(req.Messages) == 0 {
		t.Fatalf("请求 Messages 为空")
	}
	prompt := req.Messages[0].Content
	for _, key := range []string{
		"match_constellation", "advice", "avoid", "reunion_tip",
		"single_tip", "job_tip", "reunion_score", "single_score",
		"job_score", "daily_todos",
	} {
		if !strings.Contains(prompt, key) {
			t.Errorf("prompt 缺字段名 %q", key)
		}
	}

	row := rows[0]
	if row.Advice == "" {
		t.Errorf("rows[0].Advice = %q, want 非空", row.Advice)
	}
	if len(row.DailyTodos) != 3 {
		t.Errorf("rows[0].DailyTodos len = %d (%v), want 3", len(row.DailyTodos), row.DailyTodos)
	}
	if row.MatchConstellation == "" {
		t.Errorf("rows[0].MatchConstellation = %q, want 非空", row.MatchConstellation)
	}
}

// TestFetchFortunes_MissingShareFieldsTolerated DeepSeek 漏分享卡字段 = 落零值不判失败。
func TestFetchFortunes_MissingShareFieldsTolerated(t *testing.T) {
	const date = "2026-11-13"
	payload, err := json.Marshal(twelveRows(date, "缺"))
	if err != nil {
		t.Fatalf("marshal 十二行失败: %v", err)
	}
	var arr []map[string]any
	if err := json.Unmarshal(payload, &arr); err != nil {
		t.Fatalf("转 []map 失败: %v", err)
	}
	newKeys := []string{
		"match_constellation", "advice", "avoid", "reunion_tip", "single_tip",
		"job_tip", "reunion_score", "single_score", "job_score", "daily_todos",
	}
	for i := range arr {
		for _, k := range newKeys {
			delete(arr[i], k)
		}
	}
	stripped, err := json.Marshal(arr)
	if err != nil {
		t.Fatalf("marshal 删除后失败: %v", err)
	}

	f := newTestDeepSeek(t, string(stripped), nil)
	rows, err := f.FetchFortunes(context.Background(), date)
	if err != nil {
		t.Fatalf("漏字段应容错, err = %v, want nil", err)
	}
	if len(rows) != 12 {
		t.Fatalf("rows len = %d, want 12", len(rows))
	}
	for i, row := range rows {
		if row.Advice != "" {
			t.Errorf("rows[%d].Advice = %q, want 空", i, row.Advice)
		}
		if row.DailyTodos != nil {
			t.Errorf("rows[%d].DailyTodos = %v, want nil", i, row.DailyTodos)
		}
		if row.ReunionScore != 0 {
			t.Errorf("rows[%d].ReunionScore = %d, want 0", i, row.ReunionScore)
		}
	}
}

// TestFetchFortunes_InvalidMatchConstellationToEmpty 非法合拍星座 → 存空串，其余行不动。
func TestFetchFortunes_InvalidMatchConstellationToEmpty(t *testing.T) {
	const date = "2026-11-13"
	payload, err := json.Marshal(twelveRows(date, "枚"))
	if err != nil {
		t.Fatalf("marshal 十二行失败: %v", err)
	}
	var arr []map[string]any
	if err := json.Unmarshal(payload, &arr); err != nil {
		t.Fatalf("转 []map 失败: %v", err)
	}
	arr[0]["match_constellation"] = "麒麟座" // 12 星座枚举之外
	mutated, err := json.Marshal(arr)
	if err != nil {
		t.Fatalf("marshal 变异后失败: %v", err)
	}
	wantOther := twelveRows(date, "枚")[1].MatchConstellation // 其余行期望值快照

	f := newTestDeepSeek(t, string(mutated), nil)
	rows, err := f.FetchFortunes(context.Background(), date)
	if err != nil {
		t.Fatalf("FetchFortunes err = %v, want nil", err)
	}
	if len(rows) != 12 {
		t.Fatalf("rows len = %d, want 12", len(rows))
	}
	if rows[0].MatchConstellation != "" {
		t.Errorf("非法枚举未清空: rows[0].MatchConstellation = %q, want 空", rows[0].MatchConstellation)
	}
	if rows[1].MatchConstellation != wantOther {
		t.Errorf("其余行受影响: rows[1].MatchConstellation = %q, want %q", rows[1].MatchConstellation, wantOther)
	}
}
