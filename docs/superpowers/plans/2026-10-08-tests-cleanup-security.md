# alexGo-cloud 测试补齐 / 项目清理 / 安全加固 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 alexGo-cloud 补齐核心逻辑单元测试（当前为 0），修复环境变量配置失效等安全问题，完成 frontend/ 冗余清理、health 探活、pprof 保护、文档补全等清理项。

**Architecture:** 不改变业务架构。测试全部为**无外部依赖的进程内测试**（miniredis 模拟 Redis、内存 Casbin、sqlite 内存库）；安全修复集中在 `pkg/config` 环境变量覆盖链路与 pprof/secret 配置门控；清理项为删冗余、修 Makefile、补文档。

**Tech Stack:** Go 1.25、testing 标准库、miniredis、casbin、gorm + glebarez/sqlite（测试用）、gin httptest。

**Spec:** 无独立 spec 文件。需求来源为 2026-10-08 会话中用户确认的三项范围：
1. 补齐核心测试（限流、熔断、Outbox、RBAC/JWT 鉴权）
2. 清理项目问题（删 frontend/、修 generate-all、补 docs、health 探活、pprof 保护）
3. 安全问题（config.yaml 明文密码、DB_DSN/JWT_SECRET 环境变量失效、K8s ConfigMap 明文密钥、pprof 无鉴权暴露）

## Global Constraints

- 项目路径：`/Users/alex/Desktop/goWork/alexGo-cloud`（模块名 `alexGo-cloud`，Go 1.25）
- **测试必须无外部依赖**：不得要求 MySQL/Redis/NATS/Docker 运行（GitHub CI `ci.yml` 无任何 service 容器）
- 项目当前**不是 git 仓库**（但已有 .github/workflows 与 .gitignore）；Task 1 先建立 git 基线，之后每任务一次 commit
- 本地未安装 golangci-lint（CI 中有）；本地验证命令统一为：`go build ./... && go vet ./... && go test ./...`
- 注释风格与现有一致：中文、解释"为什么"，文件级 doc comment 保留原设计说明
- 环境变量命名沿用现有约定：`DB_DSN`、`JWT_SECRET`（新）、`HTTP_ADDR`、`GRPC_ADDR`
- 修改生产代码前先写失败测试（TDD）；无生产代码改动的任务（纯测试/纯文档）直接写并验证

## Review Focus

以下 5 类输入/故障最可能出错，各有归属任务的测试钉住：

1. **Token Bucket Lua 语义**：burst 内放行、桶耗尽拒绝、Redis 宕机/未配置时 fail-open 放行 → Task 5 测试
2. **熔断状态机**：连续失败达阈值开闸、Open 期拒绝、到期 HalfOpen 探测、成功复位 Closed、nil 接收者安全 → Task 4 测试
3. **JWT 伪造与兼容**：错误 secret / 过期 / `alg=none` 伪造 token 必须拒绝；`Bearer ` 前缀与裸 token 均可解析 → Task 3 测试
4. **鉴权中间件门控**：非 `/api/admin/` 放行、admin 无/坏 token → 401、Casbin 无策略 → 403、放行路径注入 claims → 200 → Task 6 测试
5. **环境变量覆盖优先级**：`DB_DSN`/`JWT_SECRET` 必须压过 config.yaml 与模块配置（当前代码 `DB_DSN` 是死变量，Compose/K8s 注入无效） → Task 2 测试

---

### Task 1: 仓库基线（git init + .gitignore 补全）

**Files:**
- Modify: `.gitignore`

**Interfaces:**
- Consumes: 无
- Produces: 一个可提交的 git 仓库，后续所有任务在其中 commit

- [ ] **Step 1: 补全 .gitignore**

在 `.gitignore` 末尾追加（当前缺少 node_modules 与本地工具目录）：

```
node_modules/
.ccto/
coverage.out
```

- [ ] **Step 2: 初始化仓库并提交基线**

```bash
cd /Users/alex/Desktop/goWork/alexGo-cloud
git init
git add -A
git status   # 确认没有 node_modules/ 或大文件被纳入
git commit -m "chore: baseline import of alexGo-cloud"
```

- [ ] **Step 3: 验证**

Run: `git log --oneline && git status`
Expected: 一条 baseline commit，工作区干净

---

### Task 2: 安全修复——DB_DSN / JWT_SECRET 环境变量真正生效（TDD）

**背景（为什么是 bug）：** `deployments/docker-compose/docker-compose.yml` 和 `deployments/kubernetes/configmap.yaml` 都注入了 `DB_DSN` 环境变量，`pkg/config/loader.go` 的注释也声称支持，但代码里**从未 BindEnv 该变量**；且模块配置合并用 `v.Set()`（viper 中 override 优先级**高于** env），即使 BindEnv 也无法覆盖 `system.jwt_secret`。结论：容器里的 `DB_DSN` 目前是死变量，密钥只能靠 ConfigMap 明文 config.yaml 传递。修复方式：Unmarshal 之后做显式 env 覆盖，保证 Secret 注入生效。

**Files:**
- Create: `pkg/config/loader_test.go`
- Modify: `pkg/config/loader.go`（`LoadGlobalConfig` 返回前）

**Interfaces:**
- Consumes: 现有 `config.LoadGlobalConfig() (*Config, error)`
- Produces: `applyEnvOverrides(cfg *Config)`（包内私有函数）；环境变量 `DB_DSN`→`Database.DSN`、`JWT_SECRET`→`System.JWTSecret`、`REDIS_PASSWORD`→`Redis.Password` 生效

- [ ] **Step 1: 写失败测试**

创建 `pkg/config/loader_test.go`：

```go
package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"alexGo-cloud/pkg/config"
)

// chdirRepoRoot 切到仓库根目录：LoadGlobalConfig 使用固定相对路径
// alexgo-server/configs/config.yaml（测试运行于 pkg/config 目录）。
func chdirRepoRoot(t *testing.T) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Clean(filepath.Join(wd, "..", ".."))
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
}

// 环境变量必须压过 config.yaml / 模块配置（K8s Secret 注入的前提）。
func TestLoadGlobalConfig_EnvOverridesBeatFile(t *testing.T) {
	chdirRepoRoot(t)
	t.Setenv("DB_DSN", "envuser:envpass@tcp(127.0.0.1:3306)/envdb?charset=utf8mb4&parseTime=True&loc=Local")
	t.Setenv("JWT_SECRET", "env-jwt-secret")
	t.Setenv("REDIS_PASSWORD", "env-redis-pass")

	cfg, err := config.LoadGlobalConfig()
	if err != nil {
		t.Fatalf("LoadGlobalConfig() error = %v", err)
	}
	if want := "envuser:envpass@tcp(127.0.0.1:3306)/envdb?charset=utf8mb4&parseTime=True&loc=Local"; cfg.Database.DSN != want {
		t.Errorf("DSN = %q, want env override %q", cfg.Database.DSN, want)
	}
	if cfg.System.JWTSecret != "env-jwt-secret" {
		t.Errorf("JWTSecret = %q, want %q", cfg.System.JWTSecret, "env-jwt-secret")
	}
	if cfg.Redis.Password != "env-redis-pass" {
		t.Errorf("Redis.Password = %q, want %q", cfg.Redis.Password, "env-redis-pass")
	}
}

// env 为空时回退到 config 文件（本地开发体验不变）。
func TestLoadGlobalConfig_FallbackToFileWhenEnvEmpty(t *testing.T) {
	chdirRepoRoot(t)
	t.Setenv("DB_DSN", "")
	t.Setenv("JWT_SECRET", "")
	t.Setenv("REDIS_PASSWORD", "")

	cfg, err := config.LoadGlobalConfig()
	if err != nil {
		t.Fatalf("LoadGlobalConfig() error = %v", err)
	}
	if cfg.Database.DSN == "" {
		t.Error("DSN empty: config.yaml fallback broken")
	}
	if cfg.System.JWTSecret == "" {
		t.Error("JWTSecret empty: module config fallback broken")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./pkg/config/ -run TestLoadGlobalConfig -v`
