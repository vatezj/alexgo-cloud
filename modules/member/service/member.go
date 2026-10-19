package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"alexGo-cloud/modules/member/model"
	"alexGo-cloud/modules/member/repository"
	oauth2svc "alexGo-cloud/modules/system/oauth2/service"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/tenant"
	"alexGo-cloud/pkg/token"
)

// mobileRe 中国大陆手机号（1 开头、第二位 3-9、共 11 位）。
var mobileRe = regexp.MustCompile(`^1[3-9]\d{9}$`)

// ErrIssuance 签发失败哨兵：密码/状态校验已通过，是 token.Issuer 侧故障
// （gRPC 不可达、DB 故障、issuer 未装配）→ controller 映射 503，
// 与"凭据错误 401"区分——否则 infra 故障会被误报成"密码错误"（I2）。
var ErrIssuance = errors.New("issuance failed")

// ErrInvalidStatus 状态值越界哨兵：status ∉ {0,1} → controller 映射 400（T5 deferred）。
var ErrInvalidStatus = errors.New("invalid status")

// LoginResult 与 system 同形：双端登录/刷新的统一响应（controller 直接序列化）。
type LoginResult struct {
	AccessToken  string `json:"token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

// MemberService 会员用户领域服务（app 端注册/登录/刷新/登出 + admin 端列表/启停）。
type MemberService interface {
	Register(ctx context.Context, mobile, password, nickname, ip string) (*model.MemberUser, error)
	Authenticate(ctx context.Context, mobile, password, ip string) (*oauth2svc.AuthIdentity, error)
	Login(ctx context.Context, mobile, password, ip string) (*LoginResult, error)
	Refresh(ctx context.Context, refreshToken string) (*LoginResult, error)
	Logout(ctx context.Context, accessToken string) error
	List(ctx context.Context, page, size int) ([]*model.MemberUser, int64, error)
	Disable(ctx context.Context, id uint64, status int) error
	// Export 租户内全量会员（导出用；与 List 同查询口径、不截断不分页）。
	Export(ctx context.Context) ([]*model.MemberUser, error)
}

type memberService struct {
	repo   repository.MemberRepository
	issuer token.Issuer
	// limit 账号额度检查（实现由 system 模块 Provide，FX 按 pkg/tenant 窄接口注入）；
	// 可为 nil（未装配额度）→ Register 跳过检查。
	limit tenant.AccountLimitChecker
}

// NewMemberService 构造服务；token.Issuer 由入口（cmd/main.go）映射提供、
// tenant.AccountLimitChecker 由 system 模块 FxModule 映射提供，均不在本模块内重复 Provide。
func NewMemberService(repo repository.MemberRepository, issuer token.Issuer, limit tenant.AccountLimitChecker) MemberService {
	return &memberService{repo: repo, issuer: issuer, limit: limit}
}

func toResult(i *token.Issued) *LoginResult {
	return &LoginResult{AccessToken: i.AccessToken, RefreshToken: i.RefreshToken, ExpiresIn: i.ExpiresIn}
}

// Register 校验手机号/密码 → 查重 → 账号额度 → bcrypt 落库；昵称缺省用手机号尾 4 位占位。
func (s *memberService) Register(ctx context.Context, mobile, password, nickname, ip string) (*model.MemberUser, error) {
	if !mobileRe.MatchString(mobile) {
		return nil, errs.New(errs.CodeParamInvalid, fmt.Errorf("invalid mobile"))
	}
	if len(password) < 8 {
		return nil, errs.New(errs.CodeParamInvalid, fmt.Errorf("password too short (min 8)"))
	}
	// 昵称禁止含 ':'：角色 sub 命名空间为 "{tid}:role:{code}"，用户 sub 为
	// "{tid}:{ut}:{nickname}"。若昵称可含 ':'，注册昵称 "role:admin" 会得到
	// "{tid}:role:admin"，与管理员角色 sub 撞车 → casbin g(x,x) 恒等 → 提权。
	if strings.Contains(nickname, ":") {
		return nil, errs.New(errs.CodeParamInvalid, fmt.Errorf("username/nickname must not contain ':'"))
	}
	tid := tenant.TenantIDFromContext(ctx)
	if _, err := s.repo.GetByMobile(ctx, tid, mobile); err == nil {
		return nil, errs.New(errs.CodeMobileTaken, nil)
	}
	// 账号额度闸门（system 建用户同款检查），触额原样返回 error、不落库；limit 未装配时跳过。
	if s.limit != nil {
		if err := s.limit.CheckAccountLimit(ctx, tid); err != nil {
			return nil, err
		}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	if nickname == "" {
		// 无昵称时用手机号尾 4 位生成占位昵称。
		nickname = "用户" + mobile[len(mobile)-4:]
	}
	now := time.Now()
	u := &model.MemberUser{
		Nickname: nickname, Status: 1, Mobile: mobile,
		Password: string(hash), RegisterIP: ip, LoginDate: &now,
		TenantID: tid,
	}
	if err := s.repo.Create(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

// Authenticate member 账号凭据校验（members 表零改动；统一登录/兼容层共用）。
// 凭据校验通过后记录登录态：ip 非空才写 LoginIP/LoginDate；
// 写库失败不阻断签发（与旧 Login 不同——旧路径 Update 失败直接返回错误）。
func (s *memberService) Authenticate(ctx context.Context, mobile, password, ip string) (*oauth2svc.AuthIdentity, error) {
	tid := tenant.TenantIDFromContext(ctx)
	u, err := s.repo.GetByMobile(ctx, tid, mobile)
	if err != nil || u == nil {
		return nil, oauth2svc.ErrInvalidCredentials
	}
	if u.Status != 1 {
		return nil, oauth2svc.ErrUserDisabled
	}
	if bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)) != nil {
		return nil, oauth2svc.ErrInvalidCredentials
	}
	if ip != "" {
		now := time.Now()
		u.LoginIP, u.LoginDate = ip, &now
		_ = s.repo.Update(ctx, u)
	}
	return &oauth2svc.AuthIdentity{
		UserID:      u.ID,
		TenantID:    tid,
		Username:    u.Mobile,
		AccountType: oauth2svc.AccountTypeMember,
	}, nil
}

// Login 状态/密码校验通过后才签发（失败路径绝不触碰 issuer）。
// 凭据前三步（查用户→比密码→写登录 IP/日期）委托 Authenticate，本方法只负责签发。
func (s *memberService) Login(ctx context.Context, mobile, password, ip string) (*LoginResult, error) {
	ident, err := s.Authenticate(ctx, mobile, password, ip)
	if err != nil {
		return nil, err
	}
	if s.issuer == nil {
		// 未装配 issuer（micro 未接 gRPC / mono 漏注入）：与签发故障同语义 → 503。
		return nil, fmt.Errorf("%w: issuer not wired", ErrIssuance)
	}
	issued, err := s.issuer.Issue(ctx, token.IssueParams{
		UserID: ident.UserID, UserType: token.UserTypeMember, TenantID: ident.TenantID, ClientID: "alexgo-app",
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrIssuance, err)
	}
	return toResult(issued), nil
}

// Refresh 轮换刷新令牌（旧 refresh 单次使用，由 token.Issuer 保证）。
func (s *memberService) Refresh(ctx context.Context, refreshToken string) (*LoginResult, error) {
	issued, err := s.issuer.Refresh(ctx, refreshToken)
	if err != nil {
		return nil, err
	}
	return toResult(issued), nil
}

// Logout 吊销当前 access token。
func (s *memberService) Logout(ctx context.Context, accessToken string) error {
	return s.issuer.Revoke(ctx, accessToken)
}

// List 分页列出当前租户下的会员。
func (s *memberService) List(ctx context.Context, page, size int) ([]*model.MemberUser, int64, error) {
	return s.repo.List(ctx, tenant.TenantIDFromContext(ctx), page, size)
}

// Disable 启用/停用会员（status: 1启用 0停用）；停用不吊销既有 token——
// 中间件 Validate 查用户存在性，但状态检查在 login 层——既有 token 生命周期内仍有效
// （一期取舍：踢人场景由 RevokeAll 覆盖，本接口后续接 RevokeAll 联动）。
// status 边界校验（T5 deferred）：仅接受 0/1，其余值返回 ErrInvalidStatus → controller 400；
// "空 body 默认停用"在 controller 侧用指针 bind 关闭（字段缺失即 400，service 层不可见存在性）。
func (s *memberService) Disable(ctx context.Context, id uint64, status int) error {
	if status != 0 && status != 1 {
		return fmt.Errorf("%w: %d (want 0 or 1)", ErrInvalidStatus, status)
	}
	// 仓储无按 ID 直取——用 UpdateStatus 语义按 tenant+id 局部更新。
	return s.repo.UpdateStatus(ctx, tenant.TenantIDFromContext(ctx), id, status)
}

// Export 租户内全量会员：租户 ID 取自 ctx（tenant 插件经 ctx 照常隔离），经
// repo.ListAll 不截断返回——controller 组 excel 列映射。
func (s *memberService) Export(ctx context.Context) ([]*model.MemberUser, error) {
	return s.repo.ListAll(ctx, tenant.TenantIDFromContext(ctx))
}
