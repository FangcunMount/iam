package handler

import (
	"bytes"
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

func TestProfileLinkHandlerGrantUsesCurrentUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	access := &profileLinkAccessStub{
		grantResult: &appprofilelink.ProfileLinkResult{
			ID:            1,
			UserID:        "100",
			ProfileID:     "200",
			Relation:      "parent",
			EstablishedAt: time.Now().Format(time.RFC3339),
		},
	}
	handler := NewProfileLinkHandler(access)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v2/identity/profile-links", bytes.NewBufferString(`{"profileId":"200","relation":"parent"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	requestctx.SetUserID(c, meta.FromUint64(100))

	handler.Grant(c)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body=%s", recorder.Code, http.StatusCreated, recorder.Body.String())
	}
	if len(access.grantCalls) != 1 {
		t.Fatalf("GrantForCurrentUser calls = %d, want 1", len(access.grantCalls))
	}
	if access.grantCalls[0].currentUserID != meta.FromUint64(100) || !access.grantCalls[0].dto.UserID.IsZero() {
		t.Fatalf("GrantForCurrentUser call = %#v, want current user 100 with empty dto user", access.grantCalls[0])
	}
}

func TestProfileLinkHandlerGrantRejectsDifferentUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	access := &profileLinkAccessStub{
		grantErr: perrors.WithCode(code.ErrPermissionDenied, "cannot grant profile link for another user"),
	}
	handler := NewProfileLinkHandler(access)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v2/identity/profile-links", bytes.NewBufferString(`{"userId":"999","profileId":"200","relation":"parent"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	requestctx.SetUserID(c, meta.FromUint64(100))

	handler.Grant(c)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
	if len(access.grantCalls) != 1 {
		t.Fatalf("GrantForCurrentUser calls = %d, want 1", len(access.grantCalls))
	}
}

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

func TestProfileLinkHandlerRevokeUsesCurrentUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	access := &profileLinkAccessStub{
		revokeResult: &appprofilelink.ProfileLinkResult{
			ID:            1,
			UserID:        "100",
			ProfileID:     "200",
			Relation:      "parent",
			EstablishedAt: time.Now().Format(time.RFC3339),
			RevokedAt:     time.Now().Format(time.RFC3339),
		},
	}
	handler := NewProfileLinkHandler(access)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v2/identity/profile-links/1/revoke", nil)
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	requestctx.SetUserID(c, meta.FromUint64(100))

	handler.Revoke(c)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if len(access.revokeCalls) != 1 {
		t.Fatalf("RevokeForCurrentUser calls = %d, want 1", len(access.revokeCalls))
	}
	if access.revokeCalls[0].currentUserID != meta.FromUint64(100) || access.revokeCalls[0].dto.ProfileLinkID != meta.FromUint64(1) {
		t.Fatalf("RevokeForCurrentUser call = %#v, want current user 100 and link id 1", access.revokeCalls[0])
	}
}

type profileLinkAccessStub struct {
	grantResult *appprofilelink.ProfileLinkResult
	grantErr    error
	grantCalls  []struct {
		currentUserID meta.ID
		dto           appprofilelink.CreateProfileLinkDTO
	}

	listResult []*appprofilelink.ProfileLinkResult
	listErr    error
	listCalls  []struct {
		currentUserID meta.ID
		dto           appprofilelink.ListProfileLinksDTO
	}

	revokeResult *appprofilelink.ProfileLinkResult
	revokeErr    error
	revokeCalls  []struct {
		currentUserID meta.ID
		dto           appprofilelink.RevokeProfileLinkBySelectorDTO
	}
}

func (s *profileLinkAccessStub) Grant(_ context.Context, currentUserID meta.ID, dto appprofilelink.CreateProfileLinkDTO) (*appprofilelink.ProfileLinkResult, error) {
	s.grantCalls = append(s.grantCalls, struct {
		currentUserID meta.ID
		dto           appprofilelink.CreateProfileLinkDTO
	}{currentUserID: currentUserID, dto: dto})
	return s.grantResult, s.grantErr
}

func (s *profileLinkAccessStub) List(_ context.Context, currentUserID meta.ID, dto appprofilelink.ListProfileLinksDTO) ([]*appprofilelink.ProfileLinkResult, error) {
	s.listCalls = append(s.listCalls, struct {
		currentUserID meta.ID
		dto           appprofilelink.ListProfileLinksDTO
	}{currentUserID: currentUserID, dto: dto})
	return s.listResult, s.listErr
}

func (s *profileLinkAccessStub) Revoke(_ context.Context, currentUserID meta.ID, dto appprofilelink.RevokeProfileLinkBySelectorDTO) (*appprofilelink.ProfileLinkResult, error) {
	s.revokeCalls = append(s.revokeCalls, struct {
		currentUserID meta.ID
		dto           appprofilelink.RevokeProfileLinkBySelectorDTO
	}{currentUserID: currentUserID, dto: dto})
	return s.revokeResult, s.revokeErr
}