Expected: `TestLoadGlobalConfig_EnvOverridesBeatFile` FAIL（DSN 仍是 config.yaml 中的 `root:root...`）

- [ ] **Step 3: 实现 env 覆盖**

修改 `pkg/config/loader.go`：在 `LoadGlobalConfig` 的 `v.Unmarshal(&cfg)` 成功后、`return &cfg, nil` 前调用 `applyEnvOverrides(&cfg)`，并在文件底部新增：

```go
// applyEnvOverrides 在 Unmarshal 之后用显式环境变量覆盖安全敏感配置。
//
// 为什么不用 viper.AutomaticEnv / BindEnv：
// 1) AutomaticEnv 的隐式映射不会参与 Unmarshal（viper 已知行为）；
// 2) 模块配置合并使用 v.Set()，其优先级高于 env，会导致 env 永远输给模块 yaml。
// 因此对 K8s Secret / Compose 注入的密钥变量，在这里做最终覆盖（仅非空时生效）。
func applyEnvOverrides(cfg *Config) {
	pairs := []struct {
		env string
		set func(v string)
	}{
		{"DB_DSN", func(v string) { cfg.Database.DSN = v }},
		{"JWT_SECRET", func(v string) { cfg.System.JWTSecret = v }},
		{"REDIS_PASSWORD", func(v string) { cfg.Redis.Password = v }},
	}
	for _, p := range pairs {
		if v := os.Getenv(p.env); v != "" {
			p.set(v)
		}
	}
}
```

同时把 `LoadGlobalConfig` 顶部 doc comment 中"额外 BindEnv：DB_DSN ..."的不实描述改为：

```
// 环境变量覆盖：
// - viper.AutomaticEnv() + BindEnv：HTTP_ADDR / GRPC_ADDR / NATS_URL / REDIS_ADDR
// - applyEnvOverrides（Unmarshal 之后）：DB_DSN / JWT_SECRET / REDIS_PASSWORD（密钥类，保证压过模块 yaml）
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./pkg/config/ -run TestLoadGlobalConfig -v`
Expected: 两个测试 PASS

- [ ] **Step 5: 全量验证**

Run: `go build ./... && go vet ./... && go test ./pkg/config/ -v`
Expected: 全部通过

- [ ] **Step 6: Commit**

```bash
git add pkg/config/
git commit -m "fix(config): make DB_DSN/JWT_SECRET/REDIS_PASSWORD env overrides actually take effect"
```

---

### Task 3: JWT 单元测试

**Files:**
- Create: `pkg/auth/jwt_test.go`

**Interfaces:**
- Consumes: `auth.GenerateToken(userID uint64, username string, tenantID uint64, cfg *config.Config) (string, error)`、`auth.ParseToken(tokenStr string, cfg *config.Config) (*Claims, error)`、`auth.Claims{UserID, Username, TenantID}`
- Produces: 无（纯测试任务）

- [ ] **Step 1: 编写测试**

创建 `pkg/auth/jwt_test.go`：

```go
package auth_test

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"alexGo-cloud/pkg/auth"
	"alexGo-cloud/pkg/config"
)

func cfgWithSecret(secret string) *config.Config {
	c := &config.Config{}
	c.System.JWTSecret = secret
	return c
}

func TestGenerateParse_Roundtrip(t *testing.T) {
	cfg := cfgWithSecret("test-secret")
	token, err := auth.GenerateToken(42, "alice", 7, cfg)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	// 兼容 "Bearer <token>" 与裸 token 两种输入。
	for _, in := range []string{token, "Bearer " + token} {
		claims, err := auth.ParseToken(in, cfg)
		if err != nil {
			t.Fatalf("ParseToken(%q) error = %v", in, err)
		}
		if claims.UserID != 42 || claims.Username != "alice" || claims.TenantID != 7 {
			t.Errorf("claims = %+v, want {42 alice 7}", claims)
		}
	}
}

func TestGenerateToken_EmptySecret(t *testing.T) {
	if _, err := auth.GenerateToken(1, "u", 1, cfgWithSecret("")); err == nil {
		t.Error("GenerateToken with empty secret should fail")
	}
}

func TestParseToken_EmptyInputs(t *testing.T) {
	cfg := cfgWithSecret("test-secret")
	for _, in := range []string{"", "Bearer ", "   "} {
		if _, err := auth.ParseToken(in, cfg); err == nil {
			t.Errorf("ParseToken(%q) should fail", in)
		}
	}
}

func TestParseToken_WrongSecret(t *testing.T) {
	token, err := auth.GenerateToken(1, "alice", 1, cfgWithSecret("secret-a"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := auth.ParseToken(token, cfgWithSecret("secret-b")); err == nil {
		t.Error("token signed with another secret must be rejected")
	}
}

// 过期 token 必须被拒（伪造一个 exp 已过期的合法签名 token）。
func TestParseToken_Expired(t *testing.T) {
	cfg := cfgWithSecret("test-secret")
	type expClaims struct {
		auth.Claims
		jwt.RegisteredClaims
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, expClaims{
		Claims: auth.Claims{UserID: 1, Username: "alice"},
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
		},
	}).SignedString([]byte(cfg.System.JWTSecret))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := auth.ParseToken(signed, cfg); err == nil {
		t.Error("expired token must be rejected")
	}
}

// alg=none 伪造 token 必须被拒（签名算法必须是 HMAC）。
func TestParseToken_AlgNoneRejected(t *testing.T) {
	cfg := cfgWithSecret("test-secret")
	type noneClaims struct {
		auth.Claims
		jwt.RegisteredClaims
	}
	forged, err := jwt.NewWithClaims(jwt.SigningMethodNone, noneClaims{
		Claims: auth.Claims{UserID: 1, Username: "admin"},
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := auth.ParseToken(forged, cfg); err == nil {
		t.Error("alg=none token must be rejected")
	}
}

func TestParseToken_EmptySecretConfig(t *testing.T) {
	if _, err := auth.ParseToken("whatever", cfgWithSecret("")); err == nil {
		t.Error("empty secret config should fail")
	}
}
```

（import 中不要包含 `strings`——上面代码已不含用到它的语句。）

- [ ] **Step 2: 运行测试**

Run: `go test ./pkg/auth/ -run TestGenerate -v && go test ./pkg/auth/ -run TestParseToken -v`
Expected: 全部 PASS（若出现非预期失败，说明发现了真实 bug——先记录失败输出，再决定修 `jwt.go` 还是修测试预期，修法需与人确认）

- [ ] **Step 3: 全量验证**

Run: `go build ./... && go vet ./... && go test ./pkg/auth/ -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add pkg/auth/jwt_test.go
git commit -m "test(auth): cover JWT roundtrip, wrong-secret, expired and alg=none rejection"
```

---

### Task 4: 熔断器单元测试

**Files:**
- Create: `pkg/circuitbreaker/breaker_test.go`

