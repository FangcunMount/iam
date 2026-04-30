package authentication

import "github.com/FangcunMount/iam/internal/apiserver/domain/authn/credential"

type AuthCredential interface {
	CredentialType() credential.CredentialType
}
