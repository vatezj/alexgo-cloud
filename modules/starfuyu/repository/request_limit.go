package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"alexGo-cloud/modules/starfuyu/model"
)

// RequestLimitRepository 每日请求限流（brief 钉死签名，Task 5 服务层唯一数据入口）。
// 用户表：不显式加 tenant 过滤——依赖 gormplugin + claims 写回的 ctx（scorecard 惯例）。
type RequestLimitRepository interface {
	// CheckAndIncrement 查或建当日行并 count+1；返回新 count。超限由 service 判定。
	CheckAndIncrement(ctx context.Context, memberUserID uint64, date, typ string) (int, error)
	// AdminTodaySummary 指定 date 按 type 聚合 SUM(count)（type+count 聚合）。
	AdminTodaySummary(ctx context.Context, date string) ([]TodayLimitRow, error)
}

// TodayLimitRow AdminTodaySummary 聚合行：当日各 type 的累计使用次数。
type TodayLimitRow struct {
	Type  string `gorm:"column:type" json:"type"`
	Count int64  `gorm:"column:count" json:"count"`
}

type requestLimitRepo struct{ db *gorm.DB }

func NewRequestLimitRepository(db *gorm.DB) RequestLimitRepository {
	return &requestLimitRepo{db: db}
}

// checkAndIncrementMaxAttempts 读改写 + 唯一键冲突的重试上限（sqlite/mysql 双兼容，
// 不走 ON DUPLICATE KEY 方言：DoNothing 插入 + 乐观锁更新跨方言一致）。
const checkAndIncrementMaxAttempts = 5

func (r *requestLimitRepo) CheckAndIncrement(ctx context.Context, memberUserID uint64, date, typ string) (int, error) {
	for attempt := 0; attempt < checkAndIncrementMaxAttempts; attempt++ {
		n, retry, err := r.tryIncrement(ctx, memberUserID, date, typ)
		if err != nil {
			return 0, err
		}
		if !retry {
			return n, nil
		}
	}
	return 0, fmt.Errorf(
		"starfuyu: CheckAndIncrement 冲突重试耗尽(%d): member=%d date=%s type=%s",
		checkAndIncrementMaxAttempts, memberUserID, date, typ)
}

// tryIncrement 单轮读改写事务：
//   - 行存在：乐观锁 UPDATE count=count+1 WHERE id AND count，RowsAffected=0 → 需重试；
//   - 行不存在：DoNothing 插入 count=1，RowsAffected=0（唯一键冲突）→ 需重试。
//
// 返回 (0, true, nil) 表示本轮冲突，请外层重试。
func (r *requestLimitRepo) tryIncrement(ctx context.Context, memberUserID uint64, date, typ string) (int, bool, error) {
	next := 0
	retry := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row model.RequestLimit
		err := tx.Where("member_user_id = ? AND date = ? AND type = ?",
			memberUserID, date, typ).First(&row).Error
		switch {
		case err == nil:
			res := tx.Model(&model.RequestLimit{}).
				Where("id = ? AND count = ?", row.ID, row.Count).
				Updates(map[string]any{"count": row.Count + 1, "updated_at": time.Now()})
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				retry = true
				return nil
			}
			next = row.Count + 1
			return nil
		case errors.Is(err, gorm.ErrRecordNotFound):
			res := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.RequestLimit{
				MemberUserID: memberUserID,
				Date:         date,
				Type:         typ,
				Count:        1,
				// tenant_id 不硬编码：gormplugin 按 ctx 回填（tid=0 保持 0）。
			})
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				retry = true // 唯一键冲突：他人已建行 → 重试走更新
				return nil
			}
			next = 1
			return nil
		default:
			return err
		}
	})
	if err != nil {
		return 0, false, err
	}
	if retry {
		return 0, true, nil
	}
	return next, false, nil
}

func (r *requestLimitRepo) AdminTodaySummary(ctx context.Context, date string) ([]TodayLimitRow, error) {
	var rows []TodayLimitRow
	err := r.db.WithContext(ctx).Model(&model.RequestLimit{}).
		Select("type, SUM(count) AS count").
		Where("date = ?", date).
		Group("type").
		Order("type asc").
		Scan(&rows).Error
	return rows, err
}
