package social

import (
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"go.uber.org/fx"
	"gorm.io/gorm"

	membermodel "alexGo-cloud/modules/member/model"
	oauth2svc "alexGo-cloud/modules/system/oauth2/service"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/tenant"
	"alexGo-cloud/pkg/token"
)

// MemberStore 会员用户窄接口（Task 6 Produces 契约）：
// 由 modules/member 的 MemberRepository 映射提供（member/module.go），
// system 微服务进程无 member 模块 → SocialDeps.Members optional 为 nil。
// GetByID 含软删行（调用方按 Deleted/Status 判定，见 member repo 口径）。
type MemberStore interface {
	GetByID(ctx context.Context, tenantID, id uint64) (*membermodel.MemberUser, error)
	GetByMobile(ctx context.Context, tenantID uint64, mobile string) (*membermodel.MemberUser, error)
	Create(ctx context.Context, u *membermodel.MemberUser) error
	Update(ctx context.Context, u *membermodel.MemberUser) error
}

// SocialDeps Fx 装配结构：必填 Repo/Adapters/Issuer，
// Members/Auditor/Limit optional（跨进程形态下对应实现缺席，见各字段注释）。
type SocialDeps struct {
	fx.In
	Repo     SocialRepository
	Members  MemberStore `optional:"true"` // system 进程无 member repo
	Adapters *AdapterRegistry
	Issuer   token.Issuer
	Auditor  oauth2svc.LoginAuditor     `optional:"true"` // member 进程无 system audit → nil 安全跳过
	Limit    tenant.AccountLimitChecker `optional:"true"`
}

// SocialService 第三方登录门面：Task 6 登录（三分支解析 + 签发 + 审计），
// Task 7 在本类型上追加绑定/合并/解绑方法。
type SocialService struct {
	repo     SocialRepository
	members  MemberStore
	adapters *AdapterRegistry
	issuer   token.Issuer
	auditor  oauth2svc.LoginAuditor
	limit    tenant.AccountLimitChecker
}

// NewSocialService 构造（fx 按 SocialDeps 注入）。
func NewSocialService(deps SocialDeps) *SocialService {
	return &SocialService{
		repo:     deps.Repo,
		members:  deps.Members,
		adapters: deps.Adapters,
		issuer:   deps.Issuer,
		auditor:  deps.Auditor,
		limit:    deps.Limit,
	}
}

// platformLabels 登录审计文案（按平台区分，供 RecordLogin 的 msg 拼接）。
var platformLabels = map[string]string{
	PlatformWechatMP:  "微信小程序登录",
	PlatformWechatWeb: "微信网页授权登录",
}

// Login 第三方登录主流程：取渠道配置 → code 换身份 → 三分支解析（spec §3）
// → 签发令牌 → 恰一次成功审计（auditor 未接线时静默跳过；
// 失败路径不记审计——与统一登录的分野：第三方登录无凭据可审）。
func (s *SocialService) Login(ctx context.Context, platform, code, ip, ua string) (*oauth2svc.LoginResult, error) {
	tid := tenant.TenantIDFromContext(ctx)
	ad, ok := s.adapters.Get(platform)
	if !ok {
		return nil, errs.New(errs.CodeSocialAppNotConfigured, nil)
	}
	app, err := s.repo.GetApp(ctx, tid, platform)
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && app.Status != 1) {
		return nil, errs.New(errs.CodeSocialAppNotConfigured, err)
	}
	if err != nil {
		return nil, err
	}
	ident, err := ad.Exchange(ctx, app, code)
	if err != nil {
		// 平台原始错误只记日志，不上抛 errcode（spec §3）
		log.Printf("social: exchange failed platform=%s: %v", platform, err)
		return nil, errs.New(errs.CodeSocialExchangeFailed, err)
	}
	u, res, err := s.resolveAndIssue(ctx, tid, platform, ident, ip, ua)
	if err != nil {
		return nil, err
	}
	if s.auditor != nil {
		_ = s.auditor.RecordLogin(ctx, u.Nickname, u.ID, ip, ua, true,
			"social: "+platformLabels[platform]+" openid="+ident.OpenID)
	}
	return res, nil
}

