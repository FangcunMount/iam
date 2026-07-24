package identity

import (
	"context"

	"github.com/FangcunMount/component-base/pkg/errors"
	appprofile "github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/profile"
	appprofilelink "github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/profilelink"
	appuser "github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/user"
	sessionrevocation "github.com/FangcunMount/iam/v2/internal/apiserver/infra/mysql/sessionrevocation"
	mysqlIdentityUow "github.com/FangcunMount/iam/v2/internal/apiserver/infra/mysql/uow/identity"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// IdentityModule 身份模块
// 负责组装身份相关的所有组件
type IdentityModule struct {
	userCreator            appuser.Creator
	userEditor             appuser.Editor
	userStatusChanger      appuser.StatusChanger
	userDirectory          appuser.Directory
	profileDirectory       appprofile.Directory
	myProfiles             appprofile.MyProfiles
	profileLinkCommands    appprofilelink.Commands
	profileLinkDirectory   appprofilelink.Directory
	myProfileLinks         appprofilelink.MyProfileLinks
	roleNames              RoleNameReader
	sessionRevocationStore *sessionrevocation.Store
	stopSessionRevocation  context.CancelFunc
	sessionRevocationDone  chan struct{}
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
	userStatusChanger := appuser.NewStatusChanger(uow)
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
	m.sessionRevocationStore = sessionrevocation.NewStore(deps.DB)
	if deps.SessionRevoker != nil && deps.SessionRevocationConfig.PollInterval > 0 {
		worker := sessionrevocation.NewWorker(m.sessionRevocationStore, deps.SessionRevoker, deps.SessionRevocationConfig)
		ctx, cancel := context.WithCancel(context.Background())
		m.stopSessionRevocation = cancel
		m.sessionRevocationDone = make(chan struct{})
		go func() {
			defer close(m.sessionRevocationDone)
			worker.Run(ctx)
		}()
	}
	return nil
}

// Cleanup 清理模块资源
func (m *IdentityModule) Cleanup() error {
	if m == nil || m.stopSessionRevocation == nil {
		return nil
	}
	m.stopSessionRevocation()
	if m.sessionRevocationDone != nil {
		<-m.sessionRevocationDone
	}
	m.stopSessionRevocation = nil
	return nil
}

func (m *IdentityModule) SessionRevocationStore() *sessionrevocation.Store {
	if m == nil {
		return nil
	}
	return m.sessionRevocationStore
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
