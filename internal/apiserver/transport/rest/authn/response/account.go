package response

import (
	"time"

	domain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/account"
)

// Account response DTO.
type Account struct {
	ID         string  `json:"id"`
	UserID     string  `json:"userId"`
	Provider   string  `json:"provider"`
	ExternalID string  `json:"externalId"`
	AppID      *string `json:"appId,omitempty"`
	Status     string  `json:"status"`
}

// OperationCredential response DTO (without hash).
type OperationCredential struct {
	AccountID      string     `json:"accountId"`
	Username       string     `json:"username"`
	LockedUntil    *time.Time `json:"lockedUntil,omitempty"`
	FailedAttempts int        `json:"failedAttempts"`
	LastChangedAt  time.Time  `json:"lastChangedAt"`
}

// OperationCredentialView bundles account + credential.
type OperationCredentialView struct {
	Account    Account             `json:"account"`
	Credential OperationCredential `json:"credential"`
}

// BindResult response.
type BindResult struct {
	AccountID string `json:"accountId"`
	Created   bool   `json:"created"`
}

// SignupResult 账号开通结果响应。
type SignupResult struct {
	UserID       uint64 `json:"userId"`
	UserName     string `json:"userName"`
	Phone        string `json:"phone"`
	Email        string `json:"email,omitempty"`
	AccountID    uint64 `json:"accountId"`
	AccountType  string `json:"accountType"`
	ExternalID   string `json:"externalId"`
	CredentialID uint64 `json:"credentialID"`
	IsNewUser    bool   `json:"isNewUser"`
	IsNewAccount bool   `json:"isNewAccount"`
}

// EnsureMockConsumerResult 内部 mock C 端 ensure 结果。
type EnsureMockConsumerResult struct {
	UserID       string `json:"user_id"`
	AccountID    string `json:"account_id"`
	LoginID      string `json:"login_id"`
	IsNewUser    bool   `json:"is_new_user"`
	IsNewAccount bool   `json:"is_new_account"`
}

// CredentialList 凭据列表响应
type CredentialList struct {
	Total int          `json:"total"`
	Items []Credential `json:"items"`
}

// Credential 凭据响应
type Credential struct {
	ID            uint64 `json:"id"`
	AccountID     uint64 `json:"accountId"`
	Type          string `json:"type"`
	IDP           string `json:"idp,omitempty"`
	IDPIdentifier string `json:"idpIdentifier"`
	AppID         string `json:"appId,omitempty"`
	Status        string `json:"status"`
}

// AccountPage paginated listing.
type AccountPage struct {
	Total  int       `json:"total"`
	Limit  int       `json:"limit"`
	Offset int       `json:"offset"`
	Items  []Account `json:"items"`
}

// NewAccount from domain model.
func NewAccount(acc *domain.Account) Account {
	if acc == nil {
		return Account{}
	}
	resp := Account{
		ID:         acc.ID.String(),
		UserID:     acc.UserID.String(),
		Provider:   acc.Type.String(),
		ExternalID: string(acc.ExternalID),
		Status:     statusToString(acc.Status),
	}
	if acc.AppID.Len() > 0 {
		appID := string(acc.AppID)
		resp.AppID = &appID
	}
	return resp
}

// NewBindResult builds bind result.
func NewBindResult(accountID string, created bool) BindResult {
	return BindResult{
		AccountID: accountID,
		Created:   created,
	}
}

// NewAccountPage builds paginated response.
func NewAccountPage(total, limit, offset int, items []*domain.Account) AccountPage {
	resp := AccountPage{
		Total:  total,
		Limit:  limit,
		Offset: offset,
		Items:  make([]Account, 0, len(items)),
	}
	for _, item := range items {
		resp.Items = append(resp.Items, NewAccount(item))
	}
	return resp
}

func statusToString(status domain.AccountStatus) string {
	switch status {
	case domain.StatusDisabled:
		return "disabled"
	case domain.StatusActive:
		return "active"
	case domain.StatusArchived:
		return "archived"
	case domain.StatusDeleted:
		return "deleted"
	default:
		return "unknown"
	}
}
