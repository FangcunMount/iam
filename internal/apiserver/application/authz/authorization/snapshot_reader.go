package authorization

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/subject"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

// SnapshotRuntime supplies the active subject authorization projection.
type SnapshotRuntime interface {
	GetAuthorizationSnapshot(context.Context, subject.Ref, string, string) (SubjectSnapshot, error)
}

// SnapshotReader exposes the application query for one subject's current
// direct roles, effective roles, permissions, and policy version.
type SnapshotReader struct {
	runtime SnapshotRuntime
}

func NewSnapshotReader(runtime SnapshotRuntime) *SnapshotReader {
	return &SnapshotReader{runtime: runtime}
}

func (r *SnapshotReader) Read(ctx context.Context, sub subject.Ref, tenantID, appName string) (SubjectSnapshot, error) {
	if r == nil || r.runtime == nil {
		return SubjectSnapshot{}, perrors.WithCode(code.ErrInternalServerError, "authorization runtime is unavailable")
	}
	if sub.IsZero() || strings.TrimSpace(tenantID) == "" || strings.TrimSpace(appName) == "" {
		return SubjectSnapshot{}, perrors.WithCode(code.ErrInvalidArgument, "subject, tenant id, and app name are required")
	}
	return r.runtime.GetAuthorizationSnapshot(ctx, sub, tenantID, appName)
}
