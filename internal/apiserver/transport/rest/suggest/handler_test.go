package suggest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	domainsuggest "github.com/FangcunMount/iam/v2/internal/apiserver/domain/suggest"
)

func TestProfileReturnsSuggestTerms(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	Register(engine, Dependencies{
		Service: suggestorStub{terms: []domainsuggest.Term{{Name: "张三", ID: 1, Mobile: "13800138000", Weight: 5}}},
		AuthMiddleware: func(c *gin.Context) {
			c.Next()
		},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v2/suggest/profile?k=张", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var body struct {
		Code int                  `json:"code"`
		Data []domainsuggest.Term `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if body.Code != 0 || len(body.Data) != 1 || body.Data[0].Name != "张三" {
		t.Fatalf("body = %#v", body)
	}
}

func TestProfileMissingKeywordReturnsBindError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	Register(engine, Dependencies{
		Service: suggestorStub{},
		AuthMiddleware: func(c *gin.Context) {
			c.Next()
		},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v2/suggest/profile", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
}

type suggestorStub struct {
	terms []domainsuggest.Term
}

func (s suggestorStub) Suggest(context.Context, string) []domainsuggest.Term {
	return append([]domainsuggest.Term(nil), s.terms...)
}
