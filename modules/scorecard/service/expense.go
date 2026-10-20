package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"gorm.io/gorm"

	"alexGo-cloud/modules/scorecard/model"
	"alexGo-cloud/modules/scorecard/repository"
	errs "alexGo-cloud/pkg/errors"
)

// ExpenseReq 记分支出请求。Score 输家分 S（>0）；Tea 仅手动 DIY 模式（未开自动百分比）生效，
// 单笔 ≤ maxTeaPerExpense；ClientRequestID 房间内幂等键（重放返回上次结果，不重复记账）。
type ExpenseReq struct {
	Score           int    `json:"score"`
	Tea             int    `json:"tea"`
	ToUserID        uint64 `json:"to_user_id"`
	TeaFromUserID   uint64 `json:"tea_from_user_id"` // 手动 DIY 来源，缺省=支出方；非 0 时须在房（30113）
	ClientRequestID string `json:"client_request_id"`
}

// ExpenseResult 支出结果：最新快照 + 流水号 + 实收茶饭 + 首次达上限（全房提醒）成员。
type ExpenseResult struct {
	Snapshot       *RoomSnapshot `json:"snapshot"`
	LogID          uint64        `json:"log_id"`
	TeaCharged     int           `json:"tea_charged"`
	AlarmedUserIDs []uint64      `json:"alarmed_user_ids"` // 首次达上限（全房提醒）的成员
}

// ConfigReq 改配置请求：指针字段 = 只改提供的项；数值项「从下把生效」写 pending_*（spec §3.2）。
type ConfigReq struct {
	TeaTargetAmount *int     `json:"tea_target_amount"`
	TeaPercent      *float64 `json:"tea_percent"`
	TeaAutoEnabled  *bool    `json:"tea_auto_enabled"`
	MaxLoseScore    *int     `json:"max_lose_score"`
}

// maxTeaPerExpense 单笔茶饭（DIY 手动）上限：与 score 侧同量级的参数保险丝，
// 超限在事务前直接 10001 拒（防误传多打零把池打爆）。
const maxTeaPerExpense = 1_000_000

