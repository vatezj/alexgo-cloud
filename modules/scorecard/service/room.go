package service

import (
	"context"
	"crypto/rand"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	membersvc "alexGo-cloud/modules/member/service"
	"alexGo-cloud/modules/scorecard/model"
	"alexGo-cloud/modules/scorecard/repository"
	errs "alexGo-cloud/pkg/errors"
)

// roomTTL 房间无操作超时：开房/进房/退房/配置变更刷 expire_time = now+2h（spec §4.2）。
const roomTTL = 2 * time.Hour

// roomCodeAlphabet 房号字符集：去掉易混的 I/O/0/1 共 32 字符（256 整除 32，取模无偏）。
const roomCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// roomCodeLen 房号位数（spec §4.2：4 位字母数字，全局唯一，冲突重试）。
const roomCodeLen = 4

// roomCodeMaxAttempts 服务端生成房号的尝试上限：首次 + 冲突重试 5 次，耗尽 → 30106。
const roomCodeMaxAttempts = 6

// recentLogLimit 快照内嵌最近流水条数（spec §4.1 快照含最近日志）。
const recentLogLimit = 20

// RoomService scorecard 房间服务：开房/进房/快照/流水分页/支出/改配置/结算/退房/转让房主。
// Task 9-13 在同一实现上追加记录查询等。
type RoomService interface {
	// Create 开房：房号可空（服务端生成 4 位），冲突重试耗尽 → 30106。
	Create(ctx context.Context, userID uint64, req CreateRoomReq) (*RoomSnapshot, error)
	// Join 进房：幂等（已在房返回现状），拉黑双向拦截 30108，重进清零为新会话。
	Join(ctx context.Context, userID uint64, roomCode string) (*RoomSnapshot, error)
	// Snapshot 房间全量快照：不存在/跨租户 30101，非成员 30102。
	Snapshot(ctx context.Context, userID, roomID uint64) (*RoomSnapshot, error)
	// Logs 流水分页（created_at 倒序，Task 12 app 控制器消费）。
	Logs(ctx context.Context, userID, roomID uint64, page, size int) ([]*model.RoomLog, int64, error)
	// Expense 记分支出核心事务：幂等/version 乐观锁/总分守恒/从下把生效/达标关/上限频控（spec §3.1）。
	Expense(ctx context.Context, userID, roomID uint64, req ExpenseReq) (*ExpenseResult, error)
	// UpdateConfig 改配置（仅房主；数值项写 pending「从下把生效」，spec §3.2）。
	UpdateConfig(ctx context.Context, userID, roomID uint64, req ConfigReq) (*RoomSnapshot, error)
	// Settle 结算（仅房主 30105）：全员 record 终局快照 + status=1 + type=5 流水（spec §3.3）。
	Settle(ctx context.Context, userID, roomID uint64) error
	// Leave 退房：房主先自动结算（settle_type=2）；普通成员写 record（3/4）后硬删成员行；
	// 最后一人离开 → status=2。重进按新会话从 session_no=1、计分清零起。
	Leave(ctx context.Context, userID, roomID uint64) error
	// TransferOwner 转让房主：目标须在房（30113）、拉黑双向拦截（30108）；
	// 两行 is_creator 互换 + creator_id + type=9 流水。
	TransferOwner(ctx context.Context, userID, roomID, targetUserID uint64) error
}

type roomService struct {
	db     *gorm.DB
	room   repository.RoomRepository
	member repository.RoomUserRepository
	log    repository.RoomLogRepository
	record repository.RecordRepository
	briefs membersvc.BriefProvider // import "alexGo-cloud/modules/member/service" 别名 membersvc
	black  repository.FriendBlackRepository
}

// NewRoomService 构造房间服务（fx 装配 Task 12 Provide；测试经 newRoomServiceForTest 构造）。
func NewRoomService(db *gorm.DB, room repository.RoomRepository, member repository.RoomUserRepository,
	log repository.RoomLogRepository, record repository.RecordRepository,
	briefs membersvc.BriefProvider, black repository.FriendBlackRepository) RoomService {
	return &roomService{db: db, room: room, member: member, log: log, record: record, briefs: briefs, black: black}
}

