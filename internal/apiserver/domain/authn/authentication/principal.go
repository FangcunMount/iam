package authentication

import "github.com/FangcunMount/iam/v2/internal/pkg/meta"

func buildLoginIdentityPrincipal(
	loginIdentityID meta.ID,
	userID meta.ID,
	tenantID meta.ID,
	authMethod string,
	realm string,
	amr []string,
	claims map[string]any,
) *Principal {
	if claims == nil {
		claims = map[string]any{}
	}
	claims["login_identity_id"] = loginIdentityID.String()
	claims["auth_method"] = authMethod
	claims["realm"] = realm
	return &Principal{
		LoginIdentityID: loginIdentityID,
		UserID:          userID,
		TenantID:        tenantID,
		AuthMethod:      authMethod,
		Realm:           realm,
		AMR:             amr,
		Claims:          claims,
	}
}
