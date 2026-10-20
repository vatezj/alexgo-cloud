package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"alexGo-cloud/modules/scorecard/model"
	"alexGo-cloud/pkg/repo"
	"alexGo-cloud/pkg/tenant"
)

// RoomLogRepository 房间流水数据访问。幂等靠 GetByRequestID 软查（service 先查后插
// 同事务）；租户隔离靠 gormplugin。
type RoomLogRepository interface {
	Create(ctx context.Context, l *model.RoomLog) error
	GetByRequestID(ctx context.Context, roomID uint64, reqID string) (*model.RoomLog, error) // 软查：无 → (nil, nil)
	ListByRoom(ctx context.Context, roomID uint64, page, size int) ([]*model.RoomLog, int64, error)
	ListTypeByUser(ctx context.Context, roomID, userID, typ, limit int) ([]*model.RoomLog, error) // 上限频控：取该用户近期该类型日志，service 解码 extra.session_no 判本会话首触发
}

type roomLogRepo struct {
	db *gorm.DB
}

func NewRoomLogRepository(db *gorm.DB) RoomLogRepository {
	return &roomLogRepo{db: db}
}

func (r *roomLogRepo) Create(ctx context.Context, entity *model.RoomLog) error {
	// 打戳（spec §9.2）：json:"-" 使客户端灌不进来，零值才回填、显式值保留。
	if entity.TenantID == 0 {
		entity.TenantID = tenant.TenantIDFromContext(ctx)
	}
	return r.db.WithContext(ctx).Create(entity).Error
}

// GetByRequestID 幂等键软查：type=1 支出的 ClientRequestID 房间内查重。
func (r *roomLogRepo) GetByRequestID(ctx context.Context, roomID uint64, reqID string) (*model.RoomLog, error) {
	var out model.RoomLog
	err := r.db.WithContext(ctx).
		Where("room_id = ? AND client_request_id = ?", roomID, reqID).First(&out).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &out, err
}

// ListByRoom 分页流水（created_at 倒序，快照最近 20 条即 page=1 size=20）。
func (r *roomLogRepo) ListByRoom(ctx context.Context, roomID uint64, page, size int) ([]*model.RoomLog, int64, error) {
	q := r.db.Model(&model.RoomLog{}).
		Where("room_id = ?", roomID).Order("created_at desc, id desc")
	return repo.Paginate[*model.RoomLog](ctx, q, page, size)
}

// ListTypeByUser 该用户近期某类型日志（倒序取前 limit 条），上限频控判本会话首触发用。
func (r *roomLogRepo) ListTypeByUser(ctx context.Context, roomID, userID, typ, limit int) ([]*model.RoomLog, error) {
	if limit <= 0 {
		limit = 20
	}
	var list []*model.RoomLog
	err := r.db.WithContext(ctx).
		Where("room_id = ? AND from_user_id = ? AND type = ?", roomID, userID, typ).
		Order("id desc").Limit(limit).Find(&list).Error
	return list, err
}
