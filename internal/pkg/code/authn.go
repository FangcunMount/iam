package code

import (
	"net/http"

	"github.com/FangcunMount/component-base/pkg/errors"
)

// Authn: 认证相关所有错误码 (102000～102999).

// Authn: 基础认证错误 (102000～102099).
const (
	// ErrUnauthenticated - 401: Authentication failed.
	ErrUnauthenticated = 102000

	// ErrInvalidCredentials - 401: Invalid credentials.
	ErrInvalidCredentials = 102001

	// ErrTokenInvalid - 401: Token invalid.
	ErrTokenInvalid = 102002

	// ErrEncrypt - 401: Error occurred while encrypting the user password.
	ErrEncrypt = 102003

	// ErrSignatureInvalid - 401: Signature is invalid.
	ErrSignatureInvalid = 102004

	// ErrExpired - 401: Token expired.
	ErrExpired = 102005

	// ErrInvalidAuthHeader - 401: Invalid authorization header.
	ErrInvalidAuthHeader = 102006

	// ErrMissingHeader - 401: The `Authorization` header was empty.
	ErrMissingHeader = 102007

	// ErrPasswordIncorrect - 401: Password was incorrect.
	ErrPasswordIncorrect = 102008

	// ErrUserNotRegistered - 401: User not registered.
	ErrUserNotRegistered = 102009
)

// Authn: JWKS 密钥管理相关错误 (102100～102199).
const (
	// ErrInvalidKid - 400: Invalid kid: kid cannot be empty.
	ErrInvalidKid = 102100

	// ErrInvalidJWK - 400: Invalid JWK: kty cannot be empty.
	ErrInvalidJWK = 102101

	// ErrInvalidJWKUse - 400: Invalid JWK: use must be 'sig'.
	ErrInvalidJWKUse = 102102

	// ErrInvalidJWKAlg - 400: Invalid JWK: alg cannot be empty.
	ErrInvalidJWKAlg = 102103

	// ErrKidMismatch - 400: Kid mismatch: key.Kid and JWK.Kid must be equal.
	ErrKidMismatch = 102104

	// ErrUnsupportedKty - 400: Unsupported key type.
	ErrUnsupportedKty = 102105

	// ErrMissingRSAParams - 400: Missing RSA parameters: n and e are required.
	ErrMissingRSAParams = 102106

	// ErrMissingECParams - 400: Missing EC parameters: crv, x, y are required.
	ErrMissingECParams = 102107

	// ErrMissingOKPParams - 400: Missing OKP parameters: crv, x are required.
	ErrMissingOKPParams = 102108

	// ErrInvalidStateTransition - 400: Invalid key state transition.
	ErrInvalidStateTransition = 102109

	// ErrInvalidTimeRange - 400: Invalid time range: NotAfter must be after NotBefore.
	ErrInvalidTimeRange = 102110

	// ErrEmptyJWKS - 400: JWKS cannot be empty.
	ErrEmptyJWKS = 102111

	// ErrInvalidRotationInterval - 400: Rotation interval must be positive.
	ErrInvalidRotationInterval = 102112

	// ErrInvalidGracePeriod - 400: Grace period must be positive.
	ErrInvalidGracePeriod = 102113

	// ErrInvalidMaxKeys - 400: Max keys must be at least 2.
	ErrInvalidMaxKeys = 102114

	// ErrGracePeriodTooLong - 400: Grace period must be shorter than rotation interval.
	ErrGracePeriodTooLong = 102115

	// ErrKeyNotFound - 404: Key not found.
	ErrKeyNotFound = 102116

	// ErrNoActiveKey - 404: No active key available.
	ErrNoActiveKey = 102117

	// ErrKeyAlreadyExists - 409: Key with this kid already exists.
	ErrKeyAlreadyExists = 102118
)

// Authn: 账号相关错误码 (102200～102299).
const (
	ErrAccountExists   = 102200
	ErrExternalExists  = 102201
	ErrNotFoundAccount = 102202
	ErrUniqueIDExists  = 102203
	ErrInvalidUniqueID = 102204
)

