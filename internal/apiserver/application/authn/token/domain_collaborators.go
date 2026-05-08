package token

import sessiondomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/session"

// SessionManager 是 token 用例依赖的会话领域协作者。
type SessionManager = sessiondomain.Manager

// SubjectAccessEvaluator 是 token 用例依赖的主体访问状态领域协作者。
type SubjectAccessEvaluator = sessiondomain.SubjectAccessEvaluator
