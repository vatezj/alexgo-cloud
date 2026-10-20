package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/scorecard/model"
	"alexGo-cloud/modules/scorecard/service"
)

// stubRooms 只覆写 AdminListRooms 的 RoomService 桩：其余方法经嵌入接口下沉
// （nil 方法，若被误调用会 panic 暴露，不是静默通过）。
type stubRooms struct {
	service.RoomService
	status     int
	roomCode   string
	page, size int
	called     bool
}

func (s *stubRooms) AdminListRooms(_ context.Context, status int, roomCode string, page, size int) ([]*model.Room, int64, error) {
	s.called = true
	s.status, s.roomCode, s.page, s.size = status, roomCode, page, size
	return []*model.Room{}, 0, nil
}

func listRooms(t *testing.T, query string) *stubRooms {
	t.Helper()
	gin.SetMode(gin.TestMode)
	stub := &stubRooms{}
	c := &AdminController{rooms: stub, records: nil, friends: nil}
	r := gin.New()
	r.GET("/rooms", c.ListRooms)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/rooms"+query, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("http=%d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body.Code != 0 {
		t.Fatalf("信封 code 应 0: %v body=%s", err, w.Body.String())
	}
	if !stub.called {
		t.Fatal("AdminListRooms 未被调用")
	}
	return stub
}

// TestListRooms_RoomCodeFormTag 终审 Important #3：form:"room_code" 透传（房号筛的
// controller 一环；tag 写错会静默丢参）。
func TestListRooms_RoomCodeFormTag(t *testing.T) {
	stub := listRooms(t, "?page=1&size=10&status=1&room_code=ABCD")
	if stub.roomCode != "ABCD" || stub.status != 1 || stub.page != 1 || stub.size != 10 {
		t.Fatalf("透传错: code=%q status=%d page=%d size=%d",
			stub.roomCode, stub.status, stub.page, stub.size)
	}
}

// TestListRooms_DefaultsKeepNoFilter 参缺省：status=-1（不过滤）、room_code 空串。
func TestListRooms_DefaultsKeepNoFilter(t *testing.T) {
	stub := listRooms(t, "?page=2&size=20")
	if stub.status != -1 || stub.roomCode != "" {
		t.Fatalf("参缺省错: status=%d code=%q", stub.status, stub.roomCode)
	}
	if stub.page != 2 || stub.size != 20 {
		t.Fatalf("分页透传错: page=%d size=%d", stub.page, stub.size)
	}
}
