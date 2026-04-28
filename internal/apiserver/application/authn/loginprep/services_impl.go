package loginprep

import (
	"context"
	"fmt"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

type loginPreparationService struct {
	phoneOTP *PhoneOTPDeps
}

var _ LoginPreparationService = (*loginPreparationService)(nil)

// NewLoginPreparationService 构造登录预准备应用服务
func NewLoginPreparationService(phoneOTP *PhoneOTPDeps) LoginPreparationService {
	return &loginPreparationService{phoneOTP: phoneOTP}
}

// SendPhoneOTPForLogin 发送登录短信验证码
func (s *loginPreparationService) SendPhoneOTPForLogin(ctx context.Context, rawPhone string) error {
	l := logger.L(ctx)
	if s.phoneOTP == nil || s.phoneOTP.Store == nil || s.phoneOTP.Gate == nil || s.phoneOTP.SMS == nil {
		return perrors.WithCode(code.ErrInvalidArgument, "login phone OTP is not configured")
	}

	phone, err := meta.NewPhone(rawPhone)
	if err != nil {
		return perrors.WithCode(code.ErrInvalidArgument, "invalid phone: %v", err)
	}
	e164 := phone.String()
	cooldown := s.phoneOTP.effectiveCooldown()
	ttl := s.phoneOTP.effectiveTTL()
	codeLen := s.phoneOTP.effectiveCodeLen()

	ok, err := s.phoneOTP.Gate.TryAcquire(ctx, e164, loginOTPScene, cooldown)
	if err != nil {
		return fmt.Errorf("login otp send gate: %w", err)
	}
	if !ok {
		return perrors.WithCode(code.ErrOTPSendTooFrequent, "please wait before requesting another code")
	}

	otp, err := randomNumericOTP(codeLen)
	if err != nil {
		return perrors.WithCode(code.ErrInternalServerError, "failed to generate otp: %v", err)
	}

	if err := s.phoneOTP.Store.Put(ctx, e164, loginOTPScene, otp, ttl); err != nil {
		return fmt.Errorf("store login otp: %w", err)
	}

	if err := s.phoneOTP.SMS.SendLoginOTP(ctx, e164, otp); err != nil {
		_ = s.phoneOTP.Store.Delete(ctx, e164, loginOTPScene, otp)
		return fmt.Errorf("send login otp sms: %w", err)
	}

	l.Debugw("login phone otp sent",
		"action", logger.ActionLogin,
		"phase", "login_prep",
		"phone", e164,
		"result", logger.ResultSuccess,
	)
	return nil
}
