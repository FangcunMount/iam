package authn

import (
	"github.com/FangcunMount/component-base/pkg/log"
	schedulerInfra "github.com/FangcunMount/iam/v2/internal/apiserver/infra/scheduler"
	apiserveroptions "github.com/FangcunMount/iam/v2/internal/apiserver/options"
)

type jwksRotationRuntimeOptions struct {
	automaticEnabled bool
	checkCron        string
}

func (m *AuthnModule) initializeSchedulers(jwksOptions apiserveroptions.JWKSOptions) {
	m.rotationOptions = jwksRotationRuntimeOptions{
		automaticEnabled: jwksOptions.Rotation.AutomaticEnabled,
		checkCron:        jwksOptions.Rotation.CheckCron,
	}
	if !m.rotationOptions.automaticEnabled {
		m.rotationScheduler = nil
		return
	}
	logger := log.New(log.NewOptions())

	m.rotationScheduler = schedulerInfra.NewKeyRotationCronScheduler(
		m.keyLifecycleApp,
		m.rotationOptions.checkCron,
		logger,
	)
}
