package wechatapi

import (
	"net/url"
	"strings"
	"testing"
)

func TestBuildWebAppQRConnectURL(t *testing.T) {
	t.Parallel()

	raw, err := BuildWebAppQRConnectURL("wx-app-id", "https://example.com/callback", "state-token")
	if err != nil {
		t.Fatalf("BuildWebAppQRConnectURL() error = %v", err)
	}
	if !strings.HasPrefix(raw, "https://open.weixin.qq.com/connect/qrconnect?") {
		t.Fatalf("unexpected url prefix: %s", raw)
	}
	if !strings.HasSuffix(raw, "#wechat_redirect") {
		t.Fatalf("expected wechat_redirect suffix: %s", raw)
	}

	parsed, err := url.Parse(strings.TrimSuffix(raw, "#wechat_redirect"))
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}
	q := parsed.Query()
	if q.Get("appid") != "wx-app-id" {
		t.Fatalf("appid = %q", q.Get("appid"))
	}
	if q.Get("scope") != wechatWebAppQRConnectScope {
		t.Fatalf("scope = %q", q.Get("scope"))
	}
	if q.Get("state") != "state-token" {
		t.Fatalf("state = %q", q.Get("state"))
	}
}
