// Package code defines shared error codes used across the iam services.
//
// 错误码按照"通用能力 + 业务模块"进行分层管理，方便在系统中快速定位并统一处理。
// 每个文件代表一个模块或域的错误码定义：
//
//   - base.go        ：平台级非业务错误码（如绑定、校验、数据库、编码等），范围：100001～100999
//   - identity.go    ：基础用户及身份档案/监护等领域错误码，范围：101000～101999
//   - authn.go       ：认证（Authentication）相关所有错误码（包含 JWKS），范围：102000～102999
//   - authz.go       ：授权（Authorization）相关所有错误码，范围：103000～103999
//   - 其他错误码     ：预留范围：104000～104999
//
// 约定：
//  1. 错误码统一通过 pkg/errors.WithCode / WrapC 产出，确保能够被统一解析。
//  2. 不同模块的错误码区间互不重叠，便于排查（详见各文件中的常量定义）。
//  3. 错误码命名遵循 Err + 模块 + 问题描述 的形式，例如 ErrUserNotFound。
//
// 使用示例：
//
//	return errors.WithCode(code.ErrUserNotFound, "user(%s) not found", userID)
package code
