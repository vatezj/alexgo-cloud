package service

import (
	"context"

	"alexGo-cloud/modules/scorecard/model"
	"alexGo-cloud/modules/scorecard/repository"
	errs "alexGo-cloud/pkg/errors"
)

// StatResult 个人战绩统计聚合（spec §3.4：总分 + 胜/负/平 + 胜率）。
type StatResult struct {
	TotalScore int64   `json:"total_score"`
	Win        int64   `json:"win"`
	Lose       int64   `json:"lose"`
	Draw       int64   `json:"draw"`
	WinRate    float64 `json:"win_rate"` // (win)/(win+lose)，分母 0 → 0
}

// RecordService scorecard 战绩服务：我的战绩分页/明细（全房反查）/删除/统计聚合/监管端列表。
// Task 12 app 控制器与 Task 13 监管控制器消费。
type RecordService interface {
	// MyRecords 我的战绩分页（仅本人行，created_at 倒序由 repo 保证）。
	MyRecords(ctx context.Context, userID uint64, page, size int) ([]*model.Record, int64, error)
	// Detail 明细：按 recordID 定位 → 不存在 30110 / 非本人 30111 → 反查全房战绩。
	Detail(ctx context.Context, userID, recordID uint64) ([]*model.Record, error)
	// Delete 删本人战绩行：查行（30110）→ 归属校验（30111）→ 单条 Delete。
	Delete(ctx context.Context, userID, recordID uint64) error
	// Stat 个人统计：总分 + 胜（score>0）/负（<0）/平（==0）场次 + 胜率。
	Stat(ctx context.Context, userID uint64) (*StatResult, error)
	// AdminList 监管端战绩分页（userID/roomID 0 = 不过滤，Task 13 消费）。
	AdminList(ctx context.Context, userID, roomID uint64, page, size int) ([]*model.Record, int64, error)
}

type recordService struct {
	repo repository.RecordRepository // 测试白盒直取：rs.repo.Create 构造非本人行
}

// NewRecordService 构造战绩服务（fx 装配；测试与 roomService 同库构造）。
func NewRecordService(repo repository.RecordRepository) RecordService {
	return &recordService{repo: repo}
}

// MyRecords 我的战绩分页：单条 ListByUser（user_id 过滤 + 租户插件隔离），本人行天然只读。
func (s *recordService) MyRecords(ctx context.Context, userID uint64, page, size int) ([]*model.Record, int64, error) {
	return s.repo.ListByUser(ctx, userID, page, size)
}

// Detail 明细：GetByID 软查（不存在/跨租户 → nil → 30110）→ 归属校验（30111）→ ListByRoom 全房。
func (s *recordService) Detail(ctx context.Context, userID, recordID uint64) ([]*model.Record, error) {
	rec, err := s.repo.GetByID(ctx, recordID)
	if err != nil {
		return nil, err
	}
	if rec == nil {
		return nil, errs.New(errs.CodeScorecardRecordNotFound, nil)
	}
	if rec.UserID != userID {
		return nil, errs.New(errs.CodeScorecardNotYourRecord, nil)
	}
	return s.repo.ListByRoom(ctx, rec.RoomID)
}

// Delete 删本人战绩行：查行（30110）→ 归属校验（30111）→ 单条 Delete。
// 不包事务：全程仅一次写语句，查-删间并发竞态窗口的最坏结果是空删（按 id 删
// 不存在行无害）；归属行不会被换绑（主键不变），故接受微竞态、免去 tx 成本。
func (s *recordService) Delete(ctx context.Context, userID, recordID uint64) error {
	rec, err := s.repo.GetByID(ctx, recordID)
	if err != nil {
		return err
	}
	if rec == nil {
		return errs.New(errs.CodeScorecardRecordNotFound, nil)
	}
	if rec.UserID != userID {
		return errs.New(errs.CodeScorecardNotYourRecord, nil)
	}
	return s.repo.Delete(ctx, recordID)
}

// Stat 个人统计：repo 单条 SQL 聚合（免拉全表）；胜率 = win/(win+lose)，分母 0（无
// 战绩或全平）→ 0。
func (s *recordService) Stat(ctx context.Context, userID uint64) (*StatResult, error) {
	total, win, lose, draw, err := s.repo.StatsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	st := &StatResult{TotalScore: total, Win: win, Lose: lose, Draw: draw}
	if denom := win + lose; denom > 0 {
		st.WinRate = float64(win) / float64(denom)
	}
	return st, nil
}

// AdminList 监管端战绩分页：userID/roomID 传 0 表示该维度不过滤（repo.ListAdmin）。
func (s *recordService) AdminList(ctx context.Context, userID, roomID uint64, page, size int) ([]*model.Record, int64, error) {
	return s.repo.ListAdmin(ctx, userID, roomID, page, size)
}
