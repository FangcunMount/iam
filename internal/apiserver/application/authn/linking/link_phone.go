package linking

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	challengeapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/challenge"
	loginidentity "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/loginidentity"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// LinkPhoneCommand 是手机号登录身份绑定命令。
type LinkPhoneCommand struct {
	UserID  meta.ID
	Phone   string
	OTPCode string
}

// SendPhoneLinkChallenge 为当前已认证用户发送手机号绑定验证码。
func (s *service) SendPhoneLinkChallenge(ctx context.Context, userID meta.ID, rawPhone string) error {
	if err := requireUserID(userID); err != nil {
		return err
	}
	if s.deps.Challenge == nil {
		return perrors.WithCode(code.ErrInvalidArgument, "phone link challenge is not configured")
	}
	return s.deps.Challenge.SendSMSOTP(ctx, challengeapp.SceneLinkPhoneOTP, rawPhone)
}

// LinkPhone 在验证码通过后为当前用户绑定手机号登录身份。
func (s *service) LinkPhone(ctx context.Context, cmd LinkPhoneCommand) (*LinkResult, error) {
	if err := requireUserID(cmd.UserID); err != nil {
		return nil, err
	}
	if s.deps.Challenge == nil {
		return nil, perrors.WithCode(code.ErrInternalServerError, "challenge service is not configured")
	}
	phone, err := meta.NewPhone(cmd.Phone)
	if err != nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "invalid phone: %v", err)
	}
	ok := s.deps.Challenge.VerifyAndConsume(ctx, challengeapp.SceneLinkPhoneOTP, phone.String(), cmd.OTPCode)
	if !ok {
		return nil, perrors.WithCode(code.ErrInvalidCredential, "invalid phone link challenge")
	}
	key := loginidentity.PhoneProviderKey(phone.String())
	return s.ensureProviderKey(ctx, cmd.UserID, key, func() (*loginidentity.LoginIdentity, error) {
		return loginidentity.NewBuilder(cmd.UserID).
			FromProviderKey(key).
			WithVerifiedAt(s.now()).
			Build()
	})
}
