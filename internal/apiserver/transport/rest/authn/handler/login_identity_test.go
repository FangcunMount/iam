package handler

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	linkingapp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/linking"
	tokenapp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/token"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/loginidentity"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/FangcunMount/iam/v3/internal/pkg/requestctx"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type capturingIdentityLinker struct {
	linkingapp.Linker
	request linkingapp.LinkRequest
}

func (s *capturingIdentityLinker) Link(_ context.Context, req linkingapp.LinkRequest) (*linkingapp.LinkResult, error) {
	s.request = req
	return &linkingapp.LinkResult{Identity: &loginidentity.LoginIdentity{ID: meta.FromUint64(3)}}, nil
}

func TestLinkHandlersUseTrustedAuthenticationTime(t *testing.T) {
	for _, method := range []string{"phone", "mini", "wecom", "open"} {
		t.Run(method, func(t *testing.T) {
			linker := &capturingIdentityLinker{}
			h := NewLoginIdentityHandler(linker, nil, nil, linkingapp.NewCompleteWechatOpenLink(linkStateVerifier{}, linker), WechatOpenLinkConfig{})
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			// A payload timestamp must not replace the trusted, older auth_time.
			c.Request = httptest.NewRequest("POST", "/", strings.NewReader(`{"phone":"13800138000","otp_code":"123456","app_id":"app","corp_id":"corp","code":"code","state":"state","authenticated_at":"2099-01-01T00:00:00Z"}`))
			c.Request.Header.Set("Content-Type", "application/json")
			at := time.Now().Add(-time.Hour).UTC().Truncate(time.Second)
			requestctx.SetUserID(c, meta.FromUint64(7))
			requestctx.SetClaims(c, &tokenapp.TokenClaims{AuthenticatedAt: at})
			switch method {
			case "phone":
				h.LinkPhone(c)
			case "mini":
				h.LinkWechatMiniProgram(c)
			case "wecom":
				h.LinkWecom(c)
			case "open":
				h.CompleteWechatOpenLink(c)
			}
			require.Equal(t, meta.FromUint64(7), linker.request.UserID)
			require.Equal(t, &at, linker.request.AuthenticatedAt)
		})
	}
}

type linkStateVerifier struct{}

func (linkStateVerifier) VerifyAndConsumeWechatOpenLink(context.Context, string) (linkingapp.WechatOpenLinkContext, error) {
	return linkingapp.WechatOpenLinkContext{UserID: meta.FromUint64(7), AppID: "app"}, nil
}
