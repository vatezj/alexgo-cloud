package service

import (
	"context"
	"fmt"

	"alexGo-cloud/modules/starfuyu/model"
	"alexGo-cloud/modules/starfuyu/repository"
	errs "alexGo-cloud/pkg/errors"
)

// ProfileService 用户星座偏好（spec §4.1 /profile）。
type ProfileService interface {
	// GetMy 未设星座 → (nil, nil)（controller 映射空串，不报错）。
	GetMy(ctx context.Context, memberUserID uint64) (*model.Profile, error)
	// SetMy 校验 12 星座枚举（IsValidConstellation），非法 → 10001；upsert。
	SetMy(ctx context.Context, memberUserID uint64, constellation string) (*model.Profile, error)
}

type profileService struct {
	profiles repository.ProfileRepository
}

func NewProfileService(profiles repository.ProfileRepository) ProfileService {
	return &profileService{profiles: profiles}
}

func (s *profileService) GetMy(ctx context.Context, memberUserID uint64) (*model.Profile, error) {
	// 仓储软查 miss → (nil, nil) 直透。
	return s.profiles.GetByMember(ctx, memberUserID)
}

func (s *profileService) SetMy(ctx context.Context, memberUserID uint64, constellation string) (*model.Profile, error) {
	if !IsValidConstellation(constellation) {
		return nil, errs.New(errs.CodeParamInvalid, fmt.Errorf("非法星座: %s", constellation))
	}
	return s.profiles.Upsert(ctx, memberUserID, constellation)
}
