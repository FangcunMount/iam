package session

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	tokenapp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/token"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

// Renewal 编排会话续期（refresh token 轮换）。
type Renewal struct {
	refresher tokenapp.Refresher
}

// Execute 使用 refresh token 续期并返回新的 token pair。
func (r *Renewal) Execute(ctx context.Context, refreshToken string) (*RenewResult, error) {
	if err := r.ensureReady(); err != nil {
		return nil, err
	}

	refreshToken, err := validateRenewRefreshToken(refreshToken)
	if err != nil {
		return nil, err
	}

	return r.refresher.RefreshToken(ctx, refreshToken)
}

func (r *Renewal) ensureReady() error {
	if r == nil || r.refresher == nil {
		return perrors.WithCode(code.ErrInvalidArgument, "token refresher is not initialized")
	}
	return nil
}

func validateRenewRefreshToken(refreshToken string) (string, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return "", perrors.WithCode(code.ErrInvalidArgument, "refresh token is required")
	}
	return refreshToken, nil
}
