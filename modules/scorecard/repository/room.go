package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"alexGo-cloud/modules/scorecard/model"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/repo"
	"alexGo-cloud/pkg/tenant"
	"alexGo-cloud/pkg/tenant/gormplugin"
)

// RoomRepository 房间数据访问。读路径的租户隔离依赖 gormplugin（ctx 带租户编号时
// Query/Update/Delete 自动追加 tenant_id 条件，生产 main.go 与单测均真注册）；
// 写路径 SaveVersioned 额外显式带 tenant_id（tid=0 时 fail-closed）。
type RoomRepository interface {
	Create(ctx context.Context, r *model.Room) error
	GetByID(ctx context.Context, id uint64) (*model.Room, error)                   // 软查（快照/校验用）：不存在/跨租户 → (nil, nil)
	GetForUpdate(ctx context.Context, tx *gorm.DB, id uint64) (*model.Room, error) // SELECT FOR UPDATE（sqlite 忽略锁子句）
	GetByCode(ctx context.Context, code string) (*model.Room, error)
	SaveVersioned(ctx context.Context, tx *gorm.DB, r *model.Room) error                     // UPDATE 全字段 WHERE id AND version；RowsAffected=0 → 30112
	ListAdmin(ctx context.Context, status int, page, size int) ([]*model.Room, int64, error) // status<0 = 不过滤
	SweepExpired(ctx context.Context, now time.Time) ([]*model.Room, error)
}

type roomRepo struct {
	db *gorm.DB
}

func NewRoomRepository(db *gorm.DB) RoomRepository {
	return &roomRepo{db: db}
}

func (r *roomRepo) Create(ctx context.Context, entity *model.Room) error {
	// 打戳（spec §9.2）：json:"-" 使客户端灌不进来，零值才回填、显式值保留。
	if entity.TenantID == 0 {
		entity.TenantID = tenant.TenantIDFromContext(ctx)
	}
	return r.db.WithContext(ctx).Create(entity).Error
}

// GetByID 软查：不存在与跨租户（插件过滤后查无）统一 (nil, nil)，由 service 映射 30101。
func (r *roomRepo) GetByID(ctx context.Context, id uint64) (*model.Room, error) {
	var out model.Room
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&out).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &out, err
}

// GetForUpdate 行锁读（支出/结算事务的第一步，MySQL 下 FOR UPDATE 串行化同房并发）。
func (r *roomRepo) GetForUpdate(ctx context.Context, tx *gorm.DB, id uint64) (*model.Room, error) {
	var out model.Room
	err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", id).First(&out).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &out, err
}

// GetByCode 房号入房查。uk_room_code 全局唯一；插件在位时叠加租户条件，
// 他租户房号对本租户表现为不存在。
func (r *roomRepo) GetByCode(ctx context.Context, code string) (*model.Room, error) {
	var out model.Room
	err := r.db.WithContext(ctx).Where("room_code = ?", code).First(&out).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &out, err
}

// SaveVersioned 乐观锁全量更新：WHERE id AND version AND tenant_id，version 就地 +1。
// 命中 0 行（版本过期/并发者已提交/跨租户）→ 30112；成功后调用方内存快照 version 同步 +1，
// 同事务内连续保存无需重新 Get。
func (r *roomRepo) SaveVersioned(ctx context.Context, tx *gorm.DB, room *model.Room) error {
	res := tx.WithContext(ctx).Model(&model.Room{}).
		Where("id = ? AND version = ? AND tenant_id = ?", room.ID, room.Version, tenant.TenantIDFromContext(ctx)).
		Updates(map[string]interface{}{
			"room_code":                 room.RoomCode,
			"creator_id":                room.CreatorID,
			"status":                    room.Status,
			"expire_time":               room.ExpireTime,
			"tea_target_amount":         room.TeaTargetAmount,
			"tea_percent":               room.TeaPercent,
			"tea_pool_total":            room.TeaPoolTotal,
			"tea_auto_enabled":          room.TeaAutoEnabled,
			"max_lose_score":            room.MaxLoseScore,
			"round_no":                  room.RoundNo,
			"pending_tea_target_amount": room.PendingTeaTargetAmount,
			"pending_tea_percent":       room.PendingTeaPercent,
			"pending_max_lose_score":    room.PendingMaxLoseScore,
			"pending_apply_round_no":    room.PendingApplyRoundNo,
			"version":                   gorm.Expr("version + 1"),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errs.New(errs.CodeScorecardVersionConflict, nil)
	}
	room.Version++
	return nil
}

// ListAdmin 监管端分页列表。status<0 不过滤；插件在位时只看本租户。
func (r *roomRepo) ListAdmin(ctx context.Context, status int, page, size int) ([]*model.Room, int64, error) {
	q := r.db.Model(&model.Room{}).Order("id desc")
	if status >= 0 {
		q = q.Where("status = ?", status)
	}
	return repo.Paginate[*model.Room](ctx, q, page, size)
}

// SweepExpired 拉逾期进行中房间。系统级任务：IgnoreTenant 跨全部租户扫
// （逐房处理由 service 各自带租户上下文写日志），与 rbac.go ListAll 同口径。
func (r *roomRepo) SweepExpired(ctx context.Context, now time.Time) ([]*model.Room, error) {
	var list []*model.Room
	err := r.db.WithContext(gormplugin.IgnoreTenant(ctx)).
		Where("status = ? AND expire_time < ?", 0, now).
		Find(&list).Error
	return list, err
}
