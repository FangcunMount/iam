package identity

import (
	"github.com/FangcunMount/component-base/pkg/errors"
	appprofile "github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/profile"
	appprofilelink "github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/profilelink"
	appuser "github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/user"
	mysqlIdentityUow "github.com/FangcunMount/iam/v2/internal/apiserver/infra/mysql/uow/identity"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// IdentityModule 身份模块
// 负责组装身份相关的所有组件
type IdentityModule struct {
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

// NewIdentityModule 创建身份模块
func NewIdentityModule() *IdentityModule {
	return &IdentityModule{}
}

// InitializeWithDeps 初始化身份模块。
func (m *IdentityModule) InitializeWithDeps(deps IdentityModuleDeps) error {
	if deps.DB == nil {
		return errors.WithCode(code.ErrModuleInitializationFailed, "database connection is nil")
	}

	// 事务
	uow := mysqlIdentityUow.NewUnitOfWork(deps.DB)

	userCreator := appuser.NewCreator(uow)
	userEditor := appuser.NewEditor(uow)
	userStatusChanger := appuser.NewStatusChanger(uow, deps.SessionRevoker)
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
func (m *IdentityModule) Cleanup() error {
	// 如果有需要清理的资源，在这里进行清理
	// 比如关闭数据库连接、释放缓存等
	return nil
}

// CheckHealth 检查模块健康状态
func (m *IdentityModule) CheckHealth() error {
	return nil
}

func (m *IdentityModule) ApplicationCapabilities() ApplicationCapabilities {
	if m == nil {
		return ApplicationCapabilities{}
	}
	return ApplicationCapabilities{
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
