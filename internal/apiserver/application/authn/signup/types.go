package signup

import (
	credDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/credential"
	loginidentityDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/loginidentity"
	userDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/user"
)

// preparedSignup 各步骤共享的准备后数据（包内流水线状态）。
type preparedSignup struct {
	User          SignupUserInput
	LoginIdentity preparedLoginIdentity
	Credential    *SignupCredentialInput
}

// registrationRepositories 事务内仓储聚合。
type registrationRepositories struct {
	Users           userDomain.Repository
	Credentials     credDomain.Repository
	LoginIdentities loginidentityDomain.Repository
}

// signupExecutionResult 事务内各步骤产出。
type signupExecutionResult struct {
	User          *resolveUserStepResult
	LoginIdentity *ensureLoginIdentityStepResult
	Credential    *ensureCredentialStepResult
}

// 步骤结果类型别名（兼容测试命名）。
type (
	UserResolveResult         = resolveUserStepResult
	LoginIdentityEnsureResult = ensureLoginIdentityStepResult
	CredentialEnsureResult    = ensureCredentialStepResult
)
