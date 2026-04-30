package keyset

import appjwks "github.com/FangcunMount/iam/internal/apiserver/application/authn/jwks"

func toAppKeyStatus(status KeyStatus) appjwks.KeyStatus {
	return appjwks.KeyStatus(status)
}

func fromAppKeyStatus(status appjwks.KeyStatus) KeyStatus {
	return KeyStatus(status)
}

func toAppPublicJWK(jwk PublicJWK) appjwks.PublicJWK {
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

// fromAppPublicJWK 将应用层的 PublicJWK 转换为领域层的 PublicJWK
// func fromAppPublicJWK(jwk appjwks.PublicJWK) PublicJWK {
// 	return PublicJWK{
// 		Kty: jwk.Kty,
// 		Use: jwk.Use,
// 		Alg: jwk.Alg,
// 		Kid: jwk.Kid,
// 		N:   jwk.N,
// 		E:   jwk.E,
// 		Crv: jwk.Crv,
// 		X:   jwk.X,
// 		Y:   jwk.Y,
// 	}
// }

func toAppManagedKey(key *Key) *appjwks.ManagedKey {
	if key == nil {
		return nil
	}
	return &appjwks.ManagedKey{
		Kid:       key.Kid,
		Status:    toAppKeyStatus(key.Status),
		JWK:       toAppPublicJWK(key.JWK),
		NotBefore: key.NotBefore,
		NotAfter:  key.NotAfter,
		CreatedAt: key.CreatedAt,
		UpdatedAt: key.UpdatedAt,
	}
}

func toAppManagedKeys(keys []*Key) []*appjwks.ManagedKey {
	if len(keys) == 0 {
		return nil
	}
	out := make([]*appjwks.ManagedKey, 0, len(keys))
	for _, key := range keys {
		out = append(out, toAppManagedKey(key))
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

func toAppRotationPolicy(policy RotationPolicy) appjwks.RotationPolicy {
	return appjwks.RotationPolicy{
		RotationInterval: policy.RotationInterval,
		GracePeriod:      policy.GracePeriod,
		MaxKeysInJWKS:    policy.MaxKeysInJWKS,
	}
}

func fromAppRotationPolicy(policy appjwks.RotationPolicy) RotationPolicy {
	return RotationPolicy{
		RotationInterval: policy.RotationInterval,
		GracePeriod:      policy.GracePeriod,
		MaxKeysInJWKS:    policy.MaxKeysInJWKS,
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
