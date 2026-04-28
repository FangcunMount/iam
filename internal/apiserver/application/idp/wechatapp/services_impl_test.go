package wechatapp

import (
	"context"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/util/idutil"
	domain "github.com/FangcunMount/iam/internal/apiserver/domain/idp/wechatapp"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

type repoStub struct {
	apps           map[string]*domain.WechatApp
	listResult     []*domain.WechatApp
	lastListFilter domain.ListFilter
	updateCalls    int
}

func (r *repoStub) Create(context.Context, *domain.WechatApp) error { return nil }

func (r *repoStub) GetByID(context.Context, idutil.ID) (*domain.WechatApp, error) { return nil, nil }

func (r *repoStub) GetByAppID(_ context.Context, appID string) (*domain.WechatApp, error) {
	if r.apps == nil {
		return nil, nil
	}
	return r.apps[appID], nil
}

func (r *repoStub) List(_ context.Context, filter domain.ListFilter) ([]*domain.WechatApp, error) {
	r.lastListFilter = filter
	return r.listResult, nil
}

func (r *repoStub) Update(_ context.Context, app *domain.WechatApp) error {
	r.updateCalls++
	if r.apps == nil {
		r.apps = map[string]*domain.WechatApp{}
	}
	r.apps[app.AppID] = app
	return nil
}

func TestWechatAppApplicationService_ListApps_AppliesFilters(t *testing.T) {
	repo := &repoStub{
		listResult: []*domain.WechatApp{
			{
				ID:     meta.MustFromUint64(1001),
				AppID:  "wx-prod",
				Name:   "Questionnaire",
				Type:   domain.MP,
				Status: domain.StatusDisabled,
			},
		},
	}
	svc := NewWechatAppApplicationService(repo, nil, nil)

	appType := domain.MP
	status := domain.StatusDisabled
	result, err := svc.ListApps(context.Background(), ListWechatAppsFilter{
		Type:   &appType,
		Status: &status,
	})

	require.NoError(t, err)
	require.Len(t, result, 1)
	require.NotNil(t, repo.lastListFilter.Type)
	require.NotNil(t, repo.lastListFilter.Status)
	require.Equal(t, domain.MP, *repo.lastListFilter.Type)
	require.Equal(t, domain.StatusDisabled, *repo.lastListFilter.Status)
	require.Equal(t, "wx-prod", result[0].AppID)
}

func TestWechatAppApplicationService_UpdateApp_UpdatesNameAndType(t *testing.T) {
	app := &domain.WechatApp{
		ID:     meta.MustFromUint64(1002),
		AppID:  "wx-edit",
		Name:   "Old Name",
		Type:   domain.MiniProgram,
		Status: domain.StatusEnabled,
	}
	repo := &repoStub{
		apps: map[string]*domain.WechatApp{
			app.AppID: app,
		},
	}
	svc := NewWechatAppApplicationService(repo, nil, nil)

	name := "  New Name  "
	appType := domain.MP
	result, err := svc.UpdateApp(context.Background(), app.AppID, UpdateWechatAppDTO{
		Name: &name,
		Type: &appType,
	})

	require.NoError(t, err)
	require.Equal(t, "New Name", result.Name)
	require.Equal(t, domain.MP, result.Type)
	require.Equal(t, "New Name", repo.apps[app.AppID].Name)
	require.Equal(t, domain.MP, repo.apps[app.AppID].Type)
	require.Equal(t, 1, repo.updateCalls)
}

func TestWechatAppApplicationService_UpdateApp_RequiresFields(t *testing.T) {
	svc := NewWechatAppApplicationService(&repoStub{}, nil, nil)

	_, err := svc.UpdateApp(context.Background(), "wx-empty", UpdateWechatAppDTO{})

	require.Error(t, err)
	require.True(t, perrors.IsCode(err, code.ErrInvalidArgument))
}

func TestWechatAppApplicationService_EnableDisableApp(t *testing.T) {
	repo := &repoStub{
		apps: map[string]*domain.WechatApp{
			"wx-enable": {
				ID:     meta.MustFromUint64(1003),
				AppID:  "wx-enable",
				Name:   "Enable Me",
				Type:   domain.MiniProgram,
				Status: domain.StatusDisabled,
			},
			"wx-disable": {
				ID:     meta.MustFromUint64(1004),
				AppID:  "wx-disable",
				Name:   "Disable Me",
				Type:   domain.MP,
				Status: domain.StatusEnabled,
			},
		},
	}
	svc := NewWechatAppApplicationService(repo, nil, nil)

	enabled, err := svc.EnableApp(context.Background(), "wx-enable")
	require.NoError(t, err)
	require.Equal(t, domain.StatusEnabled, enabled.Status)
	require.Equal(t, domain.StatusEnabled, repo.apps["wx-enable"].Status)

	disabled, err := svc.DisableApp(context.Background(), "wx-disable")
	require.NoError(t, err)
	require.Equal(t, domain.StatusDisabled, disabled.Status)
	require.Equal(t, domain.StatusDisabled, repo.apps["wx-disable"].Status)
	require.Equal(t, 2, repo.updateCalls)
}
