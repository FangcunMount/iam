package account

import (
	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/account"
	"gorm.io/gorm"
)

// 编译时验证接口实现。
var _ account.Repository = (*AccountRepository)(nil)

func ExampleNewAccountRepository() {
	var db *gorm.DB
	repo := NewAccountRepository(db)
	_ = repo
}
