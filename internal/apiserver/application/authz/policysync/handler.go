package policysync

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	authzshared "github.com/FangcunMount/iam/v2/internal/apiserver/application/authz/shared"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/policy"
	"github.com/FangcunMount/iam/v2/internal/apiserver/eventing"
)

const (
	Topic   = "iam.authz.version"
	Channel = "iam-policy-sync"
)

type RuntimePolicyReloader interface {
	LoadPolicy(ctx context.Context) error
}

type PolicyVersionEventRecorder interface {
	RecordPolicyVersionEvent(tenantID string, version int64, eventAt time.Time)
}

type VersionEventHandler struct {
	reloader RuntimePolicyReloader
	recorder PolicyVersionEventRecorder
}

func NewHandler(reloader RuntimePolicyReloader, recorder PolicyVersionEventRecorder) *VersionEventHandler {
	return &VersionEventHandler{
		reloader: reloader,
		recorder: recorder,
	}
}

func (h *VersionEventHandler) Handle(ctx context.Context, payload []byte, eventType string) error {
	if eventType != "" && eventType != eventing.AuthzVersionChanged {
		return nil
	}
	if h == nil || h.reloader == nil {
		return fmt.Errorf("authz policy sync reloader is required")
	}

	var versionEvent policy.VersionChangedPayload
	if err := json.Unmarshal(payload, &versionEvent); err != nil {
		return fmt.Errorf("decode authz policy version event: %w", err)
	}
	if versionEvent.TenantID == "" || versionEvent.Version <= 0 {
		return fmt.Errorf("invalid authz policy version event payload")
	}

	now := time.Now()
	if h.recorder != nil {
		h.recorder.RecordPolicyVersionEvent(versionEvent.TenantID, versionEvent.Version, now)
	}
	authzshared.ReloadRuntimePolicy(ctx, h.reloader, "version_changed_event")
	return nil
}
