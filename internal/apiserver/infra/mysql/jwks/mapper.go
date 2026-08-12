package jwks

import (
	"encoding/json"

	jwks "github.com/FangcunMount/iam/v3/internal/apiserver/infra/token/keyset"
)

// Mapper 负责 Domain Entity 和 PO 之间的转换
type Mapper struct{}

// NewMapper 创建 Mapper 实例
func NewMapper() *Mapper {
	return &Mapper{}
}

// ToKeyPO 将 Domain Entity 转换为 PO
func (m *Mapper) ToKeyPO(key *jwks.Key) (*KeyPO, error) {
	if key == nil {
		return nil, nil
	}

	// 序列化 PublicJWK 为 JSON
	jwkJSON, err := json.Marshal(key.JWK)
	if err != nil {
		return nil, err
	}

	// 从 JWK 中提取字段
	kty := key.JWK.Kty
	use := key.JWK.Use
	alg := key.JWK.Alg

	po := &KeyPO{
		Kid:       key.Kid,
		Status:    int8(key.Status),
		Kty:       kty,
		Use:       use,
		Alg:       alg,
		JwkJSON:   jwkJSON,
		NotBefore: key.NotBefore,
		NotAfter:  key.NotAfter,
	}

	return po, nil
}

// ToKeyEntity 将 PO 转换为 Domain Entity
func (m *Mapper) ToKeyEntity(po *KeyPO) (*jwks.Key, error) {
	if po == nil {
		return nil, nil
	}

	// 反序列化 JWK JSON
	var publicJWK jwks.PublicJWK
	if err := json.Unmarshal(po.JwkJSON, &publicJWK); err != nil {
		return nil, err
	}

	key := &jwks.Key{
		Kid:       po.Kid,
		Status:    jwks.KeyStatus(po.Status),
		JWK:       publicJWK,
		NotBefore: po.NotBefore,
		NotAfter:  po.NotAfter,
		CreatedAt: po.CreatedAt,
		UpdatedAt: po.UpdatedAt,
	}

	return key, nil
}

// ToKeyEntities 批量转换 PO 列表为 Entity 列表
func (m *Mapper) ToKeyEntities(pos []*KeyPO) ([]*jwks.Key, error) {
	if len(pos) == 0 {
		return []*jwks.Key{}, nil
	}

	entities := make([]*jwks.Key, 0, len(pos))
	for _, po := range pos {
		entity, err := m.ToKeyEntity(po)
		if err != nil {
			return nil, err
		}
		if entity != nil {
			entities = append(entities, entity)
		}
	}

	return entities, nil
}