// resolveAndIssue 三分支解析 + 签发（spec §3）：
//  1. (tenant, platform, openid) 命中 → 校验会员存活 → 签发；
//  2. 未命中且 unionid 非空 → 同 unionid 命中 → 补建绑定行 → 签发；
//  3. 都没有 → 额度检查 → 创建空壳会员 + 绑定行 → 签发。
//
// 返回命中的会员（供审计取昵称）与登录结果。
// ua 为签名保留位（审计在 Login 内做，本方法不消费）。
func (s *SocialService) resolveAndIssue(ctx context.Context, tid uint64, platform string, ident *Identity, ip, ua string) (*membermodel.MemberUser, *oauth2svc.LoginResult, error) {
	// 分支 1：openid 直接命中既有绑定
	idn, err := s.repo.GetIdentity(ctx, tid, platform, ident.OpenID)
	if err == nil {
		return s.issueForMember(ctx, tid, idn.MemberUserID)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, err
	}
	// 分支 2：unionid 同人补绑（unionid 空串不参与同人匹配——GetIdentityByUnion SQL 层已防）
	if ident.UnionID != "" {
		uidn, uerr := s.repo.GetIdentityByUnion(ctx, tid, ident.UnionID)
		if uerr == nil {
			if cerr := s.linkIdentity(ctx, tid, platform, ident, uidn.MemberUserID); cerr != nil {
				return nil, nil, cerr
			}
			return s.issueForMember(ctx, tid, uidn.MemberUserID)
		}
		if !errors.Is(uerr, gorm.ErrRecordNotFound) {
			return nil, nil, uerr
		}
	}
	// 分支 3：全新注册空壳会员
	if s.members == nil {
		return nil, nil, oauth2svc.ErrAuthenticatorMissing
	}
	if s.limit != nil {
		if err := s.limit.CheckAccountLimit(ctx, tid); err != nil {
			return nil, nil, err // 额度错误原样透传（不包业务码）
		}
	}
	now := time.Now()
	shell := &membermodel.MemberUser{
		TenantID:   tid,
		Nickname:   sanitizeNickname(ident.Nickname, ident.OpenID),
		Avatar:     ident.Avatar,
		Status:     1,
		Mobile:     "",
		Password:   "",
		RegisterIP: ip,
		LoginDate:  &now,
	}
	if err := s.members.Create(ctx, shell); err != nil {
		return nil, nil, err
	}
	// TenantID 显式带上——CreateIdentity 走 IgnoreTenant，漏传会落错租户（T3 契约）
	if err := s.repo.CreateIdentity(ctx, &MemberIdentity{
		TenantID:     tid,
		MemberUserID: shell.ID,
		Platform:     platform,
		OpenID:       ident.OpenID,
		UnionID:      ident.UnionID,
	}); err != nil {
		return nil, nil, err
	}
	res, err := s.issue(ctx, shell)
	if err != nil {
		return nil, nil, err
	}
	return shell, res, nil
}

// issueForMember 分支 1/2 共用：取会员（未接线/软删/停用判定）→ 签发。
func (s *SocialService) issueForMember(ctx context.Context, tid, memberID uint64) (*membermodel.MemberUser, *oauth2svc.LoginResult, error) {
	u, err := s.loadMember(ctx, tid, memberID)
	if err != nil {
		return nil, nil, err
	}
	res, err := s.issue(ctx, u)
	if err != nil {
		return nil, nil, err
	}
	return u, res, nil
}

// loadMember 取绑定会员并验存活：
// Members 未接线 → ErrAuthenticatorMissing；
// 行不存在 / 软删 / 停用（u.Deleted || u.Status != 1）→ 30002 用户不存在。
func (s *SocialService) loadMember(ctx context.Context, tid, memberID uint64) (*membermodel.MemberUser, error) {
	if s.members == nil {
		return nil, oauth2svc.ErrAuthenticatorMissing
	}
	u, err := s.members.GetByID(ctx, tid, memberID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.New(errs.CodeUserNotFound, nil)
		}
		return nil, err
	}
	if u.Deleted || u.Status != 1 {
		return nil, errs.New(errs.CodeUserNotFound, nil)
	}
	return u, nil
}

