// Package auth encrypt and compare password string.
package auth

import (
	"golang.org/x/crypto/bcrypt"
)

// TokenProfileAlgorithm is the only JOSE algorithm supported by IAM tokens and JWKS keys.
const TokenProfileAlgorithm = "RS256"

// Encrypt encrypts the plain text with bcrypt.
func Encrypt(source string) (string, error) {
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(source), bcrypt.DefaultCost)
	return string(hashedBytes), err
}

// Compare compares the encrypted text with the plain text if it's the same.
func Compare(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}

// 注意：JWT 签发功能已迁移至 internal/apiserver/infra/token/jwt 包
// 现在统一使用 JWKS (RS256) 非对称签名方案，不再使用 HMAC 对称密钥
// 请使用 jwt.Generator 进行 JWT 签发
