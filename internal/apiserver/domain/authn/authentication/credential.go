package authentication

import "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/credential"

type AuthCredential interface {
	CredentialType() credential.CredentialType
}