// linkIdentity 把 openid 补绑到 unionid 命中的会员。
// 唯一键 (tenant, platform, openid) 竞态（并发同 openid 登录）时重新 Get：
// 归属同一会员则幂等继续；否则回传首个插入错误（归属以先落库者为准，本次请求失败）。
func (s *SocialService) linkIdentity(ctx context.Context, tid uint64, platform string, ident *Identity, memberID uint64) error {
	// TenantID 显式带上——CreateIdentity 走 IgnoreTenant（T3 契约）
	err := s.repo.CreateIdentity(ctx, &MemberIdentity{
		TenantID:     tid,
		MemberUserID: memberID,
		Platform:     platform,
		OpenID:       ident.OpenID,
		UnionID:      ident.UnionID,
	})
	if err == nil {
		return nil
	}
	again, gerr := s.repo.GetIdentity(ctx, tid, platform, ident.OpenID)
	if gerr != nil || again.MemberUserID != memberID {
		return err // 无法确认归属或已绑他人：回传插入错误
	}
	return nil // 幂等：同一会员已持有该 openid
}

// issue 按 member 账号类型签发令牌；任何签发失败 wrap ErrIssuance。
func (s *SocialService) issue(ctx context.Context, u *membermodel.MemberUser) (*oauth2svc.LoginResult, error) {
	if s.issuer == nil {
		return nil, fmt.Errorf("%w: issuer not wired", oauth2svc.ErrIssuance)
	}
	issued, err := s.issuer.Issue(ctx, token.IssueParams{
		UserID: u.ID, TenantID: u.TenantID, ClientID: "alexgo-app", UserType: token.UserTypeMember,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", oauth2svc.ErrIssuance, err)
	}
	return &oauth2svc.LoginResult{
		AccessToken:  issued.AccessToken,
		RefreshToken: issued.RefreshToken,
		ExpiresIn:    issued.ExpiresIn,
	}, nil
}

// sanitizeNickname 第三方昵称落库前清洗：
//   - ':' 必须去掉——member 子策略命名空间 {tid}:role:{code} 与用户命名空间
//     {tid}:{ut}:{nickname} 撞车即 casbin 提权；
//   - 清洗并 trim 后为空（微信匿名化昵称常为空）→ 占位 "微信用户"+openid 尾 4
//     （token.Validate 拒空昵称）。
func sanitizeNickname(nick, openID string) string {
	n := strings.TrimSpace(strings.ReplaceAll(nick, ":", ""))
	if n == "" {
		n = "微信用户" + lastN(openID, 4)
	}
	return n
}

// lastN 取字符串尾 n 个字节（openid 为 ASCII 十六进制，按字节安全）；
// 不足 n 返回全串。
func lastN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

// mobileRe 大陆手机号格式（Task 7 首次引入，BindMobile 入参校验）。
var mobileRe = regexp.MustCompile(`^1[3-9]\d{9}$`)

// BindSocial 把当前登录会员绑定一个第三方身份（spec §5 绑定）：
// 未知平台 10001（用户输入）→ 渠道未配置/停用 30301 → code 换身份 30302
// → 已绑自己幂等 nil / 已绑他人 30303（不迁移）→ 落绑定行。
func (s *SocialService) BindSocial(ctx context.Context, memberID uint64, platform, code string) error {
	if !s.adapters.Exists(platform) {
		return errs.New(errs.CodeParamInvalid, nil) // 未知平台是用户输入
	}
	ad, _ := s.adapters.Get(platform)
	tid := tenant.TenantIDFromContext(ctx)
	app, err := s.repo.GetApp(ctx, tid, platform)
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && app.Status != 1) {
		return errs.New(errs.CodeSocialAppNotConfigured, err) // 未配置/停用
	}
	if err != nil {
		return err
	}
	if code == "" {
		return errs.New(errs.CodeParamMissing, nil)
	}
	ident, err := ad.Exchange(ctx, app, code)
	if err != nil {
		log.Printf("social: bind exchange failed platform=%s: %v", platform, err)
		return errs.New(errs.CodeSocialExchangeFailed, err)
	}
	idn, err := s.repo.GetIdentity(ctx, tid, platform, ident.OpenID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if idn != nil {
		if idn.MemberUserID == memberID {
			return nil // 已绑自己 → 幂等成功
		}
		return errs.New(errs.CodeSocialIdentityBound, nil) // 已绑他人，不迁移
	}
	return s.repo.CreateIdentity(ctx, &MemberIdentity{
		TenantID: tid, MemberUserID: memberID,
		Platform: platform, OpenID: ident.OpenID, UnionID: ident.UnionID,
	})
}

// BindMobile 绑定手机号（spec §5）：格式 10001 → 会员不存在 30002 → 已有手机号
// 30304（不允许改绑）→ 号码空闲直接落库；号码被占仅空壳（无手机号且无密码）自动
// 合并：identity 全部改指目标 → 当前空壳软删；非空壳 30305 拒绝（Review Focus #4）。
func (s *SocialService) BindMobile(ctx context.Context, memberID uint64, mobile string) error {
	if !mobileRe.MatchString(mobile) {
		return errs.New(errs.CodeParamInvalid, nil)
	}
	if s.members == nil {
		return oauth2svc.ErrAuthenticatorMissing
	}
	tid := tenant.TenantIDFromContext(ctx)
	me, err := s.members.GetByID(ctx, tid, memberID)
	if err != nil || me.Deleted {
		return errs.New(errs.CodeUserNotFound, err)
	}
	if me.Mobile != "" {
		return errs.New(errs.CodeSocialMobileBound, nil) // 已绑定，不允许改绑
	}
	taken, err := s.members.GetByMobile(ctx, tid, mobile)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		me.Mobile = mobile
		return s.members.Update(ctx, me)
	}
	if err != nil {
		return err
	}
	// mobile 已被占用 → 仅空壳自动合并（Review Focus #4）
	isShell := me.Mobile == "" && me.Password == ""
	if !isShell {
		return errs.New(errs.CodeSocialMergeRefused, nil)
	}
	if _, err := s.repo.ReassignIdentities(ctx, tid, me.ID, taken.ID); err != nil {
		return err // 显式 tenant 过滤：跨租户 identity 不改指
	}
	me.Deleted = true // 空壳软删；业务数据不迁移（空壳刚注册无业务数据）
	return s.members.Update(ctx, me)
}

