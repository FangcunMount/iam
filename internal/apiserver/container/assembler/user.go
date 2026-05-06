package assembler

import (
	"gorm.io/gorm"

	"github.com/FangcunMount/component-base/pkg/errors"
	appprofile "github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/profile"
	appprofilelink "github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/profilelink"
	appuser "github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/user"
	sessiondomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/session"
	mysqlIdentityUow "github.com/FangcunMount/iam/v2/internal/apiserver/infra/mysql/uow/identity"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// UserModule 用户模块
// 负责组装用户相关的所有组件
type UserModule struct {
	userCreator          appuser.Creator
	userEditor           appuser.Editor
	userStatusChanger    appuser.StatusChanger
	userDirectory        appuser.Directory
	profileDirectory     appprofile.Directory
	myProfiles           appprofile.MyProfiles
	profileLinkCommands  appprofilelink.Commands
	profileLinkDirectory appprofilelink.Directory
	myProfileLinks       appprofilelink.MyProfileLinks
	roleNames            RoleNameReader
}

// NewUserModule 创建用户模块
func NewUserModule() *UserModule {
	return &UserModule{}
}

type UserModuleDeps struct {
	DB             *gorm.DB
	RoleNames      RoleNameReader
	SessionManager sessiondomain.Manager
}

// InitializeWithDeps 初始化用户模块。
func (m *UserModule) InitializeWithDeps(deps UserModuleDeps) error {
	if deps.DB == nil {
		return errors.WithCode(code.ErrModuleInitializationFailed, "database connection is nil")
	}

	// 事务
	uow := mysqlIdentityUow.NewUnitOfWork(deps.DB)

	userCreator := appuser.NewCreator(uow)
	userEditor := appuser.NewEditor(uow)
	userStatusChanger := appuser.NewStatusChanger(uow, deps.SessionManager)
	userDirectory := appuser.NewDirectory(uow)
	profileDirectory := appprofile.NewDirectory(uow)
	myProfiles := appprofile.NewMyProfiles(uow)
	profileLinkCommands := appprofilelink.NewCommands(uow)
	profileLinkDirectory := appprofilelink.NewDirectory(uow)
	myProfileLinks := appprofilelink.NewMyProfileLinks(uow)

	m.userCreator = userCreator
	m.userEditor = userEditor
	m.userStatusChanger = userStatusChanger
	m.userDirectory = userDirectory
	m.profileDirectory = profileDirectory
	m.myProfiles = myProfiles
	m.profileLinkCommands = profileLinkCommands
	m.profileLinkDirectory = profileLinkDirectory
	m.myProfileLinks = myProfileLinks
	m.roleNames = deps.RoleNames
	return nil
}

// Cleanup 清理模块资源
func (m *UserModule) Cleanup() error {
	// 如果有需要清理的资源，在这里进行清理
	// 比如关闭数据库连接、释放缓存等
	return nil
}

// CheckHealth 检查模块健康状态
func (m *UserModule) CheckHealth() error {
	return nil
}

func (m *UserModule) ApplicationCapabilities() UserApplicationCapabilities {
	if m == nil {
		return UserApplicationCapabilities{}
	}
	return UserApplicationCapabilities{
		UserCreator:          m.userCreator,
		UserEditor:           m.userEditor,
		UserStatusChanger:    m.userStatusChanger,
		UserDirectory:        m.userDirectory,
		ProfileDirectory:     m.profileDirectory,
		MyProfiles:           m.myProfiles,
		ProfileLinkCommands:  m.profileLinkCommands,
		ProfileLinkDirectory: m.profileLinkDirectory,
		MyProfileLinks:       m.myProfileLinks,
		RoleNames:            m.roleNames,
	}
}
