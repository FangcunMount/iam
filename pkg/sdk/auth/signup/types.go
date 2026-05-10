// Package signup provides REST AuthN v2 clients for public signup and internal mock-consumer onboarding.
package signup

import (
	"strings"

	"github.com/FangcunMount/iam/v2/pkg/sdk/auth/internal/restclient"
)

type WechatMiniProgramRequest struct {
	Name     string         `json:"name"`
	Phone    string         `json:"phone,omitempty"`
	Email    string         `json:"email,omitempty"`
	AppID    string         `json:"appId"`
	JsCode   string         `json:"jsCode"`
	Nickname *string        `json:"nickname,omitempty"`
	Avatar   *string        `json:"avatar,omitempty"`
	Meta     map[string]any `json:"meta,omitempty"`
}

func (r WechatMiniProgramRequest) Validate() error {
	if strings.TrimSpace(r.Name) == "" {
		return restclient.InvalidArgument("name is required")
	}
	if strings.TrimSpace(r.AppID) == "" {
		return restclient.InvalidArgument("appId is required")
	}
	if strings.TrimSpace(r.JsCode) == "" {
		return restclient.InvalidArgument("jsCode is required")
	}
	return nil
}

type SignupResult struct {
	UserID          uint64            `json:"userId"`
	UserName        string            `json:"userName"`
	Phone           string            `json:"phone"`
	Email           string            `json:"email,omitempty"`
	LoginIdentityID uint64            `json:"loginIdentityId"`
	Credential      *SignupCredential `json:"credential"`
	IsNewUser       bool              `json:"isNewUser"`
	IsNewIdentity   bool              `json:"isNewIdentity"`
}

type SignupCredential struct {
	ID   uint64 `json:"id"`
	Type string `json:"type"`
}

type EnsureMockConsumerRequest struct {
	Name     string            `json:"name"`
	Phone    string            `json:"phone"`
	Email    string            `json:"email"`
	Password string            `json:"password"`
	Profile  map[string]string `json:"profile,omitempty"`
	Meta     map[string]string `json:"meta,omitempty"`
}

func (r EnsureMockConsumerRequest) Validate() error {
	if strings.TrimSpace(r.Name) == "" {
		return restclient.InvalidArgument("name is required")
	}
	if strings.TrimSpace(r.Phone) == "" {
		return restclient.InvalidArgument("phone is required")
	}
	if strings.TrimSpace(r.Email) == "" {
		return restclient.InvalidArgument("email is required")
	}
	if strings.TrimSpace(r.Password) == "" {
		return restclient.InvalidArgument("password is required")
	}
	return nil
}

type EnsureMockConsumerResult struct {
	UserID          string `json:"user_id"`
	LoginIdentityID string `json:"login_identity_id"`
	LoginID         string `json:"login_id"`
	IsNewUser       bool   `json:"is_new_user"`
	IsNewIdentity   bool   `json:"is_new_identity"`
}
