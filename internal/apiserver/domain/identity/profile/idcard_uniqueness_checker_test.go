package profile

import (
	"context"
	"errors"
	"fmt"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

func TestIDCardUniquenessChecker_CheckIDCardUniqueSuccess(t *testing.T) {
	repo := &profileRepoStub{}
	checker := NewIDCardUniquenessChecker(repo)
	idCard := mustIDCard(t, "小明", "110101202001151234")

	err := checker.CheckIDCardUnique(context.Background(), idCard)

	require.NoError(t, err)
	assert.Equal(t, 1, repo.findIDCardCalls)
}

func TestIDCardUniquenessChecker_CheckIDCardUniqueEmpty(t *testing.T) {
	repo := &profileRepoStub{}
	checker := NewIDCardUniquenessChecker(repo)

	err := checker.CheckIDCardUnique(context.Background(), meta.IDCard{})

	require.NoError(t, err)
	assert.Equal(t, 0, repo.findIDCardCalls)
}

func TestIDCardUniquenessChecker_CheckIDCardUniqueDuplicate(t *testing.T) {
	idCard := mustIDCard(t, "小明", "110101202001151234")
	repo := &profileRepoStub{
		byIDCard: map[string]*Profile{
			idCard.String(): {ID: meta.FromUint64(1), Name: "既有档案", IDCard: idCard},
		},
	}
	checker := NewIDCardUniquenessChecker(repo)

	err := checker.CheckIDCardUnique(context.Background(), idCard)

	require.Error(t, err)
	assert.True(t, perrors.IsCode(err, code.ErrIdentityProfileExists))
	assert.Equal(t, 1, repo.findIDCardCalls)
}

func TestIDCardUniquenessChecker_CheckIDCardChange(t *testing.T) {
	repo := &profileRepoStub{}
	idCard := mustIDCard(t, "小明", "110101202001151234")
	profile := &Profile{ID: meta.FromUint64(7), Name: "小明", IDCard: idCard}
	checker := NewIDCardUniquenessChecker(repo)

	err := checker.CheckIDCardChange(context.Background(), profile, idCard)
	require.NoError(t, err)
	assert.Equal(t, 0, repo.findIDCardCalls)

	newIDCard := mustIDCard(t, "小明", "110101202002021239")
	err = checker.CheckIDCardChange(context.Background(), profile, newIDCard)
	require.NoError(t, err)
	assert.Equal(t, 1, repo.findIDCardCalls)

	duplicateIDCard := mustIDCard(t, "小明", "110101202003031236")
	repo.byIDCard = map[string]*Profile{
		duplicateIDCard.String(): {ID: meta.FromUint64(8), Name: "其他档案", IDCard: duplicateIDCard},
	}
	err = checker.CheckIDCardChange(context.Background(), profile, duplicateIDCard)
	require.Error(t, err)
	assert.True(t, perrors.IsCode(err, code.ErrIdentityProfileExists))
	assert.Equal(t, 2, repo.findIDCardCalls)
}

func TestIDCardUniquenessChecker_CheckIDCardUniqueErrorPropagation(t *testing.T) {
	idCard := mustIDCard(t, "小明", "110101202001151234")
	checker := NewIDCardUniquenessChecker(&profileRepoStub{err: errors.New("db down")})

	err := checker.CheckIDCardUnique(context.Background(), idCard)

	require.Error(t, err)
	assert.Contains(t, fmt.Sprintf("%-v", err), "check profile id card")
}

func mustIDCard(t *testing.T, name string, number string) meta.IDCard {
	t.Helper()
	idCard, err := meta.NewIDCard(name, number)
	require.NoError(t, err)
	return idCard
}

type profileRepoStub struct {
	byIDCard        map[string]*Profile
	err             error
	findIDCardCalls int
}

func (s *profileRepoStub) Create(context.Context, *Profile) error { return nil }
func (s *profileRepoStub) FindByID(context.Context, meta.ID) (*Profile, error) {
	return nil, nil
}
func (s *profileRepoStub) FindByIDs(context.Context, []meta.ID) (map[meta.ID]*Profile, error) {
	return nil, nil
}
func (s *profileRepoStub) FindByName(context.Context, string) (*Profile, error) { return nil, nil }
func (s *profileRepoStub) FindByIDCard(ctx context.Context, idCard meta.IDCard) (*Profile, error) {
	s.findIDCardCalls++
	if s.err != nil {
		return nil, s.err
	}
	if s.byIDCard != nil {
		if p := s.byIDCard[idCard.String()]; p != nil {
			return p, nil
		}
	}
	return nil, perrors.WithCode(code.ErrIdentityProfileNotFound, "profile not found")
}
func (s *profileRepoStub) FindListByName(context.Context, string) ([]*Profile, error) {
	return nil, nil
}
func (s *profileRepoStub) FindListByNameAndBirthday(context.Context, string, meta.Birthday) ([]*Profile, error) {
	return nil, nil
}
func (s *profileRepoStub) Update(context.Context, *Profile) error { return nil }
