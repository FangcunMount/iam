package suggest

import (
	"context"

	"github.com/FangcunMount/iam/internal/apiserver/domain/suggest"
	"github.com/FangcunMount/iam/internal/apiserver/infra/suggest/search"
)

// Service 提供 suggest 查询
type Service struct {
	cfg Config
}

// NewService 创建 Service
func NewService(cfg Config) *Service {
	if cfg.MaxResults == 0 {
		cfg.MaxResults = 20
	}
	if cfg.KeyPadLen == 0 {
		cfg.KeyPadLen = 25
	}
	return &Service{cfg: cfg}
}

// Suggest 查询
func (s *Service) Suggest(_ context.Context, keyword string) []suggest.Term {
	store := search.Current()
	if store == nil {
		return nil
	}
	return store.Suggest(keyword, s.cfg.MaxResults, s.cfg.KeyPadLen)
}
