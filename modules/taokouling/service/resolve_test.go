package service

import (
	"context"
	"testing"

	"alexGo-cloud/modules/taokouling/model"
	errs "alexGo-cloud/pkg/errors"
)

// fakeChannelStore 三级查询桩：按需返回并记录调用次数与入参。
type fakeChannelStore struct {
	partnerCh  *model.Channel
	partnerErr error
	hostCh     *model.Channel
	hostErr    error
	defaultCh  *model.Channel
	defaultErr error

	partnerCalls int
	hostCalls    int
	defaultCalls int
	partnerArgs  []string
	hostArgs     []string
}

func (f *fakeChannelStore) GetEnabledByPartner(_ context.Context, partner string) (*model.Channel, error) {
	f.partnerCalls++
	f.partnerArgs = append(f.partnerArgs, partner)
	return f.partnerCh, f.partnerErr
}

func (f *fakeChannelStore) GetEnabledByHost(_ context.Context, host string) (*model.Channel, error) {
	f.hostCalls++
	f.hostArgs = append(f.hostArgs, host)
	return f.hostCh, f.hostErr
}

func (f *fakeChannelStore) GetDefault(_ context.Context) (*model.Channel, error) {
	f.defaultCalls++
	return f.defaultCh, f.defaultErr
}

func mkCh(name string) *model.Channel {
	return &model.Channel{Name: name, Status: 1}
}

func TestResolve_PartnerWins(t *testing.T) {
	want := mkCh("partner")
	store := &fakeChannelStore{partnerCh: want, hostCh: mkCh("host"), defaultCh: mkCh("default")}

	got, err := Resolve(context.Background(), store, "tang", "s.taobao.com")
	if err != nil {
		t.Fatalf("Resolve err=%v, want nil", err)
	}
	if got != want {
		t.Fatalf("Resolve got=%+v, want partner 行 %+v", got, want)
	}
	if store.hostCalls != 0 || store.defaultCalls != 0 {
		t.Fatalf("partner 命中不应调 Host/Default: host=%d default=%d, want 0/0",
			store.hostCalls, store.defaultCalls)
	}
}

func TestResolve_HashPrefixStripped(t *testing.T) {
	want := mkCh("tang")
	store := &fakeChannelStore{partnerCh: want}

	got, err := Resolve(context.Background(), store, "#tang", "")
	if err != nil {
		t.Fatalf("Resolve err=%v, want nil", err)
	}
	if got != want {
		t.Fatalf("Resolve got=%+v, want %+v", got, want)
	}
	if len(store.partnerArgs) != 1 || store.partnerArgs[0] != "tang" {
		t.Fatalf("#tang 应按 tang 查询, partnerArgs=%v, want [tang]", store.partnerArgs)
	}
}

func TestResolve_UnknownPartnerFallsToHostThenDefault(t *testing.T) {
	t.Run("partner miss → host hit", func(t *testing.T) {
		wantHost := mkCh("host")
		store := &fakeChannelStore{hostCh: wantHost, defaultCh: mkCh("default")}

		got, err := Resolve(context.Background(), store, "nope", "h.example.com")
		if err != nil {
			t.Fatalf("Resolve err=%v, want nil", err)
		}
		if got != wantHost {
			t.Fatalf("Resolve got=%+v, want host 行 %+v", got, wantHost)
		}
		if store.partnerCalls != 1 || store.hostCalls != 1 {
			t.Fatalf("应先查 partner 再查 host: partner=%d host=%d, want 1/1",
				store.partnerCalls, store.hostCalls)
		}
		if store.defaultCalls != 0 {
			t.Fatalf("host 命中不应调 Default: default=%d, want 0", store.defaultCalls)
		}
	})

	t.Run("partner miss + host miss → default", func(t *testing.T) {
		wantDef := mkCh("default")
		store := &fakeChannelStore{defaultCh: wantDef}

		got, err := Resolve(context.Background(), store, "nope", "h.example.com")
		if err != nil {
			t.Fatalf("Resolve err=%v, want nil", err)
		}
		if got != wantDef {
			t.Fatalf("Resolve got=%+v, want 默认行 %+v", got, wantDef)
		}
		if store.partnerCalls != 1 || store.hostCalls != 1 || store.defaultCalls != 1 {
			t.Fatalf("三级依次查询: partner=%d host=%d default=%d, want 1/1/1",
				store.partnerCalls, store.hostCalls, store.defaultCalls)
		}
	})
}

func TestResolve_HostHitSkipsDefault(t *testing.T) {
	want := mkCh("host")
	store := &fakeChannelStore{hostCh: want, defaultCh: mkCh("default")}

	got, err := Resolve(context.Background(), store, "", "h.example.com")
	if err != nil {
		t.Fatalf("Resolve err=%v, want nil", err)
	}
	if got != want {
		t.Fatalf("Resolve got=%+v, want host 行 %+v", got, want)
	}
	if store.partnerCalls != 0 {
		t.Fatalf("partner 为空不应查询: partner=%d, want 0", store.partnerCalls)
	}
	if store.defaultCalls != 0 {
		t.Fatalf("host 命中不应调 Default: default=%d, want 0", store.defaultCalls)
	}
}

func TestResolve_NoDefault_30703(t *testing.T) {
	store := &fakeChannelStore{} // 三级全 miss：(nil, nil)

	got, err := Resolve(context.Background(), store, "nope", "h.example.com")
	if got != nil {
		t.Fatalf("三级全 miss 应返回 nil 行, got=%+v", got)
	}
	if !errs.Is(err, errs.CodeTaoKoulingChannelMissing) {
		t.Fatalf("err=%v, want errs.Is(err, CodeTaoKoulingChannelMissing=30703)", err)
	}
	if store.partnerCalls != 1 || store.hostCalls != 1 || store.defaultCalls != 1 {
		t.Fatalf("三级全 miss 应依次查询: partner=%d host=%d default=%d, want 1/1/1",
			store.partnerCalls, store.hostCalls, store.defaultCalls)
	}
}

func TestResolve_EmptyPartnerHostDirectDefault(t *testing.T) {
	want := mkCh("default")
	store := &fakeChannelStore{defaultCh: want}

	got, err := Resolve(context.Background(), store, "", "")
	if err != nil {
		t.Fatalf("Resolve err=%v, want nil", err)
	}
	if got != want {
		t.Fatalf("Resolve got=%+v, want 默认行 %+v", got, want)
	}
	if store.partnerCalls != 0 || store.hostCalls != 0 {
		t.Fatalf("空参不应查 partner/host: partner=%d host=%d, want 0/0",
			store.partnerCalls, store.hostCalls)
	}
	if store.defaultCalls != 1 {
		t.Fatalf("空参应直接查默认: default=%d, want 1", store.defaultCalls)
	}
}
