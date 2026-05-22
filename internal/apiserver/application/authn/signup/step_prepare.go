package signup

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// prepareStep 准备步骤，用于准备登录身份和凭据。
type prepareStep struct {
	wechatIdentityResolver *wechatIdentityResolver
}

// newPrepareStep 创建准备步骤。
func newPrepareStep(wechatIdentityResolver *wechatIdentityResolver) *prepareStep {
	return &prepareStep{wechatIdentityResolver: wechatIdentityResolver}
}

// Run 在事务外裁剪输入、解析外部身份，生成后续步骤需要的 preparedSignup。
// 参数：
//   - ctx: 上下文
//   - req: 登录请求
//
// 返回：
//   - *preparedSignup: 准备后的登录身份和凭据
//   - error: 错误
func (s *prepareStep) Run(ctx context.Context, req SignupRequest) (*preparedSignup, error) {
	user := trimUserInput(req.User)
	credential := trimCredentialInput(req.Credential)

	if req.LoginIdentity == nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "login identity input is required")
	}

	identity, err := req.LoginIdentity.prepareSignupLoginIdentity(ctx, s.prepareDeps(), user)
	if err != nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "failed to prepare login identity: %v", err)
	}

	return &preparedSignup{
		User:          user,
		LoginIdentity: identity,
		Credential:    credential,
	}, nil
}

// prepareDeps 准备依赖。
func (s *prepareStep) prepareDeps() loginIdentityPrepareDeps {
	if s == nil {
		return loginIdentityPrepareDeps{}
	}
	return loginIdentityPrepareDeps{wechatIdentityResolver: s.wechatIdentityResolver}
}

// trimUserInput 修剪用户输入。
func trimUserInput(user SignupUserInput) SignupUserInput {
	user.Name = strings.TrimSpace(user.Name)
	return user
}

// trimCredentialInput 修剪凭据输入。
func trimCredentialInput(in *SignupCredentialInput) *SignupCredentialInput {
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

// valueOfStringPtr 获取字符串指针的值。
func valueOfStringPtr(v *string) string {
	if v == nil {
		return ""
	}
	return *v
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
