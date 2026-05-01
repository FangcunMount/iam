package jwks

import (
	"time"

	"github.com/FangcunMount/iam/v2/internal/pkg/database/mysql"
)

// KeyPO 密钥持久化对象，对应 jwks_keys 表
//
// 使用通用的 AuditFields 以便与 BaseRepository 的 Syncable 接口兼容。
type KeyPO struct {
	mysql.AuditFields
	Kid       string     `gorm:"column:kid;type:varchar(64);not null;uniqueIndex:idx_kid"`
	Status    int8       `gorm:"column:status;type:tinyint;not null;default:1;index:idx_status"`
	Kty       string     `gorm:"column:kty;type:varchar(32);not null"`
	Use       string     `gorm:"column:use;type:varchar(16);not null"`
	Alg       string     `gorm:"column:alg;type:varchar(32);not null;index:idx_alg"`
	JwkJSON   []byte     `gorm:"column:jwk_json;type:json;not null"`
	NotBefore *time.Time `gorm:"column:not_before;type:datetime"`
	NotAfter  *time.Time `gorm:"column:not_after;type:datetime;index:idx_not_after"`
}

// TableName 指定表名
func (KeyPO) TableName() string {
	return "jwks_keys"
}
