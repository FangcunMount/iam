package handler

import (
	"fmt"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	appAccount "github.com/FangcunMount/iam/internal/apiserver/application/authn/account"
	appRegister "github.com/FangcunMount/iam/internal/apiserver/application/authn/register"
	domain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/account"
	req "github.com/FangcunMount/iam/internal/apiserver/transport/rest/authn/request"
	resp "github.com/FangcunMount/iam/internal/apiserver/transport/rest/authn/response"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

func parseAccountID(idStr string) (meta.ID, error) {
	idStr = strings.TrimSpace(idStr)
	if idStr == "" {
		return meta.FromUint64(0), perrors.WithCode(code.ErrInvalidArgument, "accountId is required")
	}

	var id uint64
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		return meta.FromUint64(0), perrors.WithCode(code.ErrInvalidArgument, "invalid accountId format")
	}

	return meta.FromUint64(id), nil
}

func toAccountResponse(result *appAccount.AccountResult) resp.Account {
	appIDStr := string(result.AppID)
	return resp.Account{
		ID:         result.AccountID.String(),
		UserID:     result.UserID.String(),
		Provider:   string(result.Type),
		ExternalID: string(result.ExternalID),
		AppID:      &appIDStr,
		Status:     result.Status.String(),
	}
}

func registerWechatRequestFromHTTP(reqBody req.RegisterWeChatAccountReq) (appRegister.RegisterRequest, error) {
	var phone meta.Phone
	if strings.TrimSpace(reqBody.Phone) != "" {
		phone, _ = meta.NewPhone(strings.TrimSpace(reqBody.Phone))
	}
	var email meta.Email
	if strings.TrimSpace(reqBody.Email) != "" {
		email, _ = meta.NewEmail(strings.TrimSpace(reqBody.Email))
	}

	appID := strings.TrimSpace(reqBody.AppID)
	jsCode := strings.TrimSpace(reqBody.JsCode)

	profile := make(map[string]string)
	if reqBody.Nickname != nil && *reqBody.Nickname != "" {
		profile["nickname"] = *reqBody.Nickname
	}
	if reqBody.Avatar != nil && *reqBody.Avatar != "" {
		profile["avatar"] = *reqBody.Avatar
	}

	metaMap, err := reqBody.MetaJSON()
	if err != nil {
		return appRegister.RegisterRequest{}, err
	}

	return appRegister.RegisterRequest{
		Name:           strings.TrimSpace(reqBody.Name),
		Phone:          phone,
		Email:          email,
		AccountType:    domain.TypeWcMinip,
		CredentialType: appRegister.CredTypeWechat,
		WechatAppID:    &appID,
		WechatJsCode:   &jsCode,
		Profile:        profile,
		Meta:           metaMap,
	}, nil
}

func registerResultToResponse(result *appRegister.RegisterResult) resp.RegisterResult {
	return resp.RegisterResult{
		UserID:       result.UserID.Uint64(),
		UserName:     result.UserName,
		Phone:        result.Phone.String(),
		Email:        result.Email.String(),
		AccountID:    result.AccountID.Uint64(),
		AccountType:  string(result.AccountType),
		ExternalID:   string(result.ExternalID),
		CredentialID: result.CredentialID.Uint64(),
		IsNewUser:    result.IsNewUser,
		IsNewAccount: result.IsNewAccount,
	}
}

func profileFromUpsertRequest(reqBody req.UpsertWeChatProfileReq) map[string]string {
	profile := make(map[string]string)
	if reqBody.Nickname != nil && *reqBody.Nickname != "" {
		profile["nickname"] = *reqBody.Nickname
	}
	if reqBody.Avatar != nil && *reqBody.Avatar != "" {
		profile["avatar"] = *reqBody.Avatar
	}
	if reqBody.Meta != nil {
		for k, v := range reqBody.Meta {
			if str, ok := v.(string); ok {
				profile[k] = str
			}
		}
	}
	return profile
}
