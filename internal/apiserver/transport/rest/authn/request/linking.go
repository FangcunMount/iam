package request

import (
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

type LinkPhoneChallengeRequest struct {
	Phone string `json:"phone" binding:"required"`
}

func (r LinkPhoneChallengeRequest) Validate() error {
	if strings.TrimSpace(r.Phone) == "" {
		return perrors.WithCode(code.ErrInvalidArgument, "phone is required")
	}
	return nil
}

type LinkPhoneRequest struct {
	Phone   string `json:"phone" binding:"required"`
	OTPCode string `json:"otp_code" binding:"required"`
}

func (r LinkPhoneRequest) Validate() error {
	if strings.TrimSpace(r.Phone) == "" {
		return perrors.WithCode(code.ErrInvalidArgument, "phone is required")
	}
	if strings.TrimSpace(r.OTPCode) == "" {
		return perrors.WithCode(code.ErrInvalidArgument, "otp_code is required")
	}
	return nil
}

type LinkWechatMiniProgramRequest struct {
	AppID string `json:"app_id" binding:"required"`
	Code  string `json:"code" binding:"required"`
}

func (r LinkWechatMiniProgramRequest) Validate() error {
	if strings.TrimSpace(r.AppID) == "" {
		return perrors.WithCode(code.ErrInvalidArgument, "app_id is required")
	}
	if strings.TrimSpace(r.Code) == "" {
		return perrors.WithCode(code.ErrInvalidArgument, "code is required")
	}
	return nil
}

// LinkWechatOpenAuthorizeRequest 发起微信开放平台扫码绑定授权（可选 nonce）。
type LinkWechatOpenAuthorizeRequest struct {
	Nonce string `json:"nonce"`
}

// LinkWechatOpenRequest 完成微信开放平台扫码绑定回调（code/state 来自微信回调）。
type LinkWechatOpenRequest struct {
	Code  string `json:"code" binding:"required"`
	State string `json:"state" binding:"required"`
}

func (r LinkWechatOpenRequest) Validate() error {
	if strings.TrimSpace(r.Code) == "" {
		return perrors.WithCode(code.ErrInvalidArgument, "code is required")
	}
	if strings.TrimSpace(r.State) == "" {
		return perrors.WithCode(code.ErrInvalidArgument, "state is required")
	}
	return nil
}

type LinkWecomRequest struct {
	CorpID string `json:"corp_id" binding:"required"`
	Code   string `json:"code" binding:"required"`
}

func (r LinkWecomRequest) Validate() error {
	if strings.TrimSpace(r.CorpID) == "" {
		return perrors.WithCode(code.ErrInvalidArgument, "corp_id is required")
	}
	if strings.TrimSpace(r.Code) == "" {
		return perrors.WithCode(code.ErrInvalidArgument, "code is required")
	}
	return nil
}
