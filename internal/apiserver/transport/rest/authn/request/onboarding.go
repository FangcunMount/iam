package request

import (
	"encoding/json"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// SignUpWithWeChatMiniProgramRequest 微信小程序登录身份开通请求。
type SignUpWithWeChatMiniProgramRequest struct {
	Name     string                 `json:"name" binding:"required"`
	Phone    string                 `json:"phone,omitempty"`
	Email    string                 `json:"email,omitempty"`
	AppID    string                 `json:"appId" binding:"required"`
	JsCode   string                 `json:"jsCode" binding:"required"`
	Nickname *string                `json:"nickname,omitempty"`
	Avatar   *string                `json:"avatar,omitempty"`
	Meta     map[string]interface{} `json:"meta,omitempty"`
}

// EnsureMockConsumerReq 内部 mock C 端登录身份 ensure 请求。
type EnsureMockConsumerReq struct {
	Name     string            `json:"name" binding:"required"`
	Phone    string            `json:"phone" binding:"required"`
	Email    string            `json:"email" binding:"required"`
	Password string            `json:"password" binding:"required"`
	Profile  map[string]string `json:"profile,omitempty"`
	Meta     map[string]string `json:"meta,omitempty"`
}

func (r *EnsureMockConsumerReq) Validate() error {
	if strings.TrimSpace(r.Name) == "" {
		return perrors.WithCode(code.ErrInvalidArgument, "name is required")
	}
	if strings.TrimSpace(r.Phone) == "" {
		return perrors.WithCode(code.ErrInvalidArgument, "phone is required")
	}
	if strings.TrimSpace(r.Email) == "" {
		return perrors.WithCode(code.ErrInvalidArgument, "email is required")
	}
	if strings.TrimSpace(r.Password) == "" {
		return perrors.WithCode(code.ErrInvalidArgument, "password is required")
	}
	return nil
}

func (r *SignUpWithWeChatMiniProgramRequest) Validate() error {
	if strings.TrimSpace(r.Name) == "" {
		return perrors.WithCode(code.ErrInvalidArgument, "name is required")
	}
	if strings.TrimSpace(r.AppID) == "" {
		return perrors.WithCode(code.ErrInvalidArgument, "appId is required")
	}
	if strings.TrimSpace(r.JsCode) == "" {
		return perrors.WithCode(code.ErrInvalidArgument, "jsCode is required")
	}
	if phone := strings.TrimSpace(r.Phone); phone != "" {
		if _, err := meta.NewPhone(phone); err != nil {
			return perrors.WithCode(code.ErrInvalidArgument, "invalid phone: %v", err)
		}
	}
	if email := strings.TrimSpace(r.Email); email != "" {
		if _, err := meta.NewEmail(email); err != nil {
			return perrors.WithCode(code.ErrInvalidArgument, "invalid email: %v", err)
		}
	}
	return nil
}

func (r *SignUpWithWeChatMiniProgramRequest) MetaJSON() (map[string]string, error) {
	if r.Meta == nil {
		return nil, nil
	}
	result := make(map[string]string, len(r.Meta))
	for k, v := range r.Meta {
		if str, ok := v.(string); ok {
			result[k] = str
			continue
		}
		data, err := json.Marshal(v)
		if err != nil {
			return nil, perrors.WithCode(code.ErrInvalidArgument, "invalid meta value for %s: %v", k, err)
		}
		result[k] = string(data)
	}
	return result, nil
}
