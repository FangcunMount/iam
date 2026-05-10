package onboarding

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// preparedOnboarding 准备好的开通数据。
type preparedOnboarding struct {
	User          OnboardingUserInput
	LoginIdentity preparedLoginIdentity
	Credential    *OnboardingCredentialInput
}

// requestPreparer 请求准备器。
type requestPreparer struct {
	wechatIdentityResolver *wechatIdentityResolver
}

// newRequestPreparer 创建请求准备器。
func newRequestPreparer(wechatIdentityResolver *wechatIdentityResolver) *requestPreparer {
	return &requestPreparer{wechatIdentityResolver: wechatIdentityResolver}
}

// Prepare 在数据库事务外裁剪输入、解析外部身份，并生成后续固定流程需要的创建数据。
func (p *requestPreparer) Prepare(ctx context.Context, req OnboardingRequest) (*preparedOnboarding, error) {
	// 修剪用户输入。
	user := trimUserInput(req.User)

	// 修剪凭据输入。
	credential := trimCredentialInput(req.Credential)

	// 验证登录身份输入是否为空。
	if req.LoginIdentity == nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "login identity input is required")
	}

	// 准备登录身份。
	identity, err := req.LoginIdentity.prepareOnboardingLoginIdentity(ctx, p.prepareDeps(), user)
	if err != nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "failed to prepare login identity: %v", err)
	}

	// 返回准备好的开通数据。
	return &preparedOnboarding{
		User:          user,
		LoginIdentity: identity,
		Credential:    credential,
	}, nil
}

// prepareDeps 准备依赖。
func (p *requestPreparer) prepareDeps() loginIdentityPrepareDeps {
	if p == nil {
		return loginIdentityPrepareDeps{}
	}
	return loginIdentityPrepareDeps{wechatIdentityResolver: p.wechatIdentityResolver}
}

// trimUserInput 修剪用户输入。
func trimUserInput(user OnboardingUserInput) OnboardingUserInput {
	user.Name = strings.TrimSpace(user.Name)
	return user
}

// trimCredentialInput 修剪凭据输入。
func trimCredentialInput(in *OnboardingCredentialInput) *OnboardingCredentialInput {
	if in == nil {
		return nil
	}
	out := *in
	if in.Password != nil {
		password := *in.Password
		password.Plaintext = strings.TrimSpace(password.Plaintext)
		out.Password = &password
	}
	return &out
}

// trimStringPtr 修剪字符串指针。
func trimStringPtr(v *string) *string {
	if v == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*v)
	return &trimmed
}

// cloneStringMap 克隆字符串映射。
func cloneStringMap(src map[string]string) map[string]string {
	if src == nil {
		return nil
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
