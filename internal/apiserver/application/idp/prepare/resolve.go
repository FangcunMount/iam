package prepare

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"
	idpPort "github.com/FangcunMount/iam/v2/internal/apiserver/domain/idp/wechatapp"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// Provider 标识微信或企业微信应用配置。
type Provider string

const (
	ProviderWechat Provider = "wechat"
	ProviderWecom  Provider = "wecom"
)

// Surface 区分调用方错误码与文案风格。
type Surface string

const (
	SurfaceLinking    Surface = "linking"
	SurfaceLoginProof Surface = "login_proof"
)

// Dependencies 是查密钥所需依赖。
type Dependencies struct {
	Apps  idpPort.Repository
	Vault idpPort.SecretVault
}

// Options 控制错误映射与 login proof 结构化日志。
type Options struct {
	Provider       Provider
	Surface        Surface
	AppID          string
	CredentialKind string
}

// ResolveAppSecret 从应用仓库查询并解密应用密钥。
func ResolveAppSecret(ctx context.Context, deps Dependencies, opts Options) (string, error) {
	appID := strings.TrimSpace(opts.AppID)
	errCode := errorCode(opts.Surface)
	logProof := opts.Surface == SurfaceLoginProof && opts.CredentialKind != ""

	if deps.Apps == nil || deps.Vault == nil {
		return "", serviceUnavailable(ctx, opts, errCode, logProof)
	}

	app, err := deps.Apps.GetByAppID(ctx, appID)
	if err != nil {
		return "", queryFailed(ctx, opts, errCode, logProof, err)
	}
	if app == nil {
		return "", appNotFound(ctx, opts, errCode, logProof)
	}
	if !app.IsEnabled() {
		return "", appDisabled(ctx, opts, errCode, logProof)
	}
	if app.Cred == nil || app.Cred.Auth == nil {
		return "", credentialsMissing(ctx, opts, errCode, logProof)
	}

	plain, err := deps.Vault.Decrypt(ctx, app.Cred.Auth.AppSecretCipher)
	if err != nil {
		return "", decryptFailed(ctx, opts, errCode, logProof, err)
	}
	return string(plain), nil
}

func errorCode(surface Surface) int {
	if surface == SurfaceLinking {
		return code.ErrInvalidArgument
	}
	return code.ErrProofBuildFailed
}

func serviceUnavailable(ctx context.Context, opts Options, errCode int, logProof bool) error {
	if logProof {
		l := logger.L(ctx)
		providerLabel := proofProviderLabel(opts.Provider)
		l.Errorw(providerLabel+"应用配置服务不可用",
			"action", logger.ActionLogin,
			"credential_kind", opts.CredentialKind,
		)
		return perrors.WithCode(errCode, "%s app configuration service not available", opts.Provider)
	}
	return perrors.WithCode(errCode, "%s app configuration service is not available", opts.Provider)
}

func queryFailed(ctx context.Context, opts Options, errCode int, logProof bool, err error) error {
	if logProof {
		l := logger.L(ctx)
		l.Errorw(proofProviderLabel(opts.Provider)+"应用配置查询失败",
			"action", logger.ActionLogin,
			"credential_kind", opts.CredentialKind,
			proofAppIDKey(opts.Provider), opts.AppID,
			"error", err.Error(),
		)
		return proofQueryFailed(errCode, opts.Provider, err)
	}
	return perrors.WithCode(errCode, "failed to query %s app: %v", opts.Provider, err)
}

func appNotFound(ctx context.Context, opts Options, errCode int, logProof bool) error {
	if logProof {
		l := logger.L(ctx)
		l.Warnw(proofProviderLabel(opts.Provider)+"应用不存在",
			"action", logger.ActionLogin,
			"credential_kind", opts.CredentialKind,
			proofAppIDKey(opts.Provider), opts.AppID,
		)
		return proofAppNotFound(errCode, opts.Provider, opts.AppID)
	}
	return perrors.WithCode(errCode, "%s app is not available", opts.Provider)
}

func appDisabled(ctx context.Context, opts Options, errCode int, logProof bool) error {
	if logProof {
		l := logger.L(ctx)
		l.Warnw(proofProviderLabel(opts.Provider)+"应用已禁用",
			"action", logger.ActionLogin,
			"credential_kind", opts.CredentialKind,
			proofAppIDKey(opts.Provider), opts.AppID,
		)
		return proofAppDisabled(errCode, opts.Provider, opts.AppID)
	}
	return perrors.WithCode(errCode, "%s app is not available", opts.Provider)
}

func credentialsMissing(ctx context.Context, opts Options, errCode int, logProof bool) error {
	if logProof {
		l := logger.L(ctx)
		l.Errorw(proofProviderLabel(opts.Provider)+"应用凭据缺失",
			"action", logger.ActionLogin,
			"credential_kind", opts.CredentialKind,
			proofAppIDKey(opts.Provider), opts.AppID,
		)
		return proofCredentialsMissing(errCode, opts.Provider)
	}
	return perrors.WithCode(errCode, "%s app is not available", opts.Provider)
}

func decryptFailed(ctx context.Context, opts Options, errCode int, logProof bool, err error) error {
	if logProof {
		l := logger.L(ctx)
		l.Errorw(proofProviderLabel(opts.Provider)+"应用密钥解密失败",
			"action", logger.ActionLogin,
			"credential_kind", opts.CredentialKind,
			proofAppIDKey(opts.Provider), opts.AppID,
			"error", err.Error(),
		)
		return proofDecryptFailed(errCode, opts.Provider, err)
	}
	return perrors.WithCode(errCode, "failed to decrypt %s app secret: %v", opts.Provider, err)
}

func proofProviderLabel(provider Provider) string {
	switch provider {
	case ProviderWecom:
		return "企业微信"
	default:
		return "微信"
	}
}

func proofAppIDKey(provider Provider) string {
	if provider == ProviderWecom {
		return "corp_id"
	}
	return "app_id"
}

func proofQueryFailed(errCode int, provider Provider, err error) error {
	if provider == ProviderWecom {
		return perrors.WithCode(errCode, "failed to query wecom app: %v", err)
	}
	return perrors.WithCode(errCode, "failed to query wechat app: %v", err)
}

func proofAppNotFound(errCode int, provider Provider, appID string) error {
	if provider == ProviderWecom {
		return perrors.WithCode(errCode, "wecom app not found: %s", appID)
	}
	return perrors.WithCode(errCode, "wechat app not found: %s", appID)
}

func proofAppDisabled(errCode int, provider Provider, appID string) error {
	if provider == ProviderWecom {
		return perrors.WithCode(errCode, "wecom app is disabled: %s", appID)
	}
	return perrors.WithCode(errCode, "wechat app is disabled: %s", appID)
}

func proofCredentialsMissing(errCode int, provider Provider) error {
	if provider == ProviderWecom {
		return perrors.WithCode(errCode, "wecom app credentials not found")
	}
	return perrors.WithCode(errCode, "wechat app credentials not found")
}

func proofDecryptFailed(errCode int, provider Provider, err error) error {
	if provider == ProviderWecom {
		return perrors.WithCode(errCode, "failed to decrypt wecom corp secret: %v", err)
	}
	return perrors.WithCode(errCode, "failed to decrypt app secret: %v", err)
}
