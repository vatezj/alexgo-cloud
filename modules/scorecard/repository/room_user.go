package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"alexGo-cloud/modules/scorecard/model"
	"alexGo-cloud/pkg/tenant"
)

// RoomUserRepository 房间成员数据访问。租户隔离靠 gormplugin（ctx 带租户编号时
// 自动追加 tenant_id）；计数/减分走表达式列更新，避免读改写竞态。
type RoomUserRepository interface {
	Create(ctx context.Context, u *model.RoomUser) error
	Get(ctx context.Context, roomID, userID uint64) (*model.RoomUser, error) // 软查：不存在 → (nil, nil)
	ListByRoom(ctx context.Context, roomID uint64) ([]*model.RoomUser, error)
	CountByRoom(ctx context.Context, roomID uint64) (int64, error)
	// GetActiveRoomID 该用户进行中（room.status=0）的房 id；无 → 0。一人一局约束的查询面。
	GetActiveRoomID(ctx context.Context, userID uint64) (uint64, error)
	Delete(ctx context.Context, roomID, userID uint64) error
	AddScore(ctx context.Context, tx *gorm.DB, roomID, userID uint64, delta int) error
	AddCumLose(ctx context.Context, tx *gorm.DB, roomID, userID uint64, delta int) error // 同 AddScore，列换 cum_lose_score
	SetCreator(ctx context.Context, tx *gorm.DB, roomID, userID uint64, isCreator int) error
}

type roomUserRepo struct {
	db *gorm.DB
}

func NewRoomUserRepository(db *gorm.DB) RoomUserRepository {
	return &roomUserRepo{db: db}
}

func (r *roomUserRepo) Create(ctx context.Context, entity *model.RoomUser) error {
	// 打戳（spec §9.2）：json:"-" 使客户端灌不进来，零值才回填、显式值保留。
	if entity.TenantID == 0 {
		entity.TenantID = tenant.TenantIDFromContext(ctx)
	}
	return r.db.WithContext(ctx).Create(entity).Error
}

// Get 成员在房校验软查：不存在/跨租户统一 (nil, nil)，由 service 映射 30102。
func (r *roomUserRepo) Get(ctx context.Context, roomID, userID uint64) (*model.RoomUser, error) {
	var out model.RoomUser
	err := r.db.WithContext(ctx).
		Where("room_id = ? AND user_id = ?", roomID, userID).First(&out).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &out, err
}

func (r *roomUserRepo) ListByRoom(ctx context.Context, roomID uint64) ([]*model.RoomUser, error) {
	var list []*model.RoomUser
	err := r.db.WithContext(ctx).
		Where("room_id = ?", roomID).Order("id asc").Find(&list).Error
	return list, err
}

func (r *roomUserRepo) CountByRoom(ctx context.Context, roomID uint64) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&model.RoomUser{}).
		Where("room_id = ?", roomID).Count(&n).Error
	return n, err
}

// GetActiveRoomID 该用户是否在进行中牌局（一人一局）。主表 room_user 走插件租户过滤，
// status 过滤走子查询（JOIN 会让插件注入的无前缀 tenant_id 变成两表歧义列）；成员行与
// 房行同租户打戳，交集即本租户的进行中房。同人多行（历史脏数据，如旧后端建的房）
// 取最新一行——复用语义下用户期望回到刚开/最近在玩的局，而非最早的残留局。
func (r *roomUserRepo) GetActiveRoomID(ctx context.Context, userID uint64) (uint64, error) {
	var roomID uint64
	err := r.db.WithContext(ctx).Model(&model.RoomUser{}).
		Select("room_id").
		Where("user_id = ?", userID).
		Where("room_id IN (SELECT id FROM scorecard_room WHERE status = 0)").
		Order("id desc").Limit(1).
		Scan(&roomID).Error
	return roomID, err
}

func (r *roomUserRepo) Delete(ctx context.Context, roomID, userID uint64) error {
	return r.db.WithContext(ctx).
		Where("room_id = ? AND user_id = ?", roomID, userID).
		Delete(&model.RoomUser{}).Error
}

// AddScore 表达式列更新（cur_score += delta），锁在 UPDATE 行上，免读改写竞态。
func (r *roomUserRepo) AddScore(ctx context.Context, tx *gorm.DB, roomID, userID uint64, delta int) error {
	return tx.WithContext(ctx).Model(&model.RoomUser{}).
		Where("room_id = ? AND user_id = ?", roomID, userID).
		UpdateColumn("cur_score", gorm.Expr("cur_score + ?", delta)).Error
}

// AddCumLose 同 AddScore，列换 cum_lose_score（上限频控的累计输入）。
func (r *roomUserRepo) AddCumLose(ctx context.Context, tx *gorm.DB, roomID, userID uint64, delta int) error {
	return tx.WithContext(ctx).Model(&model.RoomUser{}).
		Where("room_id = ? AND user_id = ?", roomID, userID).
		UpdateColumn("cum_lose_score", gorm.Expr("cum_lose_score + ?", delta)).Error
}

// SetCreator 房主标记（转让房主时两行互换 is_creator）。
func (r *roomUserRepo) SetCreator(ctx context.Context, tx *gorm.DB, roomID, userID uint64, isCreator int) error {
	return tx.WithContext(ctx).Model(&model.RoomUser{}).
		Where("room_id = ? AND user_id = ?", roomID, userID).
		UpdateColumn("is_creator", isCreator).Error
}
