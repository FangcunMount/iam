package keyset

import (
	appjwks "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/jwks"
	appsigningkey "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/signingkey"
)

func keyStatusFromString(status string) KeyStatus {
	switch status {
	case "active":
		return KeyActive
	case "grace":
		return KeyGrace
	case "retired":
		return KeyRetired
	default:
		return 0
	}
}

func toJWKSPublicJWK(jwk PublicJWK) appjwks.PublicJWK {
	return appjwks.PublicJWK{
		Kty: jwk.Kty,
		Use: jwk.Use,
		Alg: jwk.Alg,
		Kid: jwk.Kid,
		N:   jwk.N,
		E:   jwk.E,
		Crv: jwk.Crv,
		X:   jwk.X,
		Y:   jwk.Y,
	}
}

func toSigningKeyPublicJWK(jwk PublicJWK) appsigningkey.PublicJWK {
	return appsigningkey.PublicJWK{
		Kty: jwk.Kty,
		Use: jwk.Use,
		Alg: jwk.Alg,
		Kid: jwk.Kid,
		N:   jwk.N,
		E:   jwk.E,
		Crv: jwk.Crv,
		X:   jwk.X,
		Y:   jwk.Y,
	}
}

func toSigningKeyManagedKey(key *Key) *appsigningkey.ManagedKey {
	if key == nil {
		return nil
	}
	return &appsigningkey.ManagedKey{
		Kid:       key.Kid,
		Algorithm: key.Algorithm,
		Status:    key.Status.String(),
		JWK:       toSigningKeyPublicJWK(key.JWK),
		NotBefore: key.NotBefore,
		NotAfter:  key.NotAfter,
		CreatedAt: key.CreatedAt,
		UpdatedAt: key.UpdatedAt,
	}
}

func toSigningKeyManagedKeys(keys []*Key) []*appsigningkey.ManagedKey {
	if len(keys) == 0 {
		return nil
	}
	out := make([]*appsigningkey.ManagedKey, 0, len(keys))
	for _, key := range keys {
		out = append(out, toSigningKeyManagedKey(key))
	}
	return out
}

func toAppPublishableKeys(keys []*Key) []*appjwks.PublishableKey {
	if len(keys) == 0 {
		return nil
	}
	out := make([]*appjwks.PublishableKey, 0, len(keys))
	for _, key := range keys {
		if key == nil {
			continue
		}
		out = append(out, &appjwks.PublishableKey{
			Kid:       key.Kid,
			Status:    key.Status.String(),
			JWK:       toJWKSPublicJWK(key.JWK),
			NotBefore: key.NotBefore,
			NotAfter:  key.NotAfter,
			CreatedAt: key.CreatedAt,
			UpdatedAt: key.UpdatedAt,
		})
	}
	return out
}

func toAppCacheTag(tag CacheTag) appjwks.CacheTag {
	return appjwks.CacheTag{
		ETag:         tag.ETag,
		LastModified: tag.LastModified,
	}
}

func fromAppCacheTag(tag appjwks.CacheTag) CacheTag {
	return CacheTag{
		ETag:         tag.ETag,
		LastModified: tag.LastModified,
	}
}

func toAppSnapshotStatus(status SnapshotStatus) appjwks.SnapshotStatus {
	return appjwks.SnapshotStatus{
		Cached:        status.Cached,
		KeyCount:      status.KeyCount,
		CacheTag:      toAppCacheTag(status.CacheTag),
		LastBuildTime: status.LastBuildTime,
	}
}
