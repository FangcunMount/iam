package policy

import (
	"context"
	"math"
	"os"
	"testing"

	"github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/policyversion"
	testutil "github.com/FangcunMount/iam/v5/internal/apiserver/application/identity/testutil"
	"github.com/stretchr/testify/require"
)

func TestCommittedVersionReaderTracksCommittedMySQLRow(t *testing.T) {
	if os.Getenv("MYSQL_HOST") == "" {
		t.Skip("requires isolated MySQL job")
	}
	db := testutil.OpenDBForIntegrationTest(t, &PolicyVersionPO{})
	ctx := context.Background()
	repository := NewPolicyVersionRepository(db)
	current, err := repository.GetOrCreate(ctx)
	require.NoError(t, err)
	require.Less(t, current.Version, int64(math.MaxInt64))
	original := current.Version
	t.Cleanup(func() {
		require.NoError(t, db.Model(&PolicyVersionPO{}).Where("id = ?", 1).Update("policy_version", original).Error)
	})
	reader := policyversion.NewReader(repository)
	version, err := reader.ReadCommittedVersion(ctx)
	require.NoError(t, err)
	require.Equal(t, original, version)

	tx := db.Begin()
	require.NoError(t, tx.Error)
	t.Cleanup(func() { _ = tx.Rollback().Error })
	require.NoError(t, tx.Model(&PolicyVersionPO{}).Where("id = ?", 1).Update("policy_version", original+1).Error)
	version, err = reader.ReadCommittedVersion(ctx)
	require.NoError(t, err)
	require.Equal(t, original, version, "uncommitted version must not be reported")
	require.NoError(t, tx.Commit().Error)
	version, err = reader.ReadCommittedVersion(ctx)
	require.NoError(t, err)
	require.Equal(t, original+1, version)
}
