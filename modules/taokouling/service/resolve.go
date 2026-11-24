package service

import (
	"context"
	"strings"

	"alexGo-cloud/modules/taokouling/model"
	errs "alexGo-cloud/pkg/errors"
)

// ChannelStore 渠道查询依赖（Task 4 repository.ChannelRepository 结构化满足，方法签名逐字）。
type ChannelStore interface {
	GetEnabledByPartner(ctx context.Context, partner string) (*model.Channel, error)
	GetEnabledByHost(ctx context.Context, host string) (*model.Channel, error)
	GetDefault(ctx context.Context) (*model.Channel, error)
}

// Resolve 按 spec §7 解析渠道：partner（剥 "#" 前缀）→ host → 默认行。
// 三级均 miss（nil, nil）→ errs.CodeTaoKoulingChannelMissing（30703）；
// store 查询错误原样上抛。ctx 由调用方保证已租户钉定，Resolve 不重复钉。
func Resolve(ctx context.Context, store ChannelStore, partner, host string) (*model.Channel, error) {
	partner = strings.TrimPrefix(partner, "#")
	if partner != "" {
		ch, err := store.GetEnabledByPartner(ctx, partner)
		if err != nil {
			return nil, err
		}
		if ch != nil {
			return ch, nil
		}
	}
	if host != "" {
		ch, err := store.GetEnabledByHost(ctx, host)
		if err != nil {
			return nil, err
		}
		if ch != nil {
			return ch, nil
		}
	}
	ch, err := store.GetDefault(ctx)
	if err != nil {
		return nil, err
	}
	if ch == nil {
		return nil, errs.New(errs.CodeTaoKoulingChannelMissing, nil)
	}
	return ch, nil
}
