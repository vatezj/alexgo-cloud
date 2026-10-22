package service

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"alexGo-cloud/modules/scorecard/model"
	"alexGo-cloud/modules/scorecard/repository"
	errs "alexGo-cloud/pkg/errors"
)

// Settle 结算（spec §3.3，仅房主 30105）：软查房快速失败（不存在 30101/非房主 30105/
// 非进行中 30103、30104）→ 事务内 FOR UPDATE 锁房再验（与 Expense 同序，防并发二次结算）
// → settleTx(settleType=1)：全员 record 终局快照 + status=1 + type=5 流水。
// 事务内全部库访问经 tx 绑定 repo（T7 契约）。
func (s *roomService) Settle(ctx context.Context, userID, roomID uint64) error {
	room, err := s.room.GetByID(ctx, roomID)
	if err != nil {
		return err
	}
	if room == nil {
		return errs.New(errs.CodeScorecardRoomNotFound, nil)
	}
	if room.CreatorID != userID {
		return errs.New(errs.CodeScorecardNotOwner, nil)
	}
	if room.Status == 1 {
		return errs.New(errs.CodeScorecardRoomSettled, nil)
	}
	if room.Status != 0 {
		return errs.New(errs.CodeScorecardRoomClosed, nil)
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		roomRepo := repository.NewRoomRepository(tx)
		locked, err := roomRepo.GetForUpdate(ctx, tx, roomID)
		if err != nil {
			return err
		}
		if locked == nil {
			return errs.New(errs.CodeScorecardRoomNotFound, nil)
		}
		// 锁下复验：并发下房主/状态可能已变（软查只做快速失败，判定以锁下为准）。
		if locked.CreatorID != userID {
			return errs.New(errs.CodeScorecardNotOwner, nil)
		}
		if locked.Status == 1 {
			return errs.New(errs.CodeScorecardRoomSettled, nil)
		}
		if locked.Status != 0 {
			return errs.New(errs.CodeScorecardRoomClosed, nil)
		}
		return s.settleTx(ctx, tx, locked, 1)
	})
}

