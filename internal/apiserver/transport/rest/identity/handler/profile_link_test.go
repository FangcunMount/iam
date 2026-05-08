package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/gin-gonic/gin"

	appprofilelink "github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/profilelink"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/FangcunMount/iam/v2/internal/pkg/requestctx"
)

func TestProfileLinkHandlerListDefaultsToCurrentUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	access := &profileLinkAccessStub{
		listResult: []*appprofilelink.ProfileLinkResult{{
			ID:            1,
			UserID:        "100",
			ProfileID:     "200",
			Relation:      "parent",
			EstablishedAt: time.Now().Format(time.RFC3339),
		}},
	}
	handler := NewProfileLinkHandler(access)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v2/identity/profile-links", nil)
	requestctx.SetUserID(c, meta.FromUint64(100))

	handler.List(c)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if len(access.listCalls) != 1 || access.listCalls[0].currentUserID != meta.FromUint64(100) {
		t.Fatalf("ListForCurrentUser calls = %#v, want current user 100", access.listCalls)
	}
}

func TestProfileLinkHandlerListRejectsCrossUserQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)

	access := &profileLinkAccessStub{
		listErr: perrors.WithCode(code.ErrPermissionDenied, "cannot query profile links for another user"),
	}
	handler := NewProfileLinkHandler(access)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v2/identity/profile-links?user_id=999", nil)
	requestctx.SetUserID(c, meta.FromUint64(100))

	handler.List(c)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
	if len(access.listCalls) != 1 {
		t.Fatalf("ListForCurrentUser calls = %d, want 1", len(access.listCalls))
	}
}

func TestProfileLinkHandlerListRejectsProfileLookupForNonRef(t *testing.T) {
	gin.SetMode(gin.TestMode)

	access := &profileLinkAccessStub{
		listErr: perrors.WithCode(code.ErrPermissionDenied, "forbidden"),
	}
	handler := NewProfileLinkHandler(access)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v2/identity/profile-links?profile_id=200", nil)
	requestctx.SetUserID(c, meta.FromUint64(100))

	handler.List(c)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
	if len(access.listCalls) != 1 {
		t.Fatalf("ListForCurrentUser calls = %d, want 1", len(access.listCalls))
	}
}

type profileLinkAccessStub struct {
	listResult []*appprofilelink.ProfileLinkResult
	listErr    error
	listCalls  []struct {
		currentUserID meta.ID
		dto           appprofilelink.ListProfileLinksDTO
	}
}

func (s *profileLinkAccessStub) List(_ context.Context, currentUserID meta.ID, dto appprofilelink.ListProfileLinksDTO) ([]*appprofilelink.ProfileLinkResult, error) {
	s.listCalls = append(s.listCalls, struct {
		currentUserID meta.ID
		dto           appprofilelink.ListProfileLinksDTO
	}{currentUserID: currentUserID, dto: dto})
	return s.listResult, s.listErr
}