// Create 开房：房号（自定义冲突直接 30106 / 生成冲突重试 5 次）→ room 行（status=0、
// expire=now+2h、version=1）→ 房主成员行（is_creator=1, session_no=1）→ 进房流水（type=3）
// → 快照。中间步失败不做跨行回滚：唯一可失败点是库级错误，房号唯一键是首道写入
// （repository 的 Create 无 tx 参数，是 Task 5 既定接口；Expense/Settle 类多步事务在 Task 7+）。
func (s *roomService) Create(ctx context.Context, userID uint64, req CreateRoomReq) (*RoomSnapshot, error) {
	if userID == 0 || req.TeaPercent < 0 || req.TeaPercent > 100 ||
		req.TeaTargetAmount < 0 || req.MaxLoseScore < 0 {
		return nil, errs.New(errs.CodeParamInvalid, nil)
	}
	var room *model.Room
	if req.RoomCode != "" {
		// 自定义房号：冲突不重试，直接 30106。
		cand := newRoomRow(userID, req)
		if err := s.room.Create(ctx, cand); err != nil {
			if isUniqueViolation(err) {
				return nil, errs.New(errs.CodeScorecardRoomTaken, nil)
			}
			return nil, err
		}
		room = cand
	} else {
		// 服务端生成 4 位房号：唯一键冲突换码重试，耗尽 → 30106。
		for attempt := 0; attempt < roomCodeMaxAttempts; attempt++ {
			code, err := generateRoomCode()
			if err != nil {
				return nil, err // 熵源异常原样上抛（500）
			}
			cand := newRoomRow(userID, req)
			cand.RoomCode = code
			err = s.room.Create(ctx, cand)
			if err == nil {
				room = cand
				break
			}
			if !isUniqueViolation(err) {
				return nil, err
			}
		}
		if room == nil {
			return nil, errs.New(errs.CodeScorecardRoomTaken, nil)
		}
	}
	if err := s.member.Create(ctx, &model.RoomUser{
		RoomID: room.ID, UserID: userID, IsCreator: 1, SessionNo: 1,
	}); err != nil {
		return nil, err
	}
	if err := s.log.Create(ctx, &model.RoomLog{
		RoomID: room.ID, FromUserID: userID, Type: 3,
	}); err != nil {
		return nil, err
	}
	return s.buildSnapshot(ctx, s.db, room.ID, userID)
}

// newRoomRow 组装开房行：进行中 + expire=now+2h + version=1（乐观锁起点）。
// tenant_id 由 repository 打戳 + gormplugin Create 回填双保险。
func newRoomRow(userID uint64, req CreateRoomReq) *model.Room {
	teaAuto := 0
	if req.TeaAutoEnabled {
		teaAuto = 1
	}
	return &model.Room{
		RoomCode:        req.RoomCode, // 自定义房号；生成路径随后覆写为随机 4 位
		CreatorID:       userID,
		Status:          0,
		ExpireTime:      time.Now().Add(roomTTL),
		TeaTargetAmount: req.TeaTargetAmount,
		TeaPercent:      req.TeaPercent,
		TeaAutoEnabled:  teaAuto,
		MaxLoseScore:    req.MaxLoseScore,
		Version:         1,
	}
}

// Join 进房：房号查房 30101 → 状态校验（已结算 30104/已关闭 30103）→ 拉黑双向拦截 30108
// → 已在房幂等返回现状 → 新进房建行（session_no=1、计分清零 = 新会话）+ 进房流水
// （type=3）+ 刷 expire=now+2h。拉黑判定对全成员循环双向 Exists（单房 ≤20 人）。
func (s *roomService) Join(ctx context.Context, userID uint64, roomCode string) (*RoomSnapshot, error) {
	if userID == 0 || roomCode == "" {
		return nil, errs.New(errs.CodeParamInvalid, nil)
	}
	room, err := s.room.GetByCode(ctx, roomCode)
	if err != nil {
		return nil, err
	}
	if room == nil {
		return nil, errs.New(errs.CodeScorecardRoomNotFound, nil)
	}
	if room.Status == 1 {
		return nil, errs.New(errs.CodeScorecardRoomSettled, nil)
	}
	if room.Status != 0 {
		return nil, errs.New(errs.CodeScorecardRoomClosed, nil)
	}
	// 拉黑双向拦截：与任一现有成员互黑即拒。
	members, err := s.member.ListByRoom(ctx, room.ID)
	if err != nil {
		return nil, err
	}
	for _, m := range members {
		blocked, err := s.black.Exists(ctx, userID, m.UserID)
		if err != nil {
			return nil, err
		}
		if blocked {
			return nil, errs.New(errs.CodeScorecardBlacklisted, nil)
		}
	}
	// 幂等：已在房直接返回现状（不重复加人、不重复记流水、不刷过期）。
	if u, err := s.member.Get(ctx, room.ID, userID); err != nil {
		return nil, err
	} else if u != nil {
		return s.buildSnapshot(ctx, s.db, room.ID, userID)
	}
	// 新进房：建行（退房行已被硬删，重进从 session_no=1、计分为 0 起 = 新会话清零）。
	if err := s.member.Create(ctx, &model.RoomUser{
		RoomID: room.ID, UserID: userID, IsCreator: 0, SessionNo: 1,
	}); err != nil {
		return nil, err
	}
	if err := s.log.Create(ctx, &model.RoomLog{
		RoomID: room.ID, FromUserID: userID, Type: 3,
	}); err != nil {
		return nil, err
	}
	room.ExpireTime = time.Now().Add(roomTTL)
	if err := s.room.SaveVersioned(ctx, s.db, room); err != nil {
		return nil, err // 失败即弃用内存 room（version 就地 +1 语义见 Task 5 契约）
	}
	return s.buildSnapshot(ctx, s.db, room.ID, userID)
}

