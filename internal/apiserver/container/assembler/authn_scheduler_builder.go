package assembler

import (
	"github.com/FangcunMount/component-base/pkg/log"
	schedulerInfra "github.com/FangcunMount/iam/internal/apiserver/infra/scheduler"
)

func (m *AuthnModule) initializeSchedulers() {
	logger := log.New(log.NewOptions())
	cronSpec := "0 2 * * *"

	m.rotationScheduler = schedulerInfra.NewKeyRotationCronScheduler(
		m.KeyRotationApp,
		cronSpec,
		logger,
	)
}
