package process

import (
	"fmt"
	"strings"

	"github.com/FangcunMount/component-base/pkg/log"
)

func (s *apiServer) validateCriticalModules(degradedAllowed bool) error {
	if s.container == nil {
		return fmt.Errorf("container is nil")
	}

	missing := s.container.CriticalModulesMissing()
	if len(missing) == 0 {
		return nil
	}
	if !degradedAllowed {
		return fmt.Errorf("critical modules unavailable: %s", strings.Join(missing, ", "))
	}

	log.Warnw("degraded startup: critical modules unavailable", "modules", missing)
	return nil
}