// UnbindSocial 解绑第三方身份（spec §5）：无该平台绑定幂等 nil；守门 30306——
// 解绑后必须仍剩至少一种登录方式（有密码或其他平台 identity），否则拒绝防锁死
// （Review Focus #7）。
func (s *SocialService) UnbindSocial(ctx context.Context, memberID uint64, platform string) error {
	if !s.adapters.Exists(platform) {
		return errs.New(errs.CodeParamInvalid, nil)
	}
	tid := tenant.TenantIDFromContext(ctx)
	list, err := s.repo.ListIdentities(ctx, tid, memberID)
	if err != nil {
		return err
	}
	var target bool
	others := 0
	for _, idn := range list {
		if idn.Platform == platform {
			target = true
		} else {
			others++
		}
	}
	if !target {
		return nil // 幂等：没有该平台绑定
	}
	if s.members == nil {
		return oauth2svc.ErrAuthenticatorMissing
	}
	me, err := s.members.GetByID(ctx, tid, memberID)
	if err != nil {
		return errs.New(errs.CodeUserNotFound, err)
	}
	if me.Password == "" && others == 0 {
		return errs.New(errs.CodeSocialUnbindLast, nil) // Review Focus #7：防锁死
	}
	_, err = s.repo.DeleteIdentity(ctx, tid, memberID, platform)
	return err
}