// Authn: 凭据相关错误码 (102300～102399).
const (
	ErrCredentialExists    = 102300
	ErrCredentialNotFound  = 102301
	ErrCredentialLocked    = 102302
	ErrCredentialExpired   = 102303
	ErrCredentialDisabled  = 102304
	ErrInvalidCredential   = 102305
	ErrCredentialNotUsable = 102306
)

// Authn: 认证流程相关错误码 (102400～102499).
const (
	ErrAuthenticationFailed = 102400
	ErrOTPInvalid           = 102401
	ErrStateMismatch        = 102402
	ErrIDPExchangeFailed    = 102403
	ErrNoBinding            = 102404
	ErrOTPSendTooFrequent   = 102405
	ErrTokenRevokeFailed    = 102406
)

// nolint: gochecknoinits
func init() {
	registerAuthn()
}

func registerAuthn() {
	// 基础认证错误
	errors.MustRegister(&authnCoder{code: ErrUnauthenticated, status: http.StatusUnauthorized, msg: "Authentication failed"})
	errors.MustRegister(&authnCoder{code: ErrInvalidCredentials, status: http.StatusUnauthorized, msg: "Invalid credentials"})
	errors.MustRegister(&authnCoder{code: ErrTokenInvalid, status: http.StatusUnauthorized, msg: "Token invalid"})
	errors.MustRegister(&authnCoder{code: ErrEncrypt, status: http.StatusUnauthorized, msg: "Error occurred while encrypting the user password"})
	errors.MustRegister(&authnCoder{code: ErrSignatureInvalid, status: http.StatusUnauthorized, msg: "Signature is invalid"})
	errors.MustRegister(&authnCoder{code: ErrExpired, status: http.StatusUnauthorized, msg: "Token expired"})
	errors.MustRegister(&authnCoder{code: ErrInvalidAuthHeader, status: http.StatusUnauthorized, msg: "Invalid authorization header"})
	errors.MustRegister(&authnCoder{code: ErrMissingHeader, status: http.StatusUnauthorized, msg: "The `Authorization` header was empty"})
	errors.MustRegister(&authnCoder{code: ErrPasswordIncorrect, status: http.StatusUnauthorized, msg: "Password was incorrect"})
	errors.MustRegister(&authnCoder{code: ErrUserNotRegistered, status: http.StatusUnauthorized, msg: "User not registered"})

	// JWKS 密钥管理错误
	errors.MustRegister(&authnCoder{code: ErrInvalidKid, status: http.StatusBadRequest, msg: "Invalid kid: kid cannot be empty"})
	errors.MustRegister(&authnCoder{code: ErrInvalidJWK, status: http.StatusBadRequest, msg: "Invalid JWK: kty cannot be empty"})
	errors.MustRegister(&authnCoder{code: ErrInvalidJWKUse, status: http.StatusBadRequest, msg: "Invalid JWK: use must be 'sig'"})
	errors.MustRegister(&authnCoder{code: ErrInvalidJWKAlg, status: http.StatusBadRequest, msg: "Invalid JWK: alg cannot be empty"})
	errors.MustRegister(&authnCoder{code: ErrKidMismatch, status: http.StatusBadRequest, msg: "Kid mismatch: key.Kid and JWK.Kid must be equal"})
	errors.MustRegister(&authnCoder{code: ErrUnsupportedKty, status: http.StatusBadRequest, msg: "Unsupported key type"})
	errors.MustRegister(&authnCoder{code: ErrMissingRSAParams, status: http.StatusBadRequest, msg: "Missing RSA parameters: n and e are required"})
	errors.MustRegister(&authnCoder{code: ErrMissingECParams, status: http.StatusBadRequest, msg: "Missing EC parameters: crv, x, y are required"})
	errors.MustRegister(&authnCoder{code: ErrMissingOKPParams, status: http.StatusBadRequest, msg: "Missing OKP parameters: crv, x are required"})
	errors.MustRegister(&authnCoder{code: ErrInvalidStateTransition, status: http.StatusBadRequest, msg: "Invalid key state transition"})
	errors.MustRegister(&authnCoder{code: ErrInvalidTimeRange, status: http.StatusBadRequest, msg: "Invalid time range: NotAfter must be after NotBefore"})
	errors.MustRegister(&authnCoder{code: ErrEmptyJWKS, status: http.StatusBadRequest, msg: "JWKS cannot be empty"})
	errors.MustRegister(&authnCoder{code: ErrInvalidRotationInterval, status: http.StatusBadRequest, msg: "Rotation interval must be positive"})
	errors.MustRegister(&authnCoder{code: ErrInvalidGracePeriod, status: http.StatusBadRequest, msg: "Grace period must be positive"})
	errors.MustRegister(&authnCoder{code: ErrInvalidMaxKeys, status: http.StatusBadRequest, msg: "Max keys must be at least 2"})
	errors.MustRegister(&authnCoder{code: ErrGracePeriodTooLong, status: http.StatusBadRequest, msg: "Grace period must be shorter than rotation interval"})
	errors.MustRegister(&authnCoder{code: ErrKeyNotFound, status: http.StatusNotFound, msg: "Key not found"})
	errors.MustRegister(&authnCoder{code: ErrNoActiveKey, status: http.StatusNotFound, msg: "No active key available"})
	errors.MustRegister(&authnCoder{code: ErrKeyAlreadyExists, status: http.StatusConflict, msg: "Key with this kid already exists"})

	// Account-related errors
	errors.MustRegister(&authnCoder{code: ErrAccountExists, status: http.StatusConflict, msg: "Account already exists"})
	errors.MustRegister(&authnCoder{code: ErrExternalExists, status: http.StatusConflict, msg: "External ID already exists"})
	errors.MustRegister(&authnCoder{code: ErrNotFoundAccount, status: http.StatusNotFound, msg: "Account not found"})
	errors.MustRegister(&authnCoder{code: ErrUniqueIDExists, status: http.StatusConflict, msg: "UniqueID already exists"})
	errors.MustRegister(&authnCoder{code: ErrInvalidUniqueID, status: http.StatusBadRequest, msg: "Invalid UniqueID"})

	// Credential-related errors
	errors.MustRegister(&authnCoder{code: ErrCredentialExists, status: http.StatusConflict, msg: "Credential already exists"})
	errors.MustRegister(&authnCoder{code: ErrCredentialNotFound, status: http.StatusNotFound, msg: "Credential not found"})
	errors.MustRegister(&authnCoder{code: ErrCredentialLocked, status: http.StatusLocked, msg: "Credential is locked"})
	errors.MustRegister(&authnCoder{code: ErrCredentialExpired, status: http.StatusUnauthorized, msg: "Credential has expired"})
	errors.MustRegister(&authnCoder{code: ErrCredentialDisabled, status: http.StatusForbidden, msg: "Credential is disabled"})
	errors.MustRegister(&authnCoder{code: ErrInvalidCredential, status: http.StatusBadRequest, msg: "Invalid credential"})
	errors.MustRegister(&authnCoder{code: ErrCredentialNotUsable, status: http.StatusForbidden, msg: "Credential is not usable"})

	// Authentication flow errors
	errors.MustRegister(&authnCoder{code: ErrAuthenticationFailed, status: http.StatusUnauthorized, msg: "Authentication failed"})
	errors.MustRegister(&authnCoder{code: ErrOTPInvalid, status: http.StatusUnauthorized, msg: "OTP is invalid or expired"})
	errors.MustRegister(&authnCoder{code: ErrStateMismatch, status: http.StatusUnauthorized, msg: "OAuth state mismatch"})
	errors.MustRegister(&authnCoder{code: ErrIDPExchangeFailed, status: http.StatusBadGateway, msg: "Failed to exchange code with identity provider"})
	errors.MustRegister(&authnCoder{code: ErrNoBinding, status: http.StatusUnauthorized, msg: "No account binding found"})
	errors.MustRegister(&authnCoder{code: ErrOTPSendTooFrequent, status: http.StatusTooManyRequests, msg: "OTP send too frequent"})
	errors.MustRegister(&authnCoder{code: ErrTokenRevokeFailed, status: http.StatusInternalServerError, msg: "Token revoke failed"})
}

// authnCoder 实现 errors.Coder 接口
type authnCoder struct {
	code   int
	status int
	msg    string
}

func (c *authnCoder) Code() int {
	return c.code
}

func (c *authnCoder) HTTPStatus() int {
	return c.status
}

func (c *authnCoder) String() string {
	return c.msg
}

func (c *authnCoder) Reference() string {
	return ""
}
