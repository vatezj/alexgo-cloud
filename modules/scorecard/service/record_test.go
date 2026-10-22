package service

import (
	"testing"

	"alexGo-cloud/modules/scorecard/model"
	"alexGo-cloud/modules/scorecard/repository"
)

func TestRecords_Flow(t *testing.T) {
	svc, ctx, roomID, loser, winner := setupRoom(t, 0, false, 0, 0)
	if _, err := svc.Expense(ctx, loser, roomID, ExpenseReq{Score: 20, ToUserID: winner, ClientRequestID: "r"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Settle(ctx, 100, roomID); err != nil {
		t.Fatal(err)
	}
	// 与 setupRoom 同一个 sqlite 库（跨库会读不到上面刚结算的 record）
	roomInternal := svc.(*roomService)
	rs := NewRecordService(repository.NewRecordRepository(roomInternal.db))
	list, total, err := rs.MyRecords(ctx, loser, 1, 10)
	if err != nil || total != 1 || list[0].Score != -20 {
		t.Fatalf("我的战绩: total=%d list=%v err=%v", total, list, err)
	}
	detail, err := rs.Detail(ctx, loser, list[0].ID)
	if err != nil || len(detail) != 2 {
		t.Fatalf("明细应全房 2 行: %v %v", detail, err)
	}
	st, err := rs.Stat(ctx, winner)
	if err != nil || st.TotalScore != 20 || st.Win != 1 || st.WinRate != 1 {
		t.Fatalf("统计: %+v err=%v", st, err)
	}
	// 非本人删除被拒（NewRecordService 返回接口，白盒直取 repo 需断言具体类型）
	other := model.Record{TenantID: 1, RoomID: roomID, UserID: loser, Score: -20, SettleType: 1}
	if err := rs.(*recordService).repo.Create(ctx, []*model.Record{&other}); err != nil {
		t.Fatal(err)
	}
	if err := rs.Delete(ctx, winner, other.ID); err == nil {
		t.Fatal("非本人删除应拒")
	}
	if err := rs.Delete(ctx, loser, other.ID); err != nil {
		t.Fatal(err)
	}
}

// TestRecords_AdminListAndEdge 自审覆盖：AdminList 过滤/分页、Detail 非本人 30111、
// Stat 分母 0（无战绩/全平）胜率归 0。
func TestRecords_AdminListAndEdge(t *testing.T) {
	svc, ctx, roomID, loser, winner := setupRoom(t, 0, false, 0, 0)
	if _, err := svc.Expense(ctx, loser, roomID, ExpenseReq{Score: 20, ToUserID: winner, ClientRequestID: "r"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Settle(ctx, 100, roomID); err != nil {
		t.Fatal(err)
	}
	roomInternal := svc.(*roomService)
	rs := NewRecordService(repository.NewRecordRepository(roomInternal.db))

	// AdminList：roomID 过滤全房 2 行；size=1 分页 total 仍为 2；userID 过滤只剩本人 1 行。
	rows, total, err := rs.AdminList(ctx, 0, roomID, 1, 1)
	if err != nil || total != 2 || len(rows) != 1 {
		t.Fatalf("监管分页: total=%d len=%d err=%v", total, len(rows), err)
	}
	rows, total, err = rs.AdminList(ctx, loser, 0, 1, 10)
	if err != nil || total != 1 || len(rows) != 1 || rows[0].UserID != loser {
		t.Fatalf("监管按人过滤: total=%d rows=%v err=%v", total, rows, err)
	}

	// Detail 非本人：winner 查 loser 的行 → 30111。
	mine, _, err := rs.MyRecords(ctx, loser, 1, 10)
	if err != nil || len(mine) == 0 {
		t.Fatalf("取本人行: %v %v", mine, err)
	}
	if _, err := rs.Detail(ctx, winner, mine[0].ID); err == nil {
		t.Fatal("明细非本人应拒")
	}

	// Stat 分母 0：无战绩的第三方（未进房）全 0 且胜率 0。
	st, err := rs.Stat(ctx, 999)
	if err != nil || st.TotalScore != 0 || st.Win != 0 || st.Lose != 0 || st.Draw != 0 || st.WinRate != 0 {
		t.Fatalf("空统计: %+v err=%v", st, err)
	}
	// 全平分母 0：写一行 Score=0 战绩后再统计。
	dr := model.Record{TenantID: 1, RoomID: roomID, UserID: 999, Score: 0, SettleType: 1}
	if err := rs.(*recordService).repo.Create(ctx, []*model.Record{&dr}); err != nil {
		t.Fatal(err)
	}
	st, err = rs.Stat(ctx, 999)
	if err != nil || st.Draw != 1 || st.WinRate != 0 {
		t.Fatalf("平局统计: %+v err=%v", st, err)
	}
}