// Snapshot 房间快照：不存在/跨租户 30101（gormplugin 过滤后软查为 nil）→ 非成员 30102
// → 组装 DTO（昵称头像 + 最近 20 条流水）。
func (s *roomService) Snapshot(ctx context.Context, userID, roomID uint64) (*RoomSnapshot, error) {
	room, err := s.room.GetByID(ctx, roomID)
	if err != nil {
		return nil, err
	}
	if room == nil {
		return nil, errs.New(errs.CodeScorecardRoomNotFound, nil)
	}
	u, err := s.member.Get(ctx, roomID, userID)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, errs.New(errs.CodeScorecardNotMember, nil)
	}
	return s.buildSnapshot(ctx, s.db, roomID, userID)
}

// Logs 流水分页：不存在 30101 → 非成员 30102 → repo 透传（created_at 倒序）。
func (s *roomService) Logs(ctx context.Context, userID, roomID uint64, page, size int) ([]*model.RoomLog, int64, error) {
	room, err := s.room.GetByID(ctx, roomID)
	if err != nil {
		return nil, 0, err
	}
	if room == nil {
		return nil, 0, errs.New(errs.CodeScorecardRoomNotFound, nil)
	}
	u, err := s.member.Get(ctx, roomID, userID)
	if err != nil {
		return nil, 0, err
	}
	if u == nil {
		return nil, 0, errs.New(errs.CodeScorecardNotMember, nil)
	}
	return s.log.ListByRoom(ctx, roomID, page, size)
}

// UpdateConfig 改配置（仅房主 30105）：数值项（target/percent/maxLose）写 pending_* 暂存，
// pending_apply_round_no=round_no+1「从下把生效」（spec §3.2，当前把不受影响，下一笔支出时
// applyPending 整组刷入）。部分编辑不清场：首次暂存先按当前生效值种子全部 pending 槽、
// 再覆写提供的项；已有暂存（PendingApplyRoundNo>0）只覆写提供的项、保留其余暂存值。
// tea_auto_enabled 无 pending 槽位、非「下把生效」值，提供即改生效字段本身，不动 pending。
// 刷 expire=now+2h（配置变更 = 心跳）。单行 SaveVersioned（version 乐观锁）原子落库，
// 无需外层事务；失败弃用内存 room。
func (s *roomService) UpdateConfig(ctx context.Context, userID, roomID uint64, req ConfigReq) (*RoomSnapshot, error) {
	room, err := s.room.GetByID(ctx, roomID)
	if err != nil {
		return nil, err
	}
	if room == nil {
		return nil, errs.New(errs.CodeScorecardRoomNotFound, nil)
	}
	if room.CreatorID != userID {
		return nil, errs.New(errs.CodeScorecardNotOwner, nil)
	}
	if (req.TeaPercent != nil && (*req.TeaPercent < 0 || *req.TeaPercent > 100)) ||
		(req.TeaTargetAmount != nil && *req.TeaTargetAmount < 0) ||
		(req.MaxLoseScore != nil && *req.MaxLoseScore < 0) {
		return nil, errs.New(errs.CodeParamInvalid, nil)
	}
	// 数值项进 pending（「从下把生效」）；一项未提供则不动 pending（含仅改开关/空请求）。
	if req.TeaTargetAmount != nil || req.TeaPercent != nil || req.MaxLoseScore != nil {
		if room.PendingApplyRoundNo == 0 {
			// 首次暂存：种子当前生效值，applyPending 整组刷入时未提供的项不被清零。
			room.PendingTeaTargetAmount = room.TeaTargetAmount
			room.PendingTeaPercent = room.TeaPercent
			room.PendingMaxLoseScore = room.MaxLoseScore
		}
		if req.TeaTargetAmount != nil {
			room.PendingTeaTargetAmount = *req.TeaTargetAmount
		}
		if req.TeaPercent != nil {
			room.PendingTeaPercent = *req.TeaPercent
		}
		if req.MaxLoseScore != nil {
			room.PendingMaxLoseScore = *req.MaxLoseScore
		}
		room.PendingApplyRoundNo = room.RoundNo + 1
	}
	if req.TeaAutoEnabled != nil {
		teaAuto := 0
		if *req.TeaAutoEnabled {
			teaAuto = 1
		}
		room.TeaAutoEnabled = teaAuto
	}
	room.ExpireTime = time.Now().Add(roomTTL)
	if err := s.room.SaveVersioned(ctx, s.db, room); err != nil {
		return nil, err // 失败弃用内存 room（Version 已就地 +1 语义，T5 契约）
	}
	return s.buildSnapshot(ctx, s.db, roomID, userID)
}

