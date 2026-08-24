package linking

import (
	"time"

	idpresolver "github.com/FangcunMount/iam/v3/internal/apiserver/application/idp/externalidentity"
)

// linkPrepareDeps 是 prepare 阶段可用的依赖快照，避免各 Input 依赖 *linker。
type linkPrepareDeps struct {
	phoneLinkOTP PhoneLinkChallengeVerifier
	resolver     idpresolver.Resolver
	now          func() time.Time
}

// currentTime 获取当前时间。
func (d linkPrepareDeps) currentTime() time.Time {
	if d.now != nil {
		return d.now()
	}
	return time.Now()
}
