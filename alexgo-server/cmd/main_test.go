package main

import (
	"strings"
	"testing"

	"go.uber.org/fx"

	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/token"
)

// TestMono_GraphValidates 正向干跑：mono 模式（默认，非 micro）装配
// system+order+member 全量依赖图完整（dig dry-run：不执行构造器/Invoke，
// 不需要配置文件与 DB）。
func TestMono_GraphValidates(t *testing.T) {
	cfg := &config.Config{} // deployment.mode 空 → 非 micro → mono（含 member 模块）
	if err := fx.ValidateApp(append(options(cfg, false), fx.NopLogger)...); err != nil {
		t.Fatalf("mono 图干跑失败: %v", err)
	}
}

// TestMicro_GraphValidates 正向干跑：DEPLOYMENT_MODE=micro 时 member.FxModule
// 被条件剔除，system+order 图仍完整（system-server 进程形态）。
func TestMicro_GraphValidates(t *testing.T) {
	cfg := &config.Config{}
	cfg.Deployment.Mode = "micro"
	if err := fx.ValidateApp(append(options(cfg, false), fx.NopLogger)...); err != nil {
		t.Fatalf("micro 图干跑失败: %v", err)
	}
}

// TestMigrateOnly_GraphValidates 正向干跑：--migrate-only（不起 HTTP）图完整。
func TestMigrateOnly_GraphValidates(t *testing.T) {
	cfg := &config.Config{}
	if err := fx.ValidateApp(append(options(cfg, true), fx.NopLogger)...); err != nil {
		t.Fatalf("migrate-only 图干跑失败: %v", err)
	}
}

// TestBaseOptions_RequiresTokenIssuer 反向干跑：剔除 token.Issuer/Validator
// 接口映射（ifaceOptions）后，system 模块 NewAuthService 消费 token.Issuer
// 必须报缺——证明接口映射是入口真实依赖而非挂空（T4 裁决的映射不可丢）。
func TestBaseOptions_RequiresTokenIssuer(t *testing.T) {
	err := fx.ValidateApp(append(baseOptions(&config.Config{}, false), fx.NopLogger)...)
	if err == nil {
		t.Fatal("剔除接口映射的图必须不可满足")
	}
	if !strings.Contains(err.Error(), "token.Issuer") {
		t.Fatalf("error = %v, want missing token.Issuer", err)
	}
}

// TestBaseOptions_RequiresTokenValidator 反向干跑（与 Issuer 成对）：
// token.Validator 的生产消费方 HTTPServerParams.TokenValidator 是 optional——
// 图缺该映射也能成立，故用测试内真实消费者（fx.Invoke 消费 token.Validator）
// 构造根。反向用 migrate-only 图（不起 HTTP → 模块链非根，避免模块链上的
// token.Issuer 先被报缺而遮蔽 Validator 判别）：剔除 ifaceOptions → 必须报缺
// token.Validator；正向用完整生产图 + 消费者 → 必须可满足——证明映射真实产出
// 接口类型（fx 按具体类型 *token.Service 提供、不自动满足接口）。
func TestBaseOptions_RequiresTokenValidator(t *testing.T) {
	consumeValidator := fx.Invoke(func(v token.Validator) { _ = v })

	err := fx.ValidateApp(append(baseOptions(&config.Config{}, true), consumeValidator, fx.NopLogger)...)
	if err == nil {
		t.Fatal("剔除接口映射且存在 Validator 消费者时必须不可满足")
	}
	if !strings.Contains(err.Error(), "token.Validator") {
		t.Fatalf("error = %v, want missing token.Validator", err)
	}

	full := append(append(options(&config.Config{}, false), consumeValidator), fx.NopLogger)
	if err := fx.ValidateApp(full...); err != nil {
		t.Fatalf("带接口映射的完整图必须可满足: %v", err)
	}
}
