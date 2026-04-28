package credential

import (
	credDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/credential"
	"gorm.io/gorm"
)

// 编译时验证接口实现
var _ credDomain.Repository = (*Repository)(nil)

func ExampleNewRepository() {
	var db *gorm.DB
	repo := NewRepository(db)
	_ = repo
}