// buildSnapshot 组装房间全量快照（开房/进房/快照接口与 Task 7 expense 事务内共用）。
// 全部读走传入的 tx：事务内可读未提交状态；非事务路径传 s.db。
// viewerID 在房校验（30102，viewerID=0 跳过）；昵称头像经 member BriefProvider 批量补，
// 拉取失败降级空昵称不阻断接口（快照可用性优先，昵称下次拉取自愈）。
func (s *roomService) buildSnapshot(ctx context.Context, tx *gorm.DB, roomID, viewerID uint64) (*RoomSnapshot, error) {
	room, err := s.room.GetForUpdate(ctx, tx, roomID)
	if err != nil {
		return nil, err
	}
	if room == nil {
		return nil, errs.New(errs.CodeScorecardRoomNotFound, nil)
	}
	var rows []*model.RoomUser
	if err := tx.WithContext(ctx).
		Where("room_id = ?", roomID).Order("id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	members := make([]RoomMember, 0, len(rows))
	ids := make([]uint64, 0, len(rows))
	inRoom := false
	for _, r := range rows {
		if r.UserID == viewerID {
			inRoom = true
		}
		ids = append(ids, r.UserID)
		members = append(members, RoomMember{
			UserID:       r.UserID,
			CurScore:     r.CurScore,
			CumLoseScore: r.CumLoseScore,
			IsCreator:    r.IsCreator == 1,
		})
	}
	if viewerID != 0 && !inRoom {
		return nil, errs.New(errs.CodeScorecardNotMember, nil)
	}
	// 昵称头像批量补齐（失败降级：留空）。
	if len(ids) > 0 && s.briefs != nil {
		if briefs, err := s.briefs.GetBriefs(ctx, ids); err == nil {
			byID := make(map[uint64]membersvc.UserBrief, len(briefs))
			for _, b := range briefs {
				byID[b.ID] = b
			}
			for i := range members {
				if b, ok := byID[members[i].UserID]; ok {
					members[i].Nickname = b.Nickname
					members[i].Avatar = b.Avatar
				}
			}
		}
	}
	// 最近 20 条流水（created_at 倒序，等价 log.ListByRoom(page=1,size=20)，读走 tx）。
	var logRows []*model.RoomLog
	if err := tx.WithContext(ctx).
		Where("room_id = ?", roomID).
		Order("created_at desc, id desc").Limit(recentLogLimit).Find(&logRows).Error; err != nil {
		return nil, err
	}
	logs := make([]RoomLogItem, 0, len(logRows))
	for _, l := range logRows {
		logs = append(logs, RoomLogItem{
			ID:              l.ID,
			FromUserID:      l.FromUserID,
			ToUserID:        l.ToUserID,
			Score:           l.Score,
			Tea:             l.Tea,
			Type:            l.Type,
			ClientRequestID: l.ClientRequestID,
			CreatedAt:       l.CreatedAt,
		})
	}
	return &RoomSnapshot{
		ID:                  room.ID,
		RoomCode:            room.RoomCode,
		CreatorID:           room.CreatorID,
		Status:              room.Status,
		Version:             room.Version,
		ExpireTime:          room.ExpireTime,
		TeaTargetAmount:     room.TeaTargetAmount,
		TeaPercent:          room.TeaPercent,
		TeaPoolTotal:        room.TeaPoolTotal,
		TeaAutoEnabled:      room.TeaAutoEnabled == 1,
		MaxLoseScore:        room.MaxLoseScore,
		RoundNo:             room.RoundNo,
		PendingApplyRoundNo: room.PendingApplyRoundNo,
		Members:             members,
		RecentLogs:          logs,
	}, nil
}

// generateRoomCode crypto/rand 从去混淆字符集取 roomCodeLen 位房号。
func generateRoomCode() (string, error) {
	buf := make([]byte, roomCodeLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out := make([]byte, roomCodeLen)
	for i, b := range buf {
		out[i] = roomCodeAlphabet[int(b)%len(roomCodeAlphabet)]
	}
	return string(out), nil
}

// isUniqueViolation 唯一键冲突判定：sqlite「UNIQUE constraint failed」/ MySQL 1062
// 「Duplicate entry」/ PG 23505「duplicate key value」；TranslateError 开启时另认
// gorm.ErrDuplicatedKey。
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") ||
		strings.Contains(msg, "Duplicate entry") ||
		strings.Contains(msg, "duplicate key value") ||
		strings.Contains(msg, "SQLSTATE 23505")
}
