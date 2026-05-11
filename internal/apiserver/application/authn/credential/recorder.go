package credential

import (
	"context"
	"time"

	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	credDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/credential"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// Recorder 记录长期 Credential 的认证生命周期状态。
type Recorder interface {
	Record(ctx context.Context, decision authentication.AuthDecision) error
}

// Dependencies 记录长期 Credential 的认证生命周期状态的依赖。
type Dependencies struct {
	Credentials   credDomain.Repository
	LockoutPolicy credDomain.LockoutPolicy
	Now           func() time.Time
}

// recorder 记录长期 Credential 的认证生命周期状态。
type recorder struct {
	deps Dependencies
}

// 确保 recorder 实现了 Recorder 接口。
var _ Recorder = (*recorder)(nil)

// NewRecorder 创建记录长期 Credential 的认证生命周期状态的 recorder。
func NewRecorder(deps Dependencies) Recorder {
	return &recorder{deps: deps}
}

// Record 记录长期 Credential 的认证生命周期状态。
func (r *recorder) Record(ctx context.Context, decision authentication.AuthDecision) error {
	if r == nil || r.deps.Credentials == nil || decision.CredentialID.IsZero() {
		return nil
	}
	// 获取凭据。
	cred, err := r.deps.Credentials.GetByID(ctx, decision.CredentialID)
	if err != nil {
		return err
	}

	// 凭据不存在，不需要记录认证生命周期状态，直接返回。
	if cred == nil {
		return nil
	}

	switch decision.Code {
	case code.ErrInvalidCredentials, code.ErrAuthenticationFailed:
		// 认证失败（含密码错误等），记录失败次数与最近失败时间。
		return r.recordFailure(ctx, cred, r.now())
	default:
		if !decision.OK {
			return nil
		}
		// 认证成功，记录认证成功状态。
		return r.recordSuccess(ctx, cred, decision, r.now())
	}
}

// recordSuccess 记录认证成功状态。
func (r *recorder) recordSuccess(ctx context.Context, cred *credDomain.Credential, decision authentication.AuthDecision, now time.Time) error {
	// 轮换凭据材料。
	if err := r.rotateMaterial(ctx, cred, decision); err != nil {
		return err
	}
	// 记录认证成功状态。
	cred.RecordSuccess(now)
	// 更新凭据认证状态。
	return r.deps.Credentials.UpdateAuthState(ctx, cred)
}

// recordFailure 记录认证失败状态。
func (r *recorder) recordFailure(ctx context.Context, cred *credDomain.Credential, now time.Time) error {
	// 记录认证失败状态。
	cred.RecordFailure(now)

	// 更新凭据认证状态。
	return r.deps.Credentials.UpdateAuthState(ctx, cred)
}

// rotateMaterial 轮换凭据材料。
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

// now 获取当前时间。
func (r *recorder) now() time.Time {
	if r.deps.Now != nil {
		return r.deps.Now()
	}
	return time.Now()
}
