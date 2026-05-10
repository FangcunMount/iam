// Package loginidentity provides a REST AuthN v2 client for managing linked login identities.
package loginidentity

import (
	"strings"
	"time"

	"github.com/FangcunMount/iam/v2/pkg/sdk/auth/internal/restclient"
)

type LoginIdentity struct {
	ID               string     `json:"id"`
	Provider         string     `json:"provider"`
	Realm            string     `json:"realm"`
	Identifier       string     `json:"identifier"`
	GlobalIdentifier string     `json:"global_identifier,omitempty"`
	Status           string     `json:"status"`
	VerifiedAt       *time.Time `json:"verified_at,omitempty"`
	LinkedAt         time.Time  `json:"linked_at"`
}

type ListResponse struct {
	Items []LoginIdentity `json:"items"`
}

type LinkResponse struct {
	LoginIdentity LoginIdentity `json:"login_identity"`
	Reused        bool          `json:"reused"`
}

type MessageResponse struct {
	Message string `json:"message"`
}

type LinkPhoneChallengeRequest struct {
	Phone string `json:"phone"`
}

func (r LinkPhoneChallengeRequest) Validate() error {
	if strings.TrimSpace(r.Phone) == "" {
		return restclient.InvalidArgument("phone is required")
	}
	return nil
}

type LinkPhoneRequest struct {
	Phone   string `json:"phone"`
	OTPCode string `json:"otp_code"`
}

func (r LinkPhoneRequest) Validate() error {
	if strings.TrimSpace(r.Phone) == "" {
		return restclient.InvalidArgument("phone is required")
	}
	if strings.TrimSpace(r.OTPCode) == "" {
		return restclient.InvalidArgument("otp_code is required")
	}
	return nil
}

type LinkWechatMiniProgramRequest struct {
	AppID string `json:"app_id"`
	Code  string `json:"code"`
}

func (r LinkWechatMiniProgramRequest) Validate() error {
	if strings.TrimSpace(r.AppID) == "" {
		return restclient.InvalidArgument("app_id is required")
	}
	if strings.TrimSpace(r.Code) == "" {
		return restclient.InvalidArgument("code is required")
	}
	return nil
}

type LinkWecomRequest struct {
	CorpID string `json:"corp_id"`
	Code   string `json:"code"`
}

func (r LinkWecomRequest) Validate() error {
	if strings.TrimSpace(r.CorpID) == "" {
		return restclient.InvalidArgument("corp_id is required")
	}
	if strings.TrimSpace(r.Code) == "" {
		return restclient.InvalidArgument("code is required")
	}
	return nil
}
