package process

import (
	"fmt"
	"strings"

	"github.com/FangcunMount/component-base/pkg/log"
)

// validateCriticalModules 验证关键模块
func (s *apiServer) validateCriticalModules(degradedAllowed bool) error {
	// 如果容器为空，则返回错误
	if s.container == nil {
		return fmt.Errorf("container is nil")
	}

	// 获取关键模块缺失列表
	missing := s.container.CriticalModulesMissing()
	if len(missing) == 0 {
		return nil
	}
	// 如果不允许降级启动，则返回错误
	if !degradedAllowed {
		return fmt.Errorf("critical modules unavailable: %s", strings.Join(missing, ", "))
	}

	log.Warnw("degraded startup: critical modules unavailable", "modules", missing)
	return nil
}