**Interfaces:**
- Consumes: `NewCircuitBreaker(threshold int, openFor time.Duration) *CircuitBreaker`、`(*CircuitBreaker).Do(fn func() error) error`、`ErrOpen`、`State{Closed, Open, HalfOpen}`
- Produces: 无（纯测试任务）

- [ ] **Step 1: 编写测试**

创建 `pkg/circuitbreaker/breaker_test.go`：

```go
package circuitbreaker

import (
	"errors"
	"testing"
	"time"
)

func fail() error { return errors.New("down") }

// 连续失败达 threshold → Open，后续请求快速失败且不再执行 fn。
func TestDo_OpensAfterThreshold(t *testing.T) {
	cb := NewCircuitBreaker(3, time.Minute)
	for i := 0; i < 3; i++ {
		if err := cb.Do(fail); err == nil {
			t.Fatalf("round %d: want error from fn", i)
		}
	}
	if cb.state != Open {
		t.Fatalf("state = %v, want Open", cb.state)
	}

	executed := false
	err := cb.Do(func() error { executed = true; return nil })
	if !errors.Is(err, ErrOpen) {
		t.Errorf("err = %v, want ErrOpen", err)
	}
	if executed {
		t.Error("fn must not execute while breaker is Open")
	}
}

// 成功调用会把失败计数清零（不会累积到阈值）。
func TestDo_SuccessResetsFailures(t *testing.T) {
	cb := NewCircuitBreaker(3, time.Minute)
	_ = cb.Do(fail)
	_ = cb.Do(fail)
	if err := cb.Do(func() error { return nil }); err != nil {
		t.Fatalf("success Do() error = %v", err)
	}
	if cb.failures != 0 {
		t.Errorf("failures = %d, want 0 after success", cb.failures)
	}
	// 再失败 2 次仍不应打开（第 3 次成功已重置）。
	_ = cb.Do(fail)
	_ = cb.Do(fail)
	if cb.state != Closed {
		t.Errorf("state = %v, want Closed", cb.state)
	}
}

// Open 期拒绝 → openFor 到期进入 HalfOpen 放行探测 → 探测成功回到 Closed。
func TestDo_OpenThenHalfOpenRecovers(t *testing.T) {
	cb := NewCircuitBreaker(1, 50*time.Millisecond)
	if err := cb.Do(fail); err == nil {
		t.Fatal("setup: fail() should return error")
	}
	if cb.state != Open {
		t.Fatalf("setup: state = %v, want Open", cb.state)
	}

	// 未到期：拒绝。
	if err := cb.Do(func() error { return nil }); !errors.Is(err, ErrOpen) {
		t.Fatalf("before openFor elapsed: err = %v, want ErrOpen", err)
	}

	time.Sleep(60 * time.Millisecond)

	// 到期：放行探测，成功 → Closed。
	if err := cb.Do(func() error { return nil }); err != nil {
		t.Errorf("half-open probe err = %v, want nil", err)
	}
	if cb.state != Closed {
		t.Errorf("state = %v, want Closed after successful probe", cb.state)
	}
}

// HalfOpen 探测失败 → 立即回到 Open。
func TestDo_HalfOpenFailureReopens(t *testing.T) {
	cb := NewCircuitBreaker(1, 50*time.Millisecond)
	_ = cb.Do(fail)
	time.Sleep(60 * time.Millisecond)
	if err := cb.Do(fail); err == nil {
		t.Fatal("probe should return fn error")
	}
	if cb.state != Open {
		t.Errorf("state = %v, want Open after failed probe", cb.state)
	}
}

// nil 接收者直接执行（与 Allow 一样的 fail-open 约定）。
func TestDo_NilReceiver(t *testing.T) {
	var cb *CircuitBreaker
	executed := false
	if err := cb.Do(func() error { executed = true; return nil }); err != nil {
		t.Fatalf("nil cb.Do() error = %v", err)
	}
	if !executed {
		t.Error("fn must execute on nil receiver")
	}
}

// 非法参数回退默认值，不 panic。
func TestNewCircuitBreaker_Defaults(t *testing.T) {
	cb := NewCircuitBreaker(0, 0)
	if cb.threshold != 10 || cb.openFor != 30*time.Second {
		t.Errorf("defaults = (%d, %v), want (10, 30s)", cb.threshold, cb.openFor)
	}
}
```

- [ ] **Step 2: 运行测试**

Run: `go test ./pkg/circuitbreaker/ -v -count=1`
Expected: 全部 PASS。若有失败（如 HalfOpen 时序），优先怀疑测试时序假设而非状态机——`-count=3` 重跑排除 flaky 后再分析代码

- [ ] **Step 3: 竞态检测**

Run: `go test ./pkg/circuitbreaker/ -race -count=1`
Expected: PASS（无 data race）

- [ ] **Step 4: Commit**

```bash
git add pkg/circuitbreaker/breaker_test.go
git commit -m "test(circuitbreaker): cover open/half-open/recover state machine and nil receiver"
```

---

### Task 5: 限流器单元测试（miniredis）

**Files:**
- Create: `pkg/limiter/rate_limiter_test.go`
- Modify: `go.mod`（新增测试依赖）

**Interfaces:**
- Consumes: `limiter.NewRateLimiter(rdb *redis.Client) *RateLimiter`、`(*RateLimiter).Allow(ctx, key string, rate, burst int) bool`
- Produces: 依赖 `github.com/alicebob/miniredis/v2`（进程内 Redis 模拟，支持 Lua）

- [ ] **Step 1: 添加测试依赖**

```bash
go get github.com/alicebob/miniredis/v2@latest
go mod tidy
```

- [ ] **Step 2: 编写测试**

创建 `pkg/limiter/rate_limiter_test.go`：

```go
package limiter_test

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"alexGo-cloud/pkg/limiter"
)

func newTestLimiter(t *testing.T) (*limiter.RateLimiter, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return limiter.NewRateLimiter(rdb), mr
}

// burst 内放行、桶耗尽拒绝（Token Bucket 核心语义）。
func TestAllow_TokenBucketExhaustion(t *testing.T) {
	l, _ := newTestLimiter(t)
	ctx := context.Background()

	if !l.Allow(ctx, "k1", 1, 2) {
		t.Fatal("1st request should pass (bucket starts full)")
	}
	if !l.Allow(ctx, "k1", 1, 2) {
		t.Fatal("2nd request should pass (burst=2)")
	}
	if l.Allow(ctx, "k1", 1, 2) {
		t.Error("3rd request should be limited (bucket empty)")
	}
}

// 不同 key 互不影响（限流维度隔离）。
func TestAllow_KeysAreIndependent(t *testing.T) {
	l, _ := newTestLimiter(t)
	ctx := context.Background()

	_ = l.Allow(ctx, "a", 1, 1)
	_ = l.Allow(ctx, "a", 1, 1) // a 已耗尽
	if !l.Allow(ctx, "b", 1, 1) {
		t.Error("key b should have its own bucket")
	}
}

// fail-open：未配置 Redis / Redis 宕机 / 非法参数时一律放行。
func TestAllow_FailOpen(t *testing.T) {
	ctx := context.Background()

	var nilLimiter *limiter.RateLimiter
	if !nilLimiter.Allow(ctx, "k", 10, 10) {
		t.Error("nil limiter must allow (fail-open)")
	}

	if !limiter.NewRateLimiter(nil).Allow(ctx, "k", 10, 10) {
		t.Error("nil redis must allow (fail-open)")
	}

	l, mr := newTestLimiter(t)
	if !l.Allow(ctx, "k", 0, 0) {
		t.Error("rate/burst <= 0 must allow")
	}

	mr.Close() // 模拟 Redis 宕机
	if !l.Allow(ctx, "k", 10, 10) {
		t.Error("redis outage must allow (fail-open)")
	}
}
```

