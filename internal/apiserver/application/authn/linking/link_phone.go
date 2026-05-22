package linking

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	loginidentity "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/loginidentity"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// PrepareLink 准备手机号登录身份。
func (in LinkPhoneInput) prepareLink(ctx context.Context, deps linkPrepareDeps, userID meta.ID) (preparedLink, error) {
	// 检查挑战服务是否配置。
	if deps.phoneLinkOTP == nil {
		return preparedLink{}, perrors.WithCode(code.ErrInternalServerError, "challenge service is not configured")
	}

	// 检查手机号是否有效。
	phone, err := meta.NewPhone(in.Phone)
	if err != nil {
		return preparedLink{}, perrors.WithCode(code.ErrInvalidArgument, "invalid phone: %v", err)
	}

	// 验证挑战码。
	ok := deps.phoneLinkOTP.VerifyAndConsumePhoneLinkOTP(ctx, phone.String(), in.OTPCode)
	if !ok {
		return preparedLink{}, perrors.WithCode(code.ErrInvalidCredential, "invalid phone link challenge")
	}

	// 构建提供者密钥。
	key := loginidentity.PhoneProviderKey(phone.String())

	// 构建已验证登录身份。
	verifiedAt := deps.currentTime()
	return preparedLink{
		key:   key,
		build: verifiedIdentityBuild(userID, key, verifiedAt),
	}, nil
}