// Leave 退房（spec §3.4）：软查房/成员快速失败（30101/30102）→ 事务内锁房复验 →
//   - 房主（is_creator=1）且房间进行中：先 settleTx(settleType=2) 自动结算（全员含房主
//     各一行 record、status=1、type=5 流水），再删自己的成员行；其 record 行已由 settleTx
//     写入，不重复写。
//   - 普通成员且房间进行中：先写自己的 record 行（CurScore==0 && CumLoseScore==0 →
//     settle_type=4 零数据空战绩，否则 settle_type=3）再硬删成员行（写在删前，顺序契约）。
//   - 已结算/已关闭房（status!=0）：record 已在结算时落库，只删行不重复写。
//   - 删完 CountByRoom==0 → status=2 + type=8 状态变更流水。
//
// 重进约定（已定）：room_user 硬删；Join 对不存在的行从 session_no=1、cur_score=0、
// cum_lose_score=0 起 = 新会话清零（spec 累计清零即满足；session_no 语义 = 本行进房序次）。
func (s *roomService) Leave(ctx context.Context, userID, roomID uint64) error {
	room, err := s.room.GetByID(ctx, roomID)
	if err != nil {
		return err
	}
	if room == nil {
		return errs.New(errs.CodeScorecardRoomNotFound, nil)
	}
	if u, err := s.member.Get(ctx, roomID, userID); err != nil {
		return err
	} else if u == nil {
		return errs.New(errs.CodeScorecardNotMember, nil)
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// tx 绑定 repo：事务内全部读写走同一条连接（T7 契约）。
		roomRepo := repository.NewRoomRepository(tx)
		logRepo := repository.NewRoomLogRepository(tx)
		userRepo := repository.NewRoomUserRepository(tx)
		recRepo := repository.NewRecordRepository(tx)
		locked, err := roomRepo.GetForUpdate(ctx, tx, roomID)
		if err != nil {
			return err
		}
		if locked == nil {
			return errs.New(errs.CodeScorecardRoomNotFound, nil)
		}
		me, err := userRepo.Get(ctx, roomID, userID)
		if err != nil {
			return err
		}
		if me == nil {
			return errs.New(errs.CodeScorecardNotMember, nil)
		}
		if locked.Status == 0 { // 进行中才写 record；已结算/已关闭不重复落战绩
			if me.IsCreator == 1 {
				// 房主退房：先自动结算（全员 record settle_type=2、status=1、type=5），再删行。
				if err := s.settleTx(ctx, tx, locked, 2); err != nil {
					return err
				}
			} else {
				// 普通成员：写自己的 record 行（删行前）→ 硬删。
				rec := &model.Record{
					TenantID:        locked.TenantID,
					RoomID:          roomID,
					UserID:          userID,
					Score:           me.CurScore,
					CumLoseScore:    me.CumLoseScore,
					TeaPoolTotal:    locked.TeaPoolTotal,
					TeaPercent:      locked.TeaPercent,
					TeaTargetAmount: locked.TeaTargetAmount,
					MaxLoseScore:    locked.MaxLoseScore,
					SettleType:      3,
				}
				if me.CurScore == 0 && me.CumLoseScore == 0 {
					rec.SettleType = 4 // 零数据空战绩
				}
				if err := recRepo.Create(ctx, []*model.Record{rec}); err != nil {
					return err
				}
			}
		}
		// 硬删成员行（无软删字段，gorm Delete 即物理删；重进从 session_no=1 起 = 新会话）。
		if err := userRepo.Delete(ctx, roomID, userID); err != nil {
			return err
		}
		if err := logRepo.Create(ctx, &model.RoomLog{
			TenantID: locked.TenantID, RoomID: roomID, FromUserID: userID, Type: 4,
		}); err != nil {
			return err
		}
		// 最后一人离开 → 房间关闭（空房）+ 状态变更流水。
		n, err := userRepo.CountByRoom(ctx, roomID)
		if err != nil {
			return err
		}
		if n == 0 {
			locked.Status = 2
			if err := roomRepo.SaveVersioned(ctx, tx, locked); err != nil {
				return err // 失败弃用内存 room（Version 已就地 +1，T5 契约）
			}
			if err := logRepo.Create(ctx, &model.RoomLog{
				TenantID: locked.TenantID, RoomID: roomID, FromUserID: userID,
				Type: 8, Extra: `{"status":2}`,
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

// TransferOwner 转让房主（spec §3.5）：软查房快速失败（不存在 30101/非房主 30105/
// 已结算 30104/已关闭 30103/目标不在房 30113/拉黑双向 30108）→ 事务内锁房复验
// （含 status=0 门禁，spec §3.3）→ 两行 is_creator 互换、creator_id=target +
// SaveVersioned、type=9 流水（extra from/to）。
// 拉黑判定 black.Exists 单查即双向（(a黑b) OR (b黑a)，repository 既定口径），
// brief 的 `Exists(a,b) || Exists(b,a)` 双调用是同一语义的冗余写法，此处单查等价。
func (s *roomService) TransferOwner(ctx context.Context, userID, roomID, targetUserID uint64) error {
	if userID == 0 || roomID == 0 || targetUserID == 0 {
		return errs.New(errs.CodeParamInvalid, nil)
	}
	if userID == targetUserID {
		return errs.New(errs.CodeScorecardTransferInvalid, nil) // 同人转让（30107 语义）
	}
	room, err := s.room.GetByID(ctx, roomID)
	if err != nil {
		return err
	}
	if room == nil {
		return errs.New(errs.CodeScorecardRoomNotFound, nil)
	}
	if room.CreatorID != userID {
		return errs.New(errs.CodeScorecardNotOwner, nil)
	}
	// 状态门禁（spec §3.3 校验 status=0，与 Settle 同口径）：已结算/已关闭拒转让。
	if room.Status == 1 {
		return errs.New(errs.CodeScorecardRoomSettled, nil)
	}
	if room.Status != 0 {
		return errs.New(errs.CodeScorecardRoomClosed, nil)
	}
	if to, err := s.member.Get(ctx, roomID, targetUserID); err != nil {
		return err
	} else if to == nil {
		return errs.New(errs.CodeScorecardTargetNotMember, nil)
	}
	blocked, err := s.black.Exists(ctx, userID, targetUserID)
	if err != nil {
		return err
	}
	if blocked {
		return errs.New(errs.CodeScorecardBlacklisted, nil)
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		roomRepo := repository.NewRoomRepository(tx)
		logRepo := repository.NewRoomLogRepository(tx)
		userRepo := repository.NewRoomUserRepository(tx)
		locked, err := roomRepo.GetForUpdate(ctx, tx, roomID)
		if err != nil {
			return err
		}
		if locked == nil {
			return errs.New(errs.CodeScorecardRoomNotFound, nil)
		}
		// 锁下复验房主/状态/目标在房（并发转让以锁下判定为准；status 门禁同软查）。
		if locked.CreatorID != userID {
			return errs.New(errs.CodeScorecardNotOwner, nil)
		}
		if locked.Status == 1 {
			return errs.New(errs.CodeScorecardRoomSettled, nil)
		}
		if locked.Status != 0 {
			return errs.New(errs.CodeScorecardRoomClosed, nil)
		}
		if to, err := userRepo.Get(ctx, roomID, targetUserID); err != nil {
			return err
		} else if to == nil {
			return errs.New(errs.CodeScorecardTargetNotMember, nil)
		}
		if err := userRepo.SetCreator(ctx, tx, roomID, userID, 0); err != nil {
			return err
		}
		if err := userRepo.SetCreator(ctx, tx, roomID, targetUserID, 1); err != nil {
			return err
		}
		locked.CreatorID = targetUserID
		if err := roomRepo.SaveVersioned(ctx, tx, locked); err != nil {
			return err // 失败弃用内存 room（Version 已就地 +1，T5 契约）
		}
		if err := logRepo.Create(ctx, &model.RoomLog{
			TenantID: locked.TenantID, RoomID: roomID,
			FromUserID: userID, ToUserID: targetUserID, Type: 9,
			Extra: fmt.Sprintf(`{"from":%d,"to":%d}`, userID, targetUserID),
		}); err != nil {
			return err
		}
		return nil
	})
}

// settleTx 结算核心（事务内）：全员 room_user 各写一行 record（终局快照字段从 room 拷贝、
// score=u.CurScore、cum_lose_score=u.CumLoseScore）→ room.status=1 + SaveVersioned →
// room_log(type=5, extra 含 settle_type)。Settle 与房主退房都写全员含触发者本人。
// 传入的 room 必须是本事务 GetForUpdate 锁下的实例。
func (s *roomService) settleTx(ctx context.Context, tx *gorm.DB, room *model.Room, settleType int) error {
	roomRepo := repository.NewRoomRepository(tx)
	logRepo := repository.NewRoomLogRepository(tx)
	userRepo := repository.NewRoomUserRepository(tx)
	recRepo := repository.NewRecordRepository(tx)
	users, err := userRepo.ListByRoom(ctx, room.ID)
	if err != nil {
		return err
	}
	recs := make([]*model.Record, 0, len(users))
	for _, u := range users {
		recs = append(recs, &model.Record{
			TenantID:        room.TenantID,
			RoomID:          room.ID,
			UserID:          u.UserID,
			Score:           u.CurScore,
			CumLoseScore:    u.CumLoseScore,
			TeaPoolTotal:    room.TeaPoolTotal,
			TeaPercent:      room.TeaPercent,
			TeaTargetAmount: room.TeaTargetAmount,
			MaxLoseScore:    room.MaxLoseScore,
			SettleType:      settleType,
		})
	}
	if err := recRepo.Create(ctx, recs); err != nil {
		return err
	}
	room.Status = 1 // 已结算只读：Expense/Join 均拒（30104）
	if err := roomRepo.SaveVersioned(ctx, tx, room); err != nil {
		return err // 失败弃用内存 room（Version 已就地 +1，T5 契约）
	}
	// 结算流水：FromUserID=触发者。settleTx 仅由房主路径触发（Settle 校验房主、
	// 房主退房时触发者即 creator），故取 room.CreatorID。
	return logRepo.Create(ctx, &model.RoomLog{
		TenantID: room.TenantID, RoomID: room.ID, FromUserID: room.CreatorID,
		Type: 5, Extra: fmt.Sprintf(`{"settle_type":%d}`, settleType),
	})
}
