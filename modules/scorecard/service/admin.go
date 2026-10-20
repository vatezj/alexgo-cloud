package service

import (
	"context"

	"gorm.io/gorm"

	"alexGo-cloud/modules/scorecard/model"
	"alexGo-cloud/modules/scorecard/repository"
	errs "alexGo-cloud/pkg/errors"
)

// adminDetailLogsCap AdminRoomDetail 拉取的最近流水条数上限。
// 监管详情页给的是「最近动态」视图，不做无限上翻——全量流水走 app 端
// /rooms/:id/logs 分页接口（或后续给本接口加 page/size 入参）。
const adminDetailLogsCap = 200

// AdminListRooms 监管端房间分页。status<0 = 不过滤（controller 默认 -1）；
// roomCode 空 = 不过滤，非空按房号精确等值（spec §4.2 房间列表筛选）；
// repo.ListAdmin 插件在位时只看本租户（跨租户监管由租户上下文决定）。
func (s *roomService) AdminListRooms(ctx context.Context, status int, roomCode string, page, size int) ([]*model.Room, int64, error) {
	if page <= 0 || size <= 0 {
		return nil, 0, errs.New(errs.CodeParamInvalid, nil)
	}
	return s.room.ListAdmin(ctx, status, roomCode, page, size)
}

// AdminRoomDetail 监管端房间详情：房行（不存在/跨租户 → 30101）+ 全量成员
// （ListByRoom 不分页，房成员数有界）+ 最近 adminDetailLogsCap 条流水
// （ListByRoom page=1 倒序取头一段，cap 见常量注释）。
func (s *roomService) AdminRoomDetail(ctx context.Context, roomID uint64) (*AdminRoomDetail, error) {
	if roomID == 0 {
		return nil, errs.New(errs.CodeParamInvalid, nil)
	}
	room, err := s.room.GetByID(ctx, roomID)
	if err != nil {
		return nil, err
	}
	if room == nil {
		return nil, errs.New(errs.CodeScorecardRoomNotFound, nil)
	}
	members, err := s.member.ListByRoom(ctx, roomID)
	if err != nil {
		return nil, err
	}
	logs, _, err := s.log.ListByRoom(ctx, roomID, 1, adminDetailLogsCap)
	if err != nil {
		return nil, err
	}
	return &AdminRoomDetail{Room: room, Members: members, Logs: logs}, nil
}

// AdminClose 强制关房（监管端）：不校验房主身份，status=0（进行中）与
// status=1（已结算）均可关 → status=2；房不存在/跨租户 → 30101，已关再关 → 30103。
// 单事务（tx 绑定 repo，T7 契约，同 SweepExpired 模式）：FOR UPDATE 行锁复验
// → SaveVersioned（version 就地 +1）→ type=8 流水 extra admin_close。
// 整体失败随事务回滚（含流水），不会出现「房关了但流水丢了」的半态。
func (s *roomService) AdminClose(ctx context.Context, roomID uint64) error {
	if roomID == 0 {
		return errs.New(errs.CodeParamInvalid, nil)
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		roomRepo := repository.NewRoomRepository(tx)
		logRepo := repository.NewRoomLogRepository(tx)
		locked, err := roomRepo.GetForUpdate(ctx, tx, roomID)
		if err != nil {
			return err
		}
		if locked == nil {
			return errs.New(errs.CodeScorecardRoomNotFound, nil)
		}
		if locked.Status == 2 {
			return errs.New(errs.CodeScorecardRoomClosed, nil)
		}
		locked.Status = 2
		if err := roomRepo.SaveVersioned(ctx, tx, locked); err != nil {
			return err
		}
		return logRepo.Create(ctx, &model.RoomLog{
			TenantID: locked.TenantID, RoomID: locked.ID,
			Type: 8, Extra: `{"event":"admin_close"}`,
		})
	})
}
