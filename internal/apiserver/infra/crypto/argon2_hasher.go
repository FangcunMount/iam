package crypto

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"

	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
)

// Argon2Hasher 基于Argon2id的密码哈希实现
type Argon2Hasher struct {
	pepper     string // 全局密钥（应从配置中读取）
	memory     uint32 // 内存成本（KB）
	iterations uint32 // 时间成本（迭代次数）
	threads    uint8  // 并行度
	keyLen     uint32 // 输出密钥长度
}

// 确保实现了接口
var _ authentication.PasswordHasher = (*Argon2Hasher)(nil)

// NewArgon2Hasher 创建Argon2密码哈希器
// pepper: 全局密钥，应该从环境变量或配置中读取，不应硬编码
func NewArgon2Hasher(pepper string) authentication.PasswordHasher {
	return &Argon2Hasher{
		pepper:     pepper,
		memory:     64 * 1024, // 64 MB
		iterations: 3,         // 3次迭代
		threads:    4,         // 4个线程
		keyLen:     32,        // 256位密钥
	}
}

// Verify 验证明文密码与存储的哈希值是否匹配
// storedHash: PHC格式，例如 $argon2id$v=19$m=65536,t=3,p=4$base64salt$base64hash
// plaintext: 明文密码（已加pepper）
func (h *Argon2Hasher) Verify(storedHash, plaintext string) bool {
	// 解析PHC格式
	parts := strings.Split(storedHash, "$")
	if len(parts) != 6 {
		return false
	}

	// 检查算法类型
	if parts[1] != "argon2id" {
		return false
	}

	// 解析参数
	var memory, iterations uint32
	var threads uint8
	_, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &threads)
	if err != nil {
		return false
	}

	// 解码salt和hash
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}

	expectedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}

	// 使用相同参数计算哈希
	hash := argon2.IDKey([]byte(plaintext), salt, iterations, memory, threads, uint32(len(expectedHash)))

	// 常量时间比较（防止时序攻击）
	return subtle.ConstantTimeCompare(hash, expectedHash) == 1
}

// NeedRehash 检查哈希值是否需要重新哈希（算法参数升级）
func (h *Argon2Hasher) NeedRehash(storedHash string) bool {
	parts := strings.Split(storedHash, "$")
	if len(parts) != 6 {
		return true // 格式错误，需要重新哈希
	}

	if parts[1] != "argon2id" {
		return true // 算法不匹配
	}

	// 解析参数
	var memory, iterations uint32
	var threads uint8
	_, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &threads)
	if err != nil {
		return true
	}

	// 检查参数是否与当前配置匹配
	if memory != h.memory || iterations != h.iterations || threads != h.threads {
		return true
	}

	return false
}

// Hash 对明文密码进行哈希
// plaintext: 明文密码（已加pepper）
func (h *Argon2Hasher) Hash(plaintext string) (string, error) {
	// 生成随机salt
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}

	// 计算哈希
	hash := argon2.IDKey([]byte(plaintext), salt, h.iterations, h.memory, h.threads, h.keyLen)

	// 返回PHC格式
	return fmt.Sprintf(
		"$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		h.memory,
		h.iterations,
		h.threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

// Pepper 获取全局pepper
func (h *Argon2Hasher) Pepper() string {
	return h.pepper
}
