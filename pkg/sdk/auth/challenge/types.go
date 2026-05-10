// Package challenge provides a REST AuthN v2 client for public authentication challenges.
package challenge

import (
	"strings"

	"github.com/FangcunMount/iam/v2/pkg/sdk/auth/internal/restclient"
)

type SendLoginPhoneOTPRequest struct {
	Phone string `json:"phone"`
}

func (r SendLoginPhoneOTPRequest) Validate() error {
	if strings.TrimSpace(r.Phone) == "" {
		return restclient.InvalidArgument("phone is required")
	}
	return nil
}

type MessageResponse struct {
	Message string `json:"message"`
}
