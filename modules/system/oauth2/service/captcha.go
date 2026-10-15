package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/mojocn/base64Captcha"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/fx"

	"alexGo-cloud/pkg/config"
)

// ErrCaptchaInvalid 验证码缺失/错误/过期/已消费（Task 8/10 映射 400 captcha_invalid）。
var ErrCaptchaInvalid = errors.New("captcha invalid")

// CaptchaChallenge 一次图形验证码（Image 含 data URI 前缀，前端直接作 img src）。
type CaptchaChallenge struct {
	UUID    string `json:"uuid"`
	Image   string `json:"image"`
	Enabled bool   `json:"enabled"`
}

// CaptchaDeps Redis optional：member 进程无 redis 也能装配（D6）。
type CaptchaDeps struct {
	fx.In
	Cfg   *config.Config
	Redis *goredis.Client `optional:"true"`
}

// CaptchaService 图形验证码：开关控制登录时是否校验，图像本身始终可取（spec §3.4）。
type CaptchaService struct {
	enabled bool
	driver  base64Captcha.Driver
	store   base64Captcha.Store
}

// NewCaptchaService redis 可用则挑战值存 redis（key captcha:{uuid} TTL 2 分钟），
// 否则内存 store（D6）。
func NewCaptchaService(deps CaptchaDeps) *CaptchaService {
	enabled := deps.Cfg != nil && deps.Cfg.Auth.Captcha.Enabled
	driver := base64Captcha.NewDriverDigit(80, 240, 5, 0.7, 80)
	var store base64Captcha.Store
	if deps.Redis != nil {
		store = newRedisStore(deps.Redis, 2*time.Minute)
	} else {
		store = base64Captcha.NewMemoryStore(1024, 2*time.Minute)
	}
	return &CaptchaService{enabled: enabled, driver: driver, store: store}
}

// Enabled 当前开关状态（false 时登录跳过校验）。
func (s *CaptchaService) Enabled() bool { return s.enabled }

// NewChallenge 无论开关都生成图像（开关只影响登录时是否校验，spec §3.4）。
func (s *CaptchaService) NewChallenge(ctx context.Context) (*CaptchaChallenge, error) {
	c := base64Captcha.NewCaptcha(s.driver, s.store)
	id, b64s, _, err := c.Generate()
	if err != nil {
		return nil, fmt.Errorf("captcha generate: %w", err)
	}
	return &CaptchaChallenge{
		UUID:    id,
		Image:   "data:image/png;base64," + b64s,
		Enabled: s.enabled,
	}, nil
}

// Verify disabled 恒过；错/未知/已消费 → ErrCaptchaInvalid（clear=true 一次性）。
func (s *CaptchaService) Verify(ctx context.Context, uuid, code string) error {
	if !s.enabled {
		return nil
	}
	if uuid == "" || code == "" {
		return ErrCaptchaInvalid
	}
	c := base64Captcha.NewCaptcha(s.driver, s.store)
	if !c.Verify(uuid, code, true) {
		return ErrCaptchaInvalid
	}
	return nil
}

// redisStore 适配 base64Captcha.Store，值即答案字符串。
type redisStore struct {
	rdb *goredis.Client
	ttl time.Duration
	mu  sync.Mutex
}

func newRedisStore(rdb *goredis.Client, ttl time.Duration) base64Captcha.Store {
	return &redisStore{rdb: rdb, ttl: ttl}
}

func (s *redisStore) Set(id, value string) error {
	ctx := context.Background()
	return s.rdb.Set(ctx, "captcha:"+id, value, s.ttl).Err()
}

func (s *redisStore) Get(id string, clear bool) string {
	ctx := context.Background()
	key := "captcha:" + id
	v, err := s.rdb.Get(ctx, key).Result()
	if err != nil {
		return ""
	}
	if clear {
		s.rdb.Del(ctx, key)
	}
	return v
}

func (s *redisStore) Verify(id, answer string, clear bool) bool {
	v := s.Get(id, clear)
	return v != "" && v == answer
}
