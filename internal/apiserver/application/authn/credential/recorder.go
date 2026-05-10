package credential

import (
	"context"
	"time"

	credDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/credential"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// Recorder 记录长期 Credential 的认证生命周期状态。
type Recorder interface {
	Record(ctx context.Context, decision authentication.AuthDecision) error
}

type Dependencies struct {
	Credentials   credDomain.Repository
	LockoutPolicy credDomain.LockoutPolicy
	Now           func() time.Time
}

type recorder struct {
	deps Dependencies
}

func NewRecorder(deps Dependencies) Recorder {
	return &recorder{deps: deps}
}

func (r *recorder) Record(ctx context.Context, decision authentication.AuthDecision) error {
	if r == nil || r.deps.Credentials == nil || decision.CredentialID.IsZero() {
		return nil
	}
	cred, err := r.deps.Credentials.GetByID(ctx, decision.CredentialID)
	if err != nil || cred == nil {
		return err
	}
	now := r.now()
	if decision.OK {
		if err := r.rotateMaterial(ctx, cred, decision); err != nil {
			return err
		}
		cred.RecordSuccess(now)
		return r.deps.Credentials.UpdateAuthState(ctx, cred)
	}
	if decision.Code != code.ErrInvalidCredentials {
		return nil
	}
	cred.RecordFailure(now)
	cred.ApplyLockPolicy(now, r.deps.LockoutPolicy)
	return r.deps.Credentials.UpdateAuthState(ctx, cred)
}

func (r *recorder) rotateMaterial(ctx context.Context, cred *credDomain.Credential, decision authentication.AuthDecision) error {
	if !decision.ShouldRotate || len(decision.NewMaterial) == 0 {
		return nil
	}
	algo := ""
	if cred.Algo != nil {
		algo = *cred.Algo
	}
	if decision.NewAlgo != nil {
		algo = *decision.NewAlgo
	}
	return r.deps.Credentials.UpdateMaterial(ctx, cred.ID, decision.NewMaterial, algo)
}

func (r *recorder) now() time.Time {
	if r.deps.Now != nil {
		return r.deps.Now()
	}
	return time.Now()
}