- [ ] **Step 3: 运行测试**

Run: `go test ./pkg/limiter/ -v -count=1`
Expected: 全部 PASS。若 Lua 脚本执行报错（miniredis 对 `EVALSHA` 兼容问题），go-redis 的 `Script.Run` 会自动回退 `EVAL`，不应失败；若仍失败，把 `script.Run` 改为 `script.Eval`（在生产代码中改动需单独说明）

- [ ] **Step 4: 全量验证**

Run: `go mod tidy && go build ./... && go vet ./... && go test ./pkg/limiter/ -count=1`
Expected: PASS，`go.mod` 中 miniredis 出现在 require 直接依赖

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum pkg/limiter/rate_limiter_test.go
git commit -m "test(limiter): cover token-bucket exhaustion, key isolation and fail-open paths"
```

---

### Task 6: 鉴权中间件单元测试

**Files:**
- Create: `pkg/middleware/auth_test.go`

**Interfaces:**
- Consumes: `middleware.AuthMiddleware(cfg *config.Config, enforcer *casbin.Enforcer) gin.HandlerFunc`；`auth.GenerateToken`
- Produces: 无（纯测试任务）

**说明：** 中间件逻辑是"仅 `/api/admin/**` 需要 JWT；enforcer 非 nil 时再做 Casbin 校验"。Casbin 使用与 `pkg/auth/casbin.go` 相同的 model 字符串（内存 adapter，策略测试内 AddPolicy）。

- [ ] **Step 1: 编写测试**

创建 `pkg/middleware/auth_test.go`：

```go
package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/gin-gonic/gin"

	"alexGo-cloud/pkg/auth"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/middleware"
)

func testCfg() *config.Config {
	c := &config.Config{}
	c.System.JWTSecret = "middleware-test-secret"
	return c
}

// newEnforcer 构建与 pkg/auth/casbin.go 同款 model 的内存 Casbin。
func newEnforcer(t *testing.T) *casbin.Enforcer {
	t.Helper()
	m, err := model.NewModelFromString(`
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && keyMatch2(r.obj, p.obj) && regexMatch(r.act, p.act)
`)
	if err != nil {
		t.Fatal(err)
	}
	e, err := casbin.NewEnforcer(m)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func newRouter(cfg *config.Config, enf *casbin.Enforcer) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.AuthMiddleware(cfg, enf))
	r.GET("/api/admin/system/users", func(c *gin.Context) {
		claimsAny, _ := c.Get("claims")
		claims, _ := claimsAny.(*auth.Claims)
		name := ""
		if claims != nil {
			name = claims.Username
		}
		c.JSON(http.StatusOK, gin.H{"username": name})
	})
	r.POST("/api/app/system/auth/login", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	return r
}

func do(r *gin.Engine, method, path, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set("Authorization", token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestAuthMiddleware_AdminWithoutToken_401(t *testing.T) {
	w := do(newRouter(testCfg(), nil), "GET", "/api/admin/system/users", "")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestAuthMiddleware_AdminBadToken_401(t *testing.T) {
	w := do(newRouter(testCfg(), nil), "GET", "/api/admin/system/users", "Bearer not-a-jwt")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestAuthMiddleware_AdminValidToken_200InjectsClaims(t *testing.T) {
	cfg := testCfg()
	token, err := auth.GenerateToken(9, "alice", 1, cfg)
	if err != nil {
		t.Fatal(err)
	}
	w := do(newRouter(cfg, nil), "GET", "/api/admin/system/users", "Bearer "+token)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if body := w.Body.String(); body != `{"username":"alice"}` {
		t.Errorf("body = %s, want claims injected into context", body)
	}
}

func TestAuthMiddleware_NonAdminPaths_WithoutToken_200(t *testing.T) {
	r := newRouter(testCfg(), nil)
	for _, tc := range []struct{ method, path string }{
		{"POST", "/api/app/system/auth/login"},
		{"GET", "/health"},
	} {
		if w := do(r, tc.method, tc.path, ""); w.Code != http.StatusOK {
			t.Errorf("%s %s: status = %d, want 200 (public path)", tc.method, tc.path, w.Code)
		}
	}
}

func TestAuthMiddleware_CasbinDeny_403(t *testing.T) {
	cfg := testCfg()
	enf := newEnforcer(t)
	// 只给 bob 授权；alice 请求 → 403。
	enf.AddPolicy("bob", "/api/admin/system/users", "GET")

	token, err := auth.GenerateToken(1, "alice", 1, cfg)
	if err != nil {
		t.Fatal(err)
	}
	w := do(newRouter(cfg, enf), "GET", "/api/admin/system/users", "Bearer "+token)
	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", w.Code)
	}
}

func TestAuthMiddleware_CasbinAllow_200(t *testing.T) {
	cfg := testCfg()
	enf := newEnforcer(t)
	enf.AddPolicy("alice", "/api/admin/system/users", "GET")

	token, err := auth.GenerateToken(1, "alice", 1, cfg)
	if err != nil {
		t.Fatal(err)
	}
	w := do(newRouter(cfg, enf), "GET", "/api/admin/system/users", "Bearer "+token)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}
```

- [ ] **Step 2: 运行测试**

Run: `go test ./pkg/middleware/ -run TestAuthMiddleware -v -count=1`
Expected: 全部 PASS

- [ ] **Step 3: Commit**

```bash
git add pkg/middleware/auth_test.go
git commit -m "test(middleware): cover admin JWT gating, public paths and casbin allow/deny"
```

---

### Task 7: Outbox 发布逻辑提取 + 单测

**背景：** `Relay.processPending` 抓取与发布耦合在 GORM 事务里，且 SQL 含 `FOR UPDATE SKIP LOCKED`（sqlite 不支持），无法进程内测试。做**最小重构**：把"逐条发布 + 状态回调"抽成纯函数，事务/SQL 部分保持原样；对抽出的函数注入 fake Broker 测试。

**Files:**
- Modify: `pkg/outbox/relay.go`（新增 `publishEvents`，`processPending` 改为调用它）
- Create: `pkg/outbox/relay_test.go`

**Interfaces:**
- Consumes: `mq.Broker`（`Publish(ctx, subject string, payload []byte) error`）、`systemmodel.OutboxEvent{ID, EventType, Payload json.RawMessage, Status, PublishedAt}`
- Produces: `publishEvents(ctx context.Context, broker mq.Broker, events []systemmodel.OutboxEvent, mark func(id uint64, status string, publishedAt *time.Time) error) error`（包内私有）

- [ ] **Step 1: 写失败测试（针对尚不存在的 publishEvents）**

创建 `pkg/outbox/relay_test.go`：

```go
package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	systemmodel "alexGo-cloud/modules/system/model"
	"alexGo-cloud/pkg/mq"
)

type fakeBroker struct {
	err     error
	publish []string // 记录收到的 subject
}

func (f *fakeBroker) Publish(_ context.Context, subject string, _ []byte) error {
	if f.err != nil {
		return f.err
	}
	f.publish = append(f.publish, subject)
	return nil
}

func events(ids ...uint64) []systemmodel.OutboxEvent {
	var out []systemmodel.OutboxEvent
	for _, id := range ids {
		out = append(out, systemmodel.OutboxEvent{
			ID:        id,
			EventType: "order.created",
			Payload:   json.RawMessage(`{"id":1}`),
			Status:    "pending",
		})
	}
	return out
}

// 全部发布成功 → 每条回调 published。
func TestPublishEvents_AllSuccess(t *testing.T) {
	b := &fakeBroker{}
	type marked struct {
		id     uint64
		status string
	}
	var got []marked
	err := publishEvents(context.Background(), b, events(1, 2, 3),
		func(id uint64, status string, _ *time.Time) error {
			got = append(got, marked{id, status})
			return nil
		})
	if err != nil {
		t.Fatalf("publishEvents() error = %v", err)
	}
	if len(b.publish) != 3 {
		t.Errorf("published %d, want 3", len(b.publish))
	}
	if len(got) != 3 {
		t.Fatalf("mark called %d times, want 3", len(got))
	}
	for i, g := range got {
		if g.status != "published" {
			t.Errorf("mark[%d] status = %q, want published", i, g.status)
		}
	}
}

// Broker 发布失败 → 对应事件回调 failed，且不影响后续事件。
func TestPublishEvents_PublishFailureMarksFailed(t *testing.T) {
	b := &fakeBroker{err: errors.New("nats down")}
	statuses := map[uint64]string{}
	err := publishEvents(context.Background(), b, events(1, 2),
		func(id uint64, status string, _ *time.Time) error {
			statuses[id] = status
			return nil
		})
	if err != nil {
		t.Fatalf("publishEvents() error = %v", err)
	}
	for _, id := range []uint64{1, 2} {
		if statuses[id] != "failed" {
			t.Errorf("event %d status = %q, want failed", id, statuses[id])
		}
	}
}

// 发布成功但状态回写失败 → 返回错误（让调用方感知，事务回滚语义由 processPending 决定）。
func TestPublishEvents_MarkErrorPropagates(t *testing.T) {
	b := &fakeBroker{}
	err := publishEvents(context.Background(), b, events(1),
		func(uint64, string, *time.Time) error { return errors.New("db down") })
	if err == nil {
		t.Error("mark error must propagate")
	}
}

// nil 守卫：processPending 在 db/broker 为 nil 时必须 no-op。
func TestProcessPending_NilGuards(t *testing.T) {
	var nilRelay *Relay
	if err := nilRelay.processPending(context.Background(), 10); err != nil {
		t.Errorf("nil relay err = %v, want nil", err)
	}
	r := NewRelay(nil, nil, nil)
	if err := r.processPending(context.Background(), 10); err != nil {
		t.Errorf("nil db/broker err = %v, want nil", err)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./pkg/outbox/ -v -count=1`
Expected: 编译失败 `undefined: publishEvents`（`TestProcessPending_NilGuards` 本身可通过）

- [ ] **Step 3: 提取 publishEvents**

修改 `pkg/outbox/relay.go`：新增函数（放在 `processPending` 之前），并把 `processPending` 事务内的发布循环替换为调用它：

```go
// publishEvents 逐条发布事件并通过 mark 回调记录状态（published / failed）。
//
// 抽出为纯函数的目的：事务与 FOR UPDATE SKIP LOCKED 抓取依赖 MySQL，
// 无法在单测中执行；发布语义（成功标记、失败标记、错误传播）在这里独立验证。
// mark 返回错误时立即中止（processPending 中即为事务内 UPDATE 失败）。
func publishEvents(ctx context.Context, broker mq.Broker, events []systemmodel.OutboxEvent,
	mark func(id uint64, status string, publishedAt *time.Time) error) error {
	for _, e := range events {
		payload := []byte(e.Payload)
		if err := broker.Publish(ctx, e.EventType, payload); err != nil {
			if lerr := mark(e.ID, "failed", nil); lerr != nil {
				return lerr
			}
			if logger.Log != nil {
				logger.Log.Error("outbox publish failed", zap.Uint64("id", e.ID), zap.Error(err))
			}
			continue
		}
		now := time.Now()
		if merr := mark(e.ID, "published", &now); merr != nil {
			return merr
		}
	}
	return nil
}
```

`processPending` 的事务体改为（保持原有 SQL 与日志行为）：

```go
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var events []systemmodel.OutboxEvent
		if err := tx.Raw(
			"SELECT * FROM outbox_events WHERE status = ? ORDER BY id LIMIT ? FOR UPDATE SKIP LOCKED",
			"pending",
			limit,
		).Scan(&events).Error; err != nil {
			return err
		}

		return publishEvents(ctx, r.broker, events, func(id uint64, status string, publishedAt *time.Time) error {
			updates := map[string]any{"status": status}
			if publishedAt != nil {
				updates["published_at"] = publishedAt
			}
			return tx.Model(&systemmodel.OutboxEvent{}).Where("id = ?", id).Updates(updates).Error
		})
	})
```

（原实现中 published 更新失败只记日志不中断、failed 更新错误被忽略——提取后 mark 错误会中断事务，语义更严格且可测试；这是有意的最小行为变化，在 commit message 中说明。）

- [ ] **Step 4: 运行确认通过**

Run: `go test ./pkg/outbox/ -v -count=1`
Expected: 全部 PASS

- [ ] **Step 5: 全量验证**

Run: `go build ./... && go vet ./... && go test ./... -count=1`
Expected: PASS（此时全仓库测试应有 5 个包通过）

- [ ] **Step 6: Commit**

```bash
git add pkg/outbox/
git commit -m "refactor(outbox): extract publishEvents for testability; add publish status tests"
```

---

### Task 8: pprof 配置开关（安全）+ http.go 提取 newRouter

**背景：** `/debug/pprof/**` 目前无条件挂在业务端口 `:8080` 上，任何能访问服务端口的人都能拉取 CPU profile、命令行参数等敏感信息。修复：新增 `server.pprof_enabled`（viper 默认 **false**），仅显式开启才挂载；本地开发 config.yaml 设为 true 保持 `make pprof` 可用。同时把路由构建抽成 `newRouter` 以便测试（Task 9 复用）。

**Files:**
- Modify: `pkg/config/config.go`（Server 结构体加字段）
- Modify: `pkg/config/loader.go`（SetDefault）
- Modify: `alexgo-server/configs/config.yaml`（本地开启）
- Modify: `alexgo-server/server/http.go`（提取 `newRouter` + 条件挂载）
- Create: `pkg/../alexgo-server/server/http_test.go` → 实际路径 `alexgo-server/server/http_test.go`
- Modify: `deployments/helm/alexgo-cloud/templates/configmap.yaml`、`deployments/helm/alexgo-cloud/values.yaml`、`deployments/helm/alexgo-cloud/values-dev.yaml`（pprof 开关透传）

**Interfaces:**
- Consumes: `HTTPServerParams`（现有）、`StartHTTPServer`
- Produces: `newRouter(p HTTPServerParams) *gin.Engine`（包内私有，Task 9 的 health 测试依赖它）；`Config.Server.PprofEnabled bool`

- [ ] **Step 1: 写失败测试**

创建 `alexgo-server/server/http_test.go`：

```go
package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"alexGo-cloud/pkg/config"
)

func TestPprof_NotMountedByDefault(t *testing.T) {
	cfg := &config.Config{} // pprof_enabled 零值 = false
	r := newRouter(HTTPServerParams{Cfg: cfg})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/debug/pprof/", nil))
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 when pprof disabled", w.Code)
	}
}

func TestPprof_MountedWhenEnabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.PprofEnabled = true
	r := newRouter(HTTPServerParams{Cfg: cfg})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/debug/pprof/cmdline", nil))
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 when pprof enabled", w.Code)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./alexgo-server/server/ -v -count=1`
Expected: 编译失败 `undefined: newRouter`，且 `Config.Server.PprofEnabled` 未定义

- [ ] **Step 3: 实现**

3a. `pkg/config/config.go` 的 `Server` 结构体新增：

```go
// PprofEnabled：是否挂载 /debug/pprof/**。
// 生产默认关闭：pprof 端点会泄露命令行参数、堆栈与运行时信息，应仅在本地/内网调试时开启。
PprofEnabled bool `mapstructure:"pprof_enabled"`
```

3b. `pkg/config/loader.go` 的 `SetDefault` 区新增：

```go
v.SetDefault("server.pprof_enabled", false)
```

3c. `alexgo-server/configs/config.yaml` 的 `server:` 段新增：

```yaml
  pprof_enabled: true
```

3d. `alexgo-server/server/http.go`：把 `StartHTTPServer` 中"构建 Gin Router 到 return r"的整段逻辑（中间件链 + 路由注册）抽为：

```go
// newRouter 构建完整的 Gin 路由（中间件链 + 模块路由 + 运维端点）。
// 抽出为独立函数以便在单测中直接构造路由（pprof 开关、health 行为等）。
func newRouter(p HTTPServerParams) *gin.Engine {
```

`StartHTTPServer` 变为 `r := newRouter(p)` 后接原有 `http.Server` 与生命周期代码。pprof 段改为条件挂载：

```go
	// pprof：默认关闭（server.pprof_enabled=true 才挂载），
	// 避免生产环境通过业务端口泄露运行时信息。
	if p.Cfg.Server.PprofEnabled {
		r.GET("/debug/pprof/", gin.WrapF(pprof.Index))
		r.GET("/debug/pprof/cmdline", gin.WrapF(pprof.Cmdline))
		r.GET("/debug/pprof/profile", gin.WrapF(pprof.Profile))
		r.GET("/debug/pprof/symbol", gin.WrapF(pprof.Symbol))
		r.GET("/debug/pprof/trace", gin.WrapF(pprof.Trace))
	}
```

并更新 `StartHTTPServer` doc comment 中 `/debug/pprof/**` 的说明（"默认关闭，由 server.pprof_enabled 控制"）。

3e. Helm 链路：
- `deployments/helm/alexgo-cloud/templates/configmap.yaml` 的 `server:` 段加一行：`pprof_enabled: {{ .Values.config.server.pprofEnabled }}`
- `deployments/helm/alexgo-cloud/values.yaml`：`config.server` 下加 `pprofEnabled: false`
- `deployments/helm/alexgo-cloud/values-dev.yaml`：同位置加 `pprofEnabled: true`（dev 可调试）

K8s 纯 YAML（`deployments/kubernetes/configmap.yaml`）不加该键 → 走 viper 默认 false，无需改动。

- [ ] **Step 4: 运行确认通过**

Run: `go test ./alexgo-server/server/ ./pkg/config/ -v -count=1`
Expected: pprof 两个测试 PASS，Task 2 的 config 测试仍 PASS

- [ ] **Step 5: 全量验证 + Helm lint**

Run: `go build ./... && go vet ./... && go test ./... -count=1 && helm lint deployments/helm/alexgo-cloud`
Expected: 全部通过（未装 helm 则跳过该项并在完成报告中注明）

- [ ] **Step 6: Commit**

```bash
git add pkg/config/ alexgo-server/ deployments/helm/
git commit -m "feat(server): gate pprof behind server.pprof_enabled (default off); extract newRouter for tests"
```

---

### Task 9: /health 保持存活语义 + /health/ready 数据库探活

**背景：** 现在 K8s 的 liveness **和** readiness 都打 `/health`，而 `/health` 不检查任何依赖。需求 2 要"health 探活"，但直接给 `/health` 加 DB 检查会让 DB 故障期间 liveness 失败→全员重启（雪崩）。正确做法：
- `/health`（liveness）：**保持不依赖 DB**，只证明进程活着
- `/health/ready`（readiness）：ping DB，2s 超时，失败返回 503
- K8s/Helm 的 readinessProbe 改指向 `/health/ready`

**Files:**
- Modify: `alexgo-server/server/http.go`（HTTPServerParams 加 `DB`，注册 `/health/ready`）
- Modify: `alexgo-server/server/http_test.go`（health 测试）
- Modify: `deployments/kubernetes/deployment.yaml`（readinessProbe path）
- Modify: `deployments/helm/alexgo-cloud/templates/deployment.yaml`（readinessProbe path）
- Modify: `README.md`（探针说明）

**Interfaces:**
- Consumes: `newRouter`（Task 8）、`HTTPServerParams`
- Produces: `HTTPServerParams.DB *gorm.DB`（由现有 `database.NewDB` 提供，fx 自动注入）；路由 `GET /health/ready`

- [ ] **Step 1: 写失败测试**

在 `alexgo-server/server/http_test.go` 追加：

```go
package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/pkg/config"
)

// liveness：不依赖 DB，永远 200（进程活着即可）。
func TestHealth_LivenessWithoutDB(t *testing.T) {
	r := newRouter(HTTPServerParams{Cfg: &config.Config{}})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/health", nil))
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

// readiness：DB 正常 → 200。
func TestHealth_ReadyWithLiveDB(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	r := newRouter(HTTPServerParams{Cfg: &config.Config{}, DB: db})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/health/ready", nil))
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
}

// readiness：DB 连接已关闭 → 503（K8s 摘流，但 liveness 仍 200 不触发重启）。
func TestHealth_ReadyWhenDBDown(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	_ = sqlDB.Close()

	r := newRouter(HTTPServerParams{Cfg: &config.Config{}, DB: db})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/health/ready", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", w.Code)
	}

	// 同一时刻 liveness 仍必须 200。
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest("GET", "/health", nil))
	if w2.Code != http.StatusOK {
		t.Errorf("liveness status = %d, want 200 while db down", w2.Code)
	}
}
```

注：文件顶部已有 `net/http`、`net/http/httptest`、`testing` import（Task 8），追加时合并 import 块，补 `time` 是否使用视实现而定（2s 超时在 handler 内部，测试文件若未用到 `time` 就不 import）。

- [ ] **Step 2: 运行确认失败**

Run: `go test ./alexgo-server/server/ -run TestHealth -v`
Expected: 编译失败（`HTTPServerParams` 无 `DB` 字段）或 404（`/health/ready` 未注册）

- [ ] **Step 3: 实现**

3a. `HTTPServerParams` 新增字段：

```go
	// DB：用于 /health/ready 探活（fx 由 database.NewDB 注入）。
	DB *gorm.DB
```

3b. `newRouter` 中在 `/health` 注册之后新增：

```go
	// readiness：依赖探活。DB ping 失败返回 503，供 K8s readinessProbe 摘流。
	// 注意：livenessProbe 继续使用 /health（不依赖 DB），避免 DB 故障触发全员重启。
	r.GET("/health/ready", func(c *gin.Context) {
		if p.DB == nil {
			c.JSON(http.StatusOK, gin.H{"status": "ok", "db": "unconfigured"})
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		sqlDB, err := p.DB.DB()
		if err == nil {
			err = sqlDB.PingContext(ctx)
		}
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable", "db": "down"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "db": "up"})
	})
```

补 import：`"context"`、`"time"`、`"gorm.io/gorm"`（http.go 现有 import 需核对合并）。

3c. 探针配置：
- `deployments/kubernetes/deployment.yaml`：`readinessProbe.httpGet.path` 改为 `/health/ready`
- `deployments/helm/alexgo-cloud/templates/deployment.yaml`：同样改 readinessProbe path（livenessProbe 保持 `/health`）

3d. `README.md` 在合适位置（监控栈之后）加一小节：

```markdown
## 健康检查

- `GET /health`：存活探针（不依赖 DB，livenessProbe 用）
- `GET /health/ready`：就绪探针（ping DB，失败 503，readinessProbe 用）
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./alexgo-server/server/ -v -count=1`
Expected: pprof + health 全部 PASS

- [ ] **Step 5: 全量验证**

Run: `go build ./... && go vet ./... && go test ./... -count=1`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add alexgo-server/ deployments/ README.md
git commit -m "feat(server): add /health/ready DB probe; keep /health as liveness; point readinessProbe to it"
```

---

### Task 10: 清理——删除 frontend/、修 generate-all、README 更新

**Files:**
- Delete: `frontend/`（整个目录）
- Modify: `Makefile:21`
- Modify: `README.md`

**Interfaces:**
- Consumes: 无
- Produces: `make generate-all` = proto + 模板 CRUD；README 反映当前真实用法

**背景：** `frontend/` 是早期 React 试验品（仅 App.tsx + useUsers 一个 hook），与 `admin-web/`（真正的管理后台）职责重复，且全仓库无任何引用（已 grep 确认）。项目非 git 仓库时不可恢复——**本任务在 Task 1 建立 git 之后执行，删除可从 git 历史找回**。

- [ ] **Step 1: 删除前最后确认无引用**

```bash
grep -rn "alexgo-cloud-frontend\|frontend/" --include="*.go" --include="*.yml" --include="*.yaml" --include="*.md" --include="Makefile" --include="*.json" . | grep -v node_modules | grep -v "^./frontend/"
```

Expected: 无输出。若有输出 → 停止，向人报告引用位置。

- [ ] **Step 2: 删除 frontend/**

```bash
git rm -r frontend/
```

（若 `git rm` 报错说不在仓库中，说明 Task 1 未完成——回到 Task 1。）

- [ ] **Step 3: 修 Makefile**

`Makefile:21` 由 `generate-all: proto` 改为：

```make
generate-all: proto generate-crud
```

（`generate-db-crud` 需要活的 MySQL，不纳入 `generate-all`，否则 CI 必挂；`generate-crud` 是 proto+模板驱动，无外部依赖，且生成器对已存在类型会跳过。）

- [ ] **Step 4: 验证生成器安全（关键防回归步骤）**

```bash
make generate-crud
git status --porcelain   # 应无变更或仅有 _gen.go 新文件
go build ./...
```

Expected: `go build` 通过。**若生成器产出无法编译的新文件 → `git checkout -- . && git clean -fd modules/` 撤销，并把 `generate-all: proto generate-crud` 改回 `generate-all: proto`，在完成报告中说明生成器与现有代码冲突**。

- [ ] **Step 5: 更新 README**

5a. "后台前端"小节后补充目录说明（替换/新增）：

```markdown
## 前端目录

- `admin-web/`：管理后台（Vue 3 + Naive UI + Vite），唯一维护的前端
```

5b. 顶部"启动"小节后补充测试说明：

```markdown
## 测试

```bash
make test
```

测试为进程内单元测试，不依赖 MySQL/Redis/NATS。
```

5c. pprof 小节更新为：

```markdown
## pprof

默认关闭（`server.pprof_enabled: false`）。本地 `alexgo-server/configs/config.yaml` 已开启；
生产环境请保持关闭，调试时临时开启并限制网络访问。

```bash
make pprof
```
```

5d. "健康检查"小节与 Task 9 的 3d 相同内容（若 Task 9 已加则跳过）。

- [ ] **Step 6: 验证**

Run: `make generate-all && go build ./... && go vet ./... && go test ./... -count=1 && git status --porcelain`
Expected: 全部通过、工作区除待提交变更外干净

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "chore: remove unused frontend/, fix generate-all target, refresh README"
```

---

### Task 11: 架构文档 docs/architecture.md

**Files:**
- Create: `docs/architecture.md`

**Interfaces:**
- Consumes: 项目现状（本计划执行完毕后的形态）
- Produces: 单一架构参考文档，填空 `docs/`

- [ ] **Step 1: 编写文档**

创建 `docs/architecture.md`，内容按以下结构撰写（依据代码事实，不虚构）：

```markdown
# alexGo-cloud 架构说明

## 1. 定位

模块化单体（Modular Monolith）Go 后台基座，对标 yudao-cloud：
模块独立、统一启动、按需演进微服务。技术栈：Go 1.25 + Gin + Uber FX + GORM(MySQL)
+ Casbin + JWT + Redis + NATS JetStream + OpenTelemetry + Prometheus。

## 2. 目录结构

（列出 alexgo-server / modules / pkg / admin-web / deployments / scripts / tools，
每个目录 1-2 行职责说明）

## 3. 启动与依赖装配（Uber FX）

- 入口：`alexgo-server/cmd/main.go`
- fx.Provide 基础设施 → fx.Invoke 迁移/Relay/Tracer → 模块 FxModule 聚合（group:"modules"）
- `server.Module` 接口：模块通过 `RegisterRoutes(*gin.RouterGroup)` 挂到 /api 下
- `--migrate-only`：只跑数据库迁移（K8s initJob / 发布前）

## 4. 单体 → 微服务切换

- `fx.Decorate` 把 `system.UserService` 本地实现替换为 gRPC 客户端
  （`microservice.enabled=true` 时），调用方零改动
- 模块可独立进程化：`modules/system/cmd/grpc_main.go`

## 5. HTTP 中间件链（顺序即优先级）

Recovery → Tenant(X-Tenant-ID) → Logger → Prometheus → RateLimit+Breaker
→ Auth(JWT+Casbin，仅 /api/admin/**) → OperateLog → OTel → Trace

运维端点：/health（liveness）、/health/ready（readiness，ping DB）、
/metrics、/debug/pprof/**（server.pprof_enabled=true 才挂载）

## 6. 可靠事件：Transactional Outbox

业务事务写 outbox_events(pending) → `pkg/outbox` Relay 每 5s 批量
`FOR UPDATE SKIP LOCKED` 抓取 → 发布到 NATS JetStream → 标记 published/failed。

## 7. 配置体系

优先级：环境变量（DB_DSN/JWT_SECRET/REDIS_PASSWORD 在 Unmarshal 后显式覆盖）
> 模块 yaml（v.Set）> 全局 config.yaml > 默认值。
模块开关 `modules.<name>` 决定是否加载 `modules/<name>/configs/config.yaml`（命名空间合并）。

## 8. 测试策略

全部进程内单元测试，CI 无外部服务：
- miniredis 模拟 Redis（限流 Lua）
- 内存 Casbin（鉴权中间件）
- sqlite 内存库（health 探活）
- fake Broker（Outbox 发布语义）
跑法：`make test`。

## 9. 部署拓扑

- 本地：make run / docker-compose
- K8s：kustomize（deployments/kubernetes）或 Helm 多环境
  （values-dev/gray/prod + ArgoCD Application）
- 密钥：DB_DSN/JWT_SECRET 经 K8s Secret 注入（env 覆盖 config 文件）
- CI/CD：GitHub Actions（lint→test→build→helm lint→docker build）
```

（写正文时展开各节，确保与执行完毕的代码一致——尤其第 5、7 节以 Task 8/9/2 的最终形态为准。）

- [ ] **Step 2: 验证**

Run: `head -50 docs/architecture.md && grep -c "^#" docs/architecture.md`
Expected: 文档存在、章节齐全；内容与代码一致（抽查 pprof、/health/ready、env 覆盖三处描述）

- [ ] **Step 3: Commit**

```bash
git add docs/architecture.md
git commit -m "docs: add architecture overview (modules, fx wiring, middleware, outbox, config, deploy)"
```

---

### Task 12: 密钥治理——K8s/Helm Secret 化 + helm lint

**背景：** `deployments/kubernetes/configmap.yaml` 把 DB 密码写在 ConfigMap（集群内任何能读 ConfigMap 的主体都可见，且 ConfigMap 不是密钥的正确载体）；`deployments/helm/alexgo-cloud/values.yaml` 里 `jwtSecret` 明文进 git。修复：密钥改走 Secret 注入 `DB_DSN`/`JWT_SECRET` 环境变量（Task 2 已保证 env 覆盖生效），values 中密钥字段留空并由部署时 `--set` / 外部密钥系统（Sealed Secrets、External Secrets）提供。

**Files:**
- Create: `deployments/kubernetes/secret.yaml`
- Modify: `deployments/kubernetes/kustomization.yaml`（纳入 secret）
- Modify: `deployments/kubernetes/deployment.yaml`（env secretKeyRef）
- Modify: `deployments/kubernetes/configmap.yaml`（删除顶层明文 `DB_DSN` key，注释说明）
- Create: `deployments/helm/alexgo-cloud/templates/secret.yaml`
- Modify: `deployments/helm/alexgo-cloud/templates/deployment.yaml`（env secretKeyRef）
- Modify: `deployments/helm/alexgo-cloud/templates/configmap.yaml`（dsn/jwt 仅作兜底，加注释）
- Modify: `deployments/helm/alexgo-cloud/values.yaml`（`secrets.dbDsn`/`secrets.jwtSecret` 默认空 + 注释）
- Modify: `deployments/helm/alexgo-cloud/values-dev.yaml`、`values-gray.yaml`、`values-prod.yaml`（同上，dev 可留占位值）

**Interfaces:**
- Consumes: Task 2 的 env 覆盖（`DB_DSN`、`JWT_SECRET`）
- Produces: Secret `alexgo-secrets`（k8s）/ `<release>-secrets`（helm），deployment 通过 `secretKeyRef` 注入

- [ ] **Step 1: K8s Secret**

创建 `deployments/kubernetes/secret.yaml`：

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: alexgo-secrets
  namespace: alexgo-cloud
type: Opaque
stringData:
  # 占位值：真实环境请用 Sealed Secrets / External Secrets / kubectl create secret 覆盖，
  # 切勿把真实密码提交进 git。
  DB_DSN: "root:change-me@tcp(mysql.alexgo-cloud.svc.cluster.local:3306)/alexgo?charset=utf8mb4&parseTime=True&loc=Local"
  JWT_SECRET: "change-me"
```

`deployments/kubernetes/kustomization.yaml` 的 `resources:` 列表加入 `secret.yaml`。

`deployments/kubernetes/deployment.yaml` 容器 `envFrom` 同级新增：

```yaml
          env:
            - name: DB_DSN
              valueFrom:
                secretKeyRef:
                  name: alexgo-secrets
                  key: DB_DSN
            - name: JWT_SECRET
              valueFrom:
                secretKeyRef:
                  name: alexgo-secrets
                  key: JWT_SECRET
```

`deployments/kubernetes/configmap.yaml`：删除顶层 `DB_DSN: "root:password@..."` 一行（它是死变量且明文）；config.yaml 内嵌 dsn 保留为"无 Secret 时的兜底"，并加注释 `# dsn 兜底：生产应由环境变量 DB_DSN（Secret）覆盖`。

- [ ] **Step 2: Helm Secret 模板**

创建 `deployments/helm/alexgo-cloud/templates/secret.yaml`：

```yaml
{{- if or .Values.secrets.dbDsn .Values.secrets.jwtSecret }}
apiVersion: v1
kind: Secret
metadata:
  name: {{ include "alexgo-cloud.fullname" . }}-secrets
type: Opaque
stringData:
  {{- if .Values.secrets.dbDsn }}
  DB_DSN: {{ .Values.secrets.dbDsn | quote }}
  {{- end }}
  {{- if .Values.secrets.jwtSecret }}
  JWT_SECRET: {{ .Values.secrets.jwtSecret | quote }}
  {{- end }}
{{- end }}
```

（`_helpers.tpl` 只有 `alexgo-cloud.name` / `alexgo-cloud.fullname` 两个 helper，无 labels helper——与现有 configmap 模板一致，不加 labels 段。）

`templates/deployment.yaml` 容器 spec 新增：

```yaml
            env:
              - name: DB_DSN
                valueFrom:
                  secretKeyRef:
                    name: {{ include "alexgo-cloud.fullname" . }}-secrets
                    key: DB_DSN
              - name: JWT_SECRET
                valueFrom:
                  secretKeyRef:
                    name: {{ include "alexgo-cloud.fullname" . }}-secrets
                    key: JWT_SECRET
```

（同样：若 values 中对应密钥为空则 Secret 不生成，此时 secretKeyRef 会导致 Pod 起不来——deployment 里给 secretKeyRef 加 `optional: true`，保证"未配置 Secret 时回退 configmap 的 config.yaml 兜底"这一原有行为不被破坏。）

`values.yaml` 末尾新增：

```yaml
# 密钥：强烈建议部署时通过 --set / -f <外部密钥文件> / Sealed Secrets 注入，
# 不要把真实值写进本文件提交 git。留空则不创建 Secret，回退 config.yaml 兜底（不推荐用于生产）。
secrets:
  dbDsn: ""
  jwtSecret: ""
```

`values-dev.yaml` / `values-gray.yaml` / `values-prod.yaml` 各加同样的 `secrets:` 段（dev 可填 `change-me` 占位；gray/prod 留空 + 注释"部署时注入"）。

- [ ] **Step 3: 验证渲染**

```bash
helm lint deployments/helm/alexgo-cloud
helm template test deployments/helm/alexgo-cloud --set secrets.jwtSecret=abc --set secrets.dbDsn='root:x@tcp(mysql:3306)/alexgo' | grep -A3 "kind: Secret"
helm template test deployments/helm/alexgo-cloud | grep -c "kind: Secret" || true
```

Expected: lint 通过；带 --set 时渲染出 Secret 与 env secretKeyRef；不带时无 Secret。
（未安装 helm → 用 `kubectl apply --dry-run=client -k deployments/kubernetes` 替代验证 K8s 部分，helm 部分注明"未本地验证，由 CI helm lint 覆盖"。）

- [ ] **Step 4: 全量验证**

Run: `go build ./... && go test ./... -count=1 && helm lint deployments/helm/alexgo-cloud`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add deployments/
git commit -m "sec(deploy): move DB_DSN/JWT_SECRET into Secrets; values use empty placeholders"
```

---

## 计划收尾验证（全部任务完成后执行一次）

```bash
go build ./... && go vet ./... && go test ./... -count=1 -race
make generate-all
git status --porcelain   # 应为空
helm lint deployments/helm/alexgo-cloud   # 若安装了 helm
```

Expected: 全绿。测试包至少覆盖：config、auth、circuitbreaker、limiter、middleware、outbox、server 共 7 个。
