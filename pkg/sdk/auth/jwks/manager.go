package jwks

import (
	"context"

	"github.com/FangcunMount/iam/v2/pkg/sdk/config"
	"github.com/lestrrat-go/jwx/v2/jwk"
)

// JWKSManager JWKS 密钥管理器（使用职责链模式）。
type JWKSManager struct {
	config *config.JWKSConfig
	chain  KeyFetcher
	cache  *CacheFetcher

	stopCh chan struct{}
}

// GetKeySet 获取当前密钥集。
func (m *JWKSManager) GetKeySet(ctx context.Context) (jwk.Set, error) {
	return m.chain.Fetch(ctx)
}

// ForceRefresh 强制刷新（绕过缓存）。
func (m *JWKSManager) ForceRefresh(ctx context.Context) error {
	if m.cache != nil && m.cache.next != nil {
		keySet, err := m.cache.next.Fetch(ctx)
		if err != nil {
			return err
		}
		m.cache.Update(keySet)
		return nil
	}
	_, err := m.chain.Fetch(ctx)
	return err
}

// Stop 停止后台刷新。
func (m *JWKSManager) Stop() {
	close(m.stopCh)
}
