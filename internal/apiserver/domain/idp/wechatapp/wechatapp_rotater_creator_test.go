package wechatapp_test

import (
	"context"
	"testing"
	"time"

	wechatapp "github.com/FangcunMount/iam/v2/internal/apiserver/domain/idp/wechatapp"
	"github.com/FangcunMount/iam/v2/internal/apiserver/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 使用共享 stub

func TestCreator_Create_SuccessAndDuplicate(t *testing.T) {
	repo := &testhelpers.WechatRepoStub{Existing: nil, Err: nil}
	c := wechatapp.NewCreator(repo)

	app, err := c.Create(context.Background(), "appid-1", "MyApp", wechatapp.MiniProgram)
	require.NoError(t, err)
	require.NotNil(t, app)
	assert.Equal(t, "appid-1", app.AppID)
	assert.Equal(t, "MyApp", app.Name)
	assert.Equal(t, wechatapp.MiniProgram, app.Type)
	assert.True(t, app.IsEnabled())

	// duplicate
	repoDup := &testhelpers.WechatRepoStub{Existing: &wechatapp.WechatApp{AppID: "appid-1"}, Err: nil}
	c2 := wechatapp.NewCreator(repoDup)
	_, err = c2.Create(context.Background(), "appid-1", "MyApp", wechatapp.MiniProgram)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestCreator_Create_InvalidParams(t *testing.T) {
	repo := &testhelpers.WechatRepoStub{}
	c := wechatapp.NewCreator(repo)
	_, err := c.Create(context.Background(), "", "name", wechatapp.MiniProgram)
	require.Error(t, err)
	_, err = c.Create(context.Background(), "appid", "", wechatapp.MiniProgram)
	require.Error(t, err)
	_, err = c.Create(context.Background(), "appid", "name", "")
	require.Error(t, err)
}

// 使用共享 VaultStub

func TestRotateAuthSecret_BasicFlows(t *testing.T) {
	now := time.Now()
	tv := func() time.Time { return now }
	vault := &testhelpers.VaultStub{}
	r := wechatapp.NewCredentialRotater(vault, tv)

	// nil app
	err := r.RotateAuthSecret(context.Background(), nil, "somesecretvalue1234")
	require.Error(t, err)

	// invalid secret (too short)
	app := wechatapp.NewWechatApp(wechatapp.MiniProgram, "appid", wechatapp.WithWechatAppName("n"))
	app.Cred = &wechatapp.Credentials{}
	err = r.RotateAuthSecret(context.Background(), app, "short")
	require.Error(t, err)

	// archived app
	app2 := wechatapp.NewWechatApp(wechatapp.MiniProgram, "appid2", wechatapp.WithWechatAppName("n2"))
	app2.Cred = &wechatapp.Credentials{}
	app2.Archive()
	err = r.RotateAuthSecret(context.Background(), app2, "longenoughsecretvalue123")
	require.Error(t, err)

	// missing vault
	rNil := wechatapp.NewCredentialRotater(nil, tv)
	app3 := wechatapp.NewWechatApp(wechatapp.MiniProgram, "appid3", wechatapp.WithWechatAppName("n3"))
	app3.Cred = &wechatapp.Credentials{}
	err = rNil.RotateAuthSecret(context.Background(), app3, "longenoughsecretvalue123")
	require.Error(t, err)

	// success
	app4 := wechatapp.NewWechatApp(wechatapp.MiniProgram, "appid4", wechatapp.WithWechatAppName("n4"))
	app4.Cred = &wechatapp.Credentials{}
	err = r.RotateAuthSecret(context.Background(), app4, "a-very-long-secret-012345")
	require.NoError(t, err)
	require.NotNil(t, app4.Cred)
	require.NotNil(t, app4.Cred.Auth)
	assert.NotEmpty(t, app4.Cred.Auth.AppSecretCipher)
	// fingerprint should match computed
	assert.Equal(t, wechatapp.Fingerprint("a-very-long-secret-012345"), app4.Cred.Auth.Fingerprint)
	assert.Equal(t, 1, app4.Cred.Auth.Version)
	require.NotNil(t, app4.Cred.Auth.LastRotatedAt)

	// idempotent: rotating again with same secret should not increase version
	prevVer := app4.Cred.Auth.Version
	err = r.RotateAuthSecret(context.Background(), app4, "a-very-long-secret-012345")
	require.NoError(t, err)
	assert.Equal(t, prevVer, app4.Cred.Auth.Version)
}

func TestRotateMsgAESKey_BasicFlows(t *testing.T) {
	now := time.Now()
	tv := func() time.Time { return now }
	vault := &testhelpers.VaultStub{}
	r := wechatapp.NewCredentialRotater(vault, tv)

	app := wechatapp.NewWechatApp(wechatapp.MiniProgram, "appid", wechatapp.WithWechatAppName("n"))
	app.Cred = &wechatapp.Credentials{}
	// invalid length
	err := r.RotateMsgAESKey(context.Background(), app, "token", "shortkey")
	require.Error(t, err)

	// missing vault
	rNil := wechatapp.NewCredentialRotater(nil, tv)
	err = rNil.RotateMsgAESKey(context.Background(), app, "t", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	require.Error(t, err)

	// success
	err = r.RotateMsgAESKey(context.Background(), app, "tok", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	require.NoError(t, err)
	require.NotNil(t, app.Cred)
	require.NotNil(t, app.Cred.Msg)
	assert.Equal(t, "tok", app.Cred.Msg.CallbackToken)
	assert.NotEmpty(t, app.Cred.Msg.EncodingAESKeyCipher)
	assert.Equal(t, 1, app.Cred.Msg.Version)
	require.NotNil(t, app.Cred.Msg.LastRotatedAt)
}
