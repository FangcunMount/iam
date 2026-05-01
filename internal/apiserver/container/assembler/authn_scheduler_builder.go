package assembler

import (
	"github.com/FangcunMount/component-base/pkg/log"
	schedulerInfra "github.com/FangcunMount/iam/v2/internal/apiserver/infra/scheduler"
)

func (m *AuthnModule) initializeSchedulers() {
	logger := log.New(log.NewOptions())
	cronSpec := "0 2 * * *"

	m.rotationScheduler = schedulerInfra.NewKeyRotationCronScheduler(
		m.keyRotationApp,
		cronSpec,
		logger,
	)
}
