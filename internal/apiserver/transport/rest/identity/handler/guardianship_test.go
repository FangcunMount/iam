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

	appguard "github.com/FangcunMount/iam/internal/apiserver/application/uc/guardianship"
	"github.com/FangcunMount/iam/internal/pkg/code"
)

func TestGuardianshipHandlerGrantUsesCurrentUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	access := &guardianshipAccessStub{
		grantResult: &appguard.GuardianshipResult{
			ID:            1,
			UserID:        "100",
			ChildID:       "200",
			Relation:      "parent",
			EstablishedAt: time.Now().Format(time.RFC3339),
		},
	}
	handler := NewGuardianshipHandler(access)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/identity/guardians/grant", bytes.NewBufferString(`{"childId":"200","relation":"parent"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", "100")

	handler.Grant(c)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body=%s", recorder.Code, http.StatusCreated, recorder.Body.String())
	}
	if len(access.grantCalls) != 1 {
		t.Fatalf("GrantForCurrentUser calls = %d, want 1", len(access.grantCalls))
	}
	if access.grantCalls[0].currentUserID != "100" || access.grantCalls[0].dto.UserID != "" {
		t.Fatalf("GrantForCurrentUser call = %#v, want current user 100 with empty dto user", access.grantCalls[0])
	}
}

func TestGuardianshipHandlerGrantRejectsDifferentUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	access := &guardianshipAccessStub{
		grantErr: perrors.WithCode(code.ErrPermissionDenied, "cannot grant guardianship for another user"),
	}
	handler := NewGuardianshipHandler(access)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/identity/guardians/grant", bytes.NewBufferString(`{"userId":"999","childId":"200","relation":"parent"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", "100")

	handler.Grant(c)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
	if len(access.grantCalls) != 1 {
		t.Fatalf("GrantForCurrentUser calls = %d, want 1", len(access.grantCalls))
	}
}

func TestGuardianshipHandlerListDefaultsToCurrentUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	access := &guardianshipAccessStub{
		listResult: []*appguard.GuardianshipResult{{
			ID:            1,
			UserID:        "100",
			ChildID:       "200",
			Relation:      "parent",
			EstablishedAt: time.Now().Format(time.RFC3339),
		}},
	}
	handler := NewGuardianshipHandler(access)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/identity/guardians", nil)
	c.Set("user_id", "100")

	handler.List(c)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if len(access.listCalls) != 1 || access.listCalls[0].currentUserID != "100" {
		t.Fatalf("ListForCurrentUser calls = %#v, want current user 100", access.listCalls)
	}
}

func TestGuardianshipHandlerListRejectsCrossUserQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)

	access := &guardianshipAccessStub{
		listErr: perrors.WithCode(code.ErrPermissionDenied, "cannot query guardianships for another user"),
	}
	handler := NewGuardianshipHandler(access)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/identity/guardians?user_id=999", nil)
	c.Set("user_id", "100")

	handler.List(c)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
	if len(access.listCalls) != 1 {
		t.Fatalf("ListForCurrentUser calls = %d, want 1", len(access.listCalls))
	}
}

func TestGuardianshipHandlerListRejectsChildLookupForNonGuardian(t *testing.T) {
	gin.SetMode(gin.TestMode)

	access := &guardianshipAccessStub{
		listErr: perrors.WithCode(code.ErrPermissionDenied, "forbidden"),
	}
	handler := NewGuardianshipHandler(access)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/identity/guardians?child_id=200", nil)
	c.Set("user_id", "100")

	handler.List(c)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
	if len(access.listCalls) != 1 {
		t.Fatalf("ListForCurrentUser calls = %d, want 1", len(access.listCalls))
	}
}

type guardianshipAccessStub struct {
	grantResult *appguard.GuardianshipResult
	grantErr    error
	grantCalls  []struct {
		currentUserID string
		dto           appguard.AddGuardianDTO
	}

	listResult []*appguard.GuardianshipResult
	listErr    error
	listCalls  []struct {
		currentUserID string
		dto           appguard.ListGuardianshipsDTO
	}
}

func (s *guardianshipAccessStub) GrantForCurrentUser(_ context.Context, currentUserID string, dto appguard.AddGuardianDTO) (*appguard.GuardianshipResult, error) {
	s.grantCalls = append(s.grantCalls, struct {
		currentUserID string
		dto           appguard.AddGuardianDTO
	}{currentUserID: currentUserID, dto: dto})
	return s.grantResult, s.grantErr
}

func (s *guardianshipAccessStub) ListForCurrentUser(_ context.Context, currentUserID string, dto appguard.ListGuardianshipsDTO) ([]*appguard.GuardianshipResult, error) {
	s.listCalls = append(s.listCalls, struct {
		currentUserID string
		dto           appguard.ListGuardianshipsDTO
	}{currentUserID: currentUserID, dto: dto})
	return s.listResult, s.listErr
}

func (s *guardianshipAccessStub) RevokeBySelector(context.Context, appguard.RevokeGuardianBySelectorDTO) (*appguard.GuardianshipResult, error) {
	return nil, nil
}
