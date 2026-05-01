package login

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"

	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

type explicitMethodSelector struct {
	catalog *SignInAdapterCatalog
}

func (s explicitMethodSelector) Select(_ context.Context, req SignInCommand) (SignInAttempt, error) {
	common := commonPayloadFromRequest(req)
	adapter, ok := s.catalog.findAuthType(req.AuthType)
	if !ok {
		return SignInAttempt{}, perrors.WithCode(code.ErrInvalidArgument, "unsupported authentication method: %s", req.AuthType)
	}
	payload, err := adapter.BuildExplicit(req, common)
	if err != nil {
		return SignInAttempt{}, err
	}
	return SignInAttempt{
		Method:  adapter.Kind(),
		Adapter: adapter,
		Payload: payload,
	}, nil
}
