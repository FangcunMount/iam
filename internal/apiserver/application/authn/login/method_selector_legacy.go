package login

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"

	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

type legacyMethodSelector struct {
	catalog *SignInAdapterCatalog
}

func (s legacyMethodSelector) Select(ctx context.Context, req SignInCommand) (SignInAttempt, error) {
	l := logger.L(ctx)
	common := commonPayloadFromRequest(req)
	var selected SignInAttempt

	for _, adapter := range s.catalog.adapters() {
		if payload, ok := adapter.TryLegacy(req, common); ok {
			selected = SignInAttempt{
				Method:  adapter.Kind(),
				Adapter: adapter,
				Payload: payload,
			}
			l.Debugw("检测到登录方式",
				"action", logger.ActionLogin,
				"scenario", string(selected.Method),
			)
		}
	}

	if selected.Method == "" {
		l.Warnw("未提供有效的认证凭据",
			"action", logger.ActionLogin,
			"result", logger.ResultFailed,
		)
		return SignInAttempt{}, perrors.WithCode(code.ErrInvalidArgument, "no valid authentication credentials provided")
	}

	return selected, nil
}
