package authorization

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	authzruntime "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/runtime"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/subject"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

type NativeRuntime interface {
	Check(ctx context.Context, request authzruntime.Request) (authzruntime.Decision, error)
	GetAuthorizationSnapshot(ctx context.Context, sub subject.Ref, tenantID, appName string) (authzruntime.SubjectSnapshot, error)
}

type NativeChecker struct{ runtime NativeRuntime }

func NewNativeChecker(runtime NativeRuntime) *NativeChecker { return &NativeChecker{runtime: runtime} }

func (c *NativeChecker) Check(ctx context.Context, request authzruntime.Request) (authzruntime.Decision, error) {
	if c == nil || c.runtime == nil {
		return authzruntime.Decision{}, perrors.WithCode(code.ErrInternalServerError, "authorization runtime is unavailable")
	}
	return c.runtime.Check(ctx, request)
}

type NativeSnapshotReader struct{ runtime NativeRuntime }

func NewNativeSnapshotReader(runtime NativeRuntime) *NativeSnapshotReader {
	return &NativeSnapshotReader{runtime: runtime}
}

func (r *NativeSnapshotReader) Read(ctx context.Context, sub subject.Ref, tenantID, appName string) (authzruntime.SubjectSnapshot, error) {
	if r == nil || r.runtime == nil {
		return authzruntime.SubjectSnapshot{}, perrors.WithCode(code.ErrInternalServerError, "authorization runtime is unavailable")
	}
	if sub.IsZero() || strings.TrimSpace(tenantID) == "" || strings.TrimSpace(appName) == "" {
		return authzruntime.SubjectSnapshot{}, perrors.WithCode(code.ErrInvalidArgument, "subject, tenant id, and app name are required")
	}
	return r.runtime.GetAuthorizationSnapshot(ctx, sub, tenantID, appName)
}
