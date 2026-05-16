package suggest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	appsuggest "github.com/FangcunMount/iam/v2/internal/apiserver/application/suggest"
	"github.com/FangcunMount/iam/v2/internal/apiserver/infra/suggest/ratelimit"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/FangcunMount/iam/v2/internal/pkg/requestctx"
)

func TestProfileReturnsSuggestItems(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	Register(engine, Dependencies{
		Service: suggestorStub{items: []appsuggest.ProfileSuggestItem{{
			ProfileID:   1,
			DisplayName: "张三",
			MobileMask:  "138****8000",
			Weight:      5,
		}}},
		Middlewares: []gin.HandlerFunc{func(c *gin.Context) {
			requestctx.SetUserID(c, meta.ID(100))
			c.Next()
		}},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v2/suggest/profile?k=张", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var body struct {
		Code int                          `json:"code"`
		Data []ProfileSuggestResponseItem `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if body.Code != 0 || len(body.Data) != 1 || body.Data[0].Name != "张三" || body.Data[0].ID != "1" {
		t.Fatalf("body = %#v", body)
	}
}

func TestProfileMissingKeywordReturnsBindError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	Register(engine, Dependencies{
		Service: suggestorStub{},
		Middlewares: []gin.HandlerFunc{func(c *gin.Context) {
			requestctx.SetUserID(c, meta.ID(100))
			c.Next()
		}},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v2/suggest/profile", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestProfileMissingUserReturnsUnauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	Register(engine, Dependencies{
		Service: suggestorStub{},
		Middlewares: []gin.HandlerFunc{func(c *gin.Context) {
			c.Next()
		}},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v2/suggest/profile?k=a", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestProfileRateLimitedSecondRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	Register(engine, Dependencies{
		Service: suggestorStub{},
		RateLimiter: ratelimit.NewMemoryLimiter(appsuggest.RateLimitConfig{
			PerOperatorQPS:                1,
			PerOperatorBurst:              1,
			MobileKeywordPerOperatorQPS:   1,
			MobileKeywordPerOperatorBurst: 1,
		}),
		Middlewares: []gin.HandlerFunc{func(c *gin.Context) {
			requestctx.SetUserID(c, meta.ID(100))
			c.Next()
		}},
	})

	reqFactory := func() *http.Request {
		return httptest.NewRequest(http.MethodGet, "/api/v2/suggest/profile?k=a", nil)
	}
	w1 := httptest.NewRecorder()
	engine.ServeHTTP(w1, reqFactory())
	if w1.Code != http.StatusOK {
		t.Fatalf("first status = %d", w1.Code)
	}
	w2 := httptest.NewRecorder()
	engine.ServeHTTP(w2, reqFactory())
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d, want 429", w2.Code)
	}
}

type suggestorStub struct {
	items []appsuggest.ProfileSuggestItem
}

func (s suggestorStub) SuggestProfile(context.Context, appsuggest.SuggestProfileRequest) ([]appsuggest.ProfileSuggestItem, error) {
	return append([]appsuggest.ProfileSuggestItem(nil), s.items...), nil
}
