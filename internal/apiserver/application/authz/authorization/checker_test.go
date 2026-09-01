package authorization_test

import (
	"context"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	authorizationapp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/authorization"
	authorizationdomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/authorization"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/subject"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestCheckerDelegatesToDecisionRuntime(t *testing.T) {
	t.Parallel()

	runtime := &decisionRuntimeStub{decision: authorizationdomain.Decision{Allowed: true}}
	request := authorizationdomain.Request{}
	decision, err := authorizationapp.NewChecker(runtime).Check(context.Background(), request)

	require.NoError(t, err)
	require.True(t, decision.Allowed)
	require.Equal(t, request, runtime.request)
}

func TestCheckerFailsWhenRuntimeIsUnavailable(t *testing.T) {
	t.Parallel()

	_, err := authorizationapp.NewChecker(nil).Check(context.Background(), authorizationdomain.Request{})
	require.True(t, perrors.IsCode(err, code.ErrInternalServerError))
}

func TestSnapshotReaderValidatesQueryBeforeDelegating(t *testing.T) {
	t.Parallel()

	reader := authorizationapp.NewSnapshotReader(&snapshotRuntimeStub{})
	_, err := reader.Read(context.Background(), subject.Ref{}, "", "")
	require.True(t, perrors.IsCode(err, code.ErrInvalidArgument))
}

func TestSnapshotReaderDelegatesValidQuery(t *testing.T) {
	t.Parallel()

	sub, err := subject.NewUserRef(meta.FromUint64(42))
	require.NoError(t, err)
	runtime := &snapshotRuntimeStub{snapshot: authorizationapp.SubjectSnapshot{EffectiveRoles: []string{"qs:staff"}}}
	snapshot, err := authorizationapp.NewSnapshotReader(runtime).Read(context.Background(), sub, "fangcun", "qs")

	require.NoError(t, err)
	require.Equal(t, runtime.snapshot, snapshot)
	require.Equal(t, sub, runtime.subject)
	require.Equal(t, "fangcun", runtime.tenantID)
	require.Equal(t, "qs", runtime.appName)
}

type decisionRuntimeStub struct {
	request  authorizationdomain.Request
	decision authorizationdomain.Decision
}

func (s *decisionRuntimeStub) Check(_ context.Context, request authorizationdomain.Request) (authorizationdomain.Decision, error) {
	s.request = request
	return s.decision, nil
}

type snapshotRuntimeStub struct {
	subject  subject.Ref
	tenantID string
	appName  string
	snapshot authorizationapp.SubjectSnapshot
}

func (s *snapshotRuntimeStub) GetAuthorizationSnapshot(
	_ context.Context,
	sub subject.Ref,
	tenantID string,
	appName string,
) (authorizationapp.SubjectSnapshot, error) {
	s.subject = sub
	s.tenantID = tenantID
	s.appName = appName
	return s.snapshot, nil
}
