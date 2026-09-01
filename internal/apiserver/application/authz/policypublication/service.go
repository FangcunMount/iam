// Package policypublication publishes accepted authorization facts into the
// immutable runtime snapshot used for permission decisions.
package policypublication

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode"

	policychange "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/policychange"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/policy"
	"github.com/FangcunMount/iam/v3/internal/apiserver/eventing"
)

const (
	Topic         = "iam.authz.version"
	ChannelPrefix = "iam-policy-sync"
)

func CurrentInstanceChannel() string {
	hostname, _ := os.Hostname()
	return InstanceChannel(hostname, os.Getpid())
}

func InstanceChannel(hostname string, pid int) string {
	host := sanitizeChannelPart(hostname)
	if host == "" {
		host = "unknown"
	}
	if pid <= 0 {
		pid = os.Getpid()
	}
	return fmt.Sprintf("%s.%s.%d#ephemeral", ChannelPrefix, host, pid)
}

func sanitizeChannelPart(value string) string {
	value = strings.TrimSpace(value)
	var builder strings.Builder
	for _, r := range value {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r), r == '-', r == '_', r == '.':
			builder.WriteRune(r)
		default:
			builder.WriteByte('-')
		}
	}
	return strings.Trim(builder.String(), "-_.")
}

type PolicyVersionEventRecorder interface {
	RecordPolicyVersionEvent(tenantID string, version int64, eventAt time.Time)
}

type Service struct {
	reloader policychange.RuntimePolicyReloader
	recorder PolicyVersionEventRecorder
}

func NewService(reloader policychange.RuntimePolicyReloader, recorder PolicyVersionEventRecorder) *Service {
	return &Service{
		reloader: reloader,
		recorder: recorder,
	}
}

func (s *Service) Handle(ctx context.Context, payload []byte, eventType string) error {
	if eventType != "" && eventType != eventing.AuthzVersionChanged {
		return nil
	}
	if s == nil || s.reloader == nil {
		return fmt.Errorf("authz policy publication reloader is required")
	}

	var versionEvent policy.VersionChangedPayload
	if err := json.Unmarshal(payload, &versionEvent); err != nil {
		return fmt.Errorf("decode authz policy version event: %w", err)
	}
	if versionEvent.TenantID == "" || versionEvent.Version <= 0 {
		return fmt.Errorf("invalid authz policy version event payload")
	}

	now := time.Now()
	if s.recorder != nil {
		s.recorder.RecordPolicyVersionEvent(versionEvent.TenantID, versionEvent.Version, now)
	}
	return policychange.ReloadRuntimePolicyWithError(ctx, s.reloader, "version_changed_event")
}