// Expense 记分支出核心事务（spec §3.1）：FOR UPDATE 锁房行 → 幂等命中回放 →
// round_no+1 应用到期 pending → 茶饭计算（自动 T=round(S×percent) 从赢家所得扣 /
// 手动 DIY 从 tea_from_user_id 扣 T 进池）→ 记账（表达式列更新，总分守恒）→
// 达标自动关 → 上限频控（extra.session_no 判本会话首提醒）→ type=1 流水 →
// version 乐观锁落库。任一步失败整体回滚（spec §5.3）。
//
// 事务内所有库访问经 tx 绑定的 repo 实例（ruling：回调里再用根 db 的 repo 会拿到池上
// 另一条连接，MySQL 下逃出原子边界、sqlite :memory: 下直接空库报错）；幂等查重在
// 事务内、行锁之后执行——锁定读先读到最新已提交行，其后的普通 SELECT 视图才不会
// 漏掉并发已提交的同键流水（锁前查重在 MySQL RR 下可双记）。
func (s *roomService) Expense(ctx context.Context, userID, roomID uint64, req ExpenseReq) (*ExpenseResult, error) {
	if req.Score <= 0 || req.ToUserID == 0 || req.ToUserID == userID || req.ClientRequestID == "" ||
		req.Tea > maxTeaPerExpense {
		return nil, errs.New(errs.CodeParamInvalid, nil)
	}
	var out *ExpenseResult
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// tx 绑定 repo：事务内全部读写走同一条连接。
		roomRepo := repository.NewRoomRepository(tx)
		logRepo := repository.NewRoomLogRepository(tx)
		userRepo := repository.NewRoomUserRepository(tx)
		// ① 锁房行（FOR UPDATE 串行化同房支出；version 乐观锁是第二道防线）。
		room, err := roomRepo.GetForUpdate(ctx, tx, roomID)
		if err != nil {
			return err
		}
		if room == nil {
			return errs.New(errs.CodeScorecardRoomNotFound, nil)
		}
		if room.Status == 1 {
			return errs.New(errs.CodeScorecardRoomSettled, nil)
		}
		if room.Status != 0 {
			return errs.New(errs.CodeScorecardRoomClosed, nil)
		}
		if u, err := userRepo.Get(ctx, roomID, userID); err != nil {
			return err
		} else if u == nil {
			return errs.New(errs.CodeScorecardNotMember, nil)
		}
		if to, err := userRepo.Get(ctx, roomID, req.ToUserID); err != nil {
			return err
		} else if to == nil {
			return errs.New(errs.CodeScorecardTargetNotMember, nil)
		}
		// ② 幂等：命中即回放。查重必须在行锁之后：锁定读先读到最新已提交行，其后的
		// 普通 SELECT 才建立在能看到并发已提交流水的视图；放锁前（MySQL RR）快照
		// 可能先于对方提交，并发同 client_request_id 会双记。重放语义：同 LogID、
		// prev.Tea，零重记。
		if prev, err := logRepo.GetByRequestID(ctx, roomID, req.ClientRequestID); err != nil {
			return err
		} else if prev != nil {
			snap, err := s.buildSnapshot(ctx, tx, roomID, userID)
			if err != nil {
				return err
			}
			out = &ExpenseResult{Snapshot: snap, LogID: prev.ID, TeaCharged: prev.Tea}
			return nil
		}
		// ③ 把序号 +1，应用到期 pending（「从下把生效」：推进到 pending 目标把则刷入并清空）。
		room.RoundNo++
		applyPending(room)
		// ④ 茶饭计算：自动 T=round(S×percent) 从赢家所得扣（守恒口径，spec §3.1.5）；
		// 手动 DIY 从 tea_from_user_id（缺省=支出方）扣 T 进池，T 不计 cum_lose_score。
		tea, srcUserID := 0, userID
		auto := room.TeaAutoEnabled == 1 && room.TeaPercent > 0 &&
			(room.TeaTargetAmount <= 0 || room.TeaPoolTotal < room.TeaTargetAmount)
		if auto {
			tea = int(math.Round(float64(req.Score) * room.TeaPercent / 100))
			srcUserID = req.ToUserID // 总分守恒：从赢家所得扣
		} else if req.Tea > 0 {
			tea = req.Tea // 手动 DIY
			if req.TeaFromUserID != 0 {
				// 来源须在房（与 to_user_id 同款校验，同码 30113）：AddScore 对 0 行静默，
				// 非成员来源会让池照加而无人扣分 → 总分守恒破坏。
				if from, err := userRepo.Get(ctx, roomID, req.TeaFromUserID); err != nil {
					return err
				} else if from == nil {
					return errs.New(errs.CodeScorecardTargetNotMember, nil)
				}
				srcUserID = req.TeaFromUserID
			}
		}
		// ⑤ 记账（表达式列更新，事务内；输家 -S、cum_lose+=S）。
		if err := userRepo.AddScore(ctx, tx, roomID, userID, -req.Score); err != nil {
			return err
		}
		if err := userRepo.AddCumLose(ctx, tx, roomID, userID, req.Score); err != nil {
			return err
		}
		if tea > 0 && auto {
			// 自动：赢家净得 S-T（spec §3.1.5「赢家 cur_score+=S-T」单语句），茶饭入池。
			if err := userRepo.AddScore(ctx, tx, roomID, req.ToUserID, req.Score-tea); err != nil {
				return err
			}
			room.TeaPoolTotal += tea
		} else if tea > 0 { // DIY：赢家全额，来源扣 T
			if err := userRepo.AddScore(ctx, tx, roomID, srcUserID, -tea); err != nil {
				return err
			}
			if err := userRepo.AddScore(ctx, tx, roomID, req.ToUserID, req.Score); err != nil {
				return err
			}
			room.TeaPoolTotal += tea
		} else {
			if err := userRepo.AddScore(ctx, tx, roomID, req.ToUserID, req.Score); err != nil {
				return err
			}
		}
		alarmed := []uint64{}
		// ⑥ 达标自动关：池达目标即关自动抽取 + type=2 提示日志。
		if room.TeaTargetAmount > 0 && room.TeaPoolTotal >= room.TeaTargetAmount && room.TeaAutoEnabled == 1 {
			room.TeaAutoEnabled = 0
			if err := logRepo.Create(ctx, &model.RoomLog{TenantID: room.TenantID, RoomID: roomID,
				FromUserID: userID, Type: 2, Tea: tea,
				Extra: `{"event":"tea_target_reached"}`}); err != nil {
				return err
			}
		}
		// ⑦ 上限频控：本会话首次达上限全房提醒（记入 AlarmedUserIDs），之后仅写日志。
		// 会话判定：日志 extra 存 session_no，等于成员当前 session_no 即视为本会话已提醒。
		// 已知限制（此处只记实情、不改逻辑）：Join 是成员行硬删重建、恒 session_no=1
		// （room.go Create/Join 同款），重进房后老 type=6 日志的 session_no 也是 1 → 依然匹配，
		// 所以「跨会话重新首提醒」并不重现——首提醒跨会话不重现属已知遗留；
		// 要真按会话区分，需让 session_no 递增或日志另存会话标识。
		if room.MaxLoseScore > 0 {
			users, err := userRepo.ListByRoom(ctx, roomID)
			if err != nil {
				return err
			}
			for _, u := range users {
				if u.CumLoseScore < room.MaxLoseScore {
					continue
				}
				first := true
				// T5 接口签名 roomID/userID 为 int（repository/room_log.go），此处收窄转换。
				recent, err := logRepo.ListTypeByUser(ctx, int(roomID), int(u.UserID), 6, 20)
				if err != nil {
					return err
				}
				for _, l := range recent {
					var e struct {
						SessionNo int `json:"session_no"`
					}
					if json.Unmarshal([]byte(l.Extra), &e) == nil && e.SessionNo == u.SessionNo {
						first = false
						break
					}
				}
				if err := logRepo.Create(ctx, &model.RoomLog{TenantID: room.TenantID, RoomID: roomID,
					FromUserID: u.UserID, Type: 6, Score: u.CumLoseScore,
					Extra: fmt.Sprintf(`{"cum_lose":%d,"limit":%d,"session_no":%d}`, u.CumLoseScore, room.MaxLoseScore, u.SessionNo)}); err != nil {
					return err
				}
				if first {
					alarmed = append(alarmed, u.UserID)
				}
			}
		}
		// ⑧ 记分日志 + 版本落库（SaveVersioned：WHERE id AND version，RowsAffected=0 → 30112）。
		logRow := &model.RoomLog{TenantID: room.TenantID, RoomID: roomID, FromUserID: userID,
			ToUserID: req.ToUserID, Score: req.Score, Tea: tea, Type: 1, ClientRequestID: req.ClientRequestID}
		if err := logRepo.Create(ctx, logRow); err != nil {
			return err
		}
		room.ExpireTime = time.Now().Add(roomTTL)
		if err := roomRepo.SaveVersioned(ctx, tx, room); err != nil {
			return err // err 上抛即整体回滚；此后不再触碰内存 room（Version 已就地 +1，T5 契约）
		}
		snap, err := s.buildSnapshot(ctx, tx, roomID, userID)
		if err != nil {
			return err
		}
		out = &ExpenseResult{Snapshot: snap, LogID: logRow.ID, TeaCharged: tea, AlarmedUserIDs: alarmed}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// applyPending 「从下把生效」：当前把序号已推进到 pending 目标时刷入并清空。
// 调用方必须在 room.RoundNo++ 之后调用——pending 在「下一笔」把序号达到
// pending_apply_round_no 时生效，当前把用旧配置（spec §3.1.4/§3.2）。
func applyPending(r *model.Room) {
	if r.PendingApplyRoundNo > 0 && r.RoundNo >= r.PendingApplyRoundNo {
		r.TeaTargetAmount = r.PendingTeaTargetAmount
		r.TeaPercent = r.PendingTeaPercent
		r.MaxLoseScore = r.PendingMaxLoseScore
		r.PendingApplyRoundNo, r.PendingTeaTargetAmount, r.PendingTeaPercent, r.PendingMaxLoseScore = 0, 0, 0, 0
	}
}
