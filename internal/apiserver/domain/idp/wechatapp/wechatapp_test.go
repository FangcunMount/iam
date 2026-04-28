package wechatapp

import (
	"testing"

	"github.com/FangcunMount/iam/internal/pkg/meta"
	"github.com/stretchr/testify/assert"
)

func TestWechatApp_CreateAndStatus(t *testing.T) {
	id := meta.FromUint64(7)
	app := NewWechatApp(MiniProgram, "appid", WithWechatAppID(id), WithWechatAppName("mini"), WithWechatAppStatus(StatusEnabled))
	assert.Equal(t, "appid", app.AppID)
	assert.Equal(t, "mini", app.Name)
	assert.True(t, app.IsEnabled())

	app.Disable()
	assert.True(t, app.IsDisabled())
	app.Archive()
	assert.True(t, app.IsArchived())
}
