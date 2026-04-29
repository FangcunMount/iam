package suggest

import (
	"context"

	"github.com/FangcunMount/iam/internal/apiserver/domain/suggest"
)

// Service 提供 suggest 查询
type Service struct {
	cfg     Config
	runtime IndexRuntime
}

// NewService 创建 Service
func NewService(cfg Config) *Service {
	return NewServiceWithRuntime(cfg, nil)
}

// NewServiceWithRuntime creates a suggest service with an explicit index runtime.
func NewServiceWithRuntime(cfg Config, runtime IndexRuntime) *Service {
	if cfg.MaxResults == 0 {
		cfg.MaxResults = 20
	}
	if cfg.KeyPadLen == 0 {
		cfg.KeyPadLen = 25
	}
	return &Service{cfg: cfg, runtime: runtime}
}

// Suggest 查询
func (s *Service) Suggest(_ context.Context, keyword string) []suggest.Term {
	if s == nil || s.runtime == nil {
		return nil
	}
	index := s.runtime.Current()
	if index == nil {
		return nil
	}
	return index.Suggest(keyword, s.cfg.MaxResults, s.cfg.KeyPadLen)
}
