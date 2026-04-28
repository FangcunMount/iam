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

	app := &guardianshipAppStub{}
	query := &guardianshipQueryStub{
		getByUserAndChild: &appguard.GuardianshipResult{
			ID:            1,
			UserID:        "100",
			ChildID:       "200",
			Relation:      "parent",
			EstablishedAt: time.Now().Format(time.RFC3339),
		},
	}
	handler := NewGuardianshipHandler(app, query)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/identity/guardians/grant", bytes.NewBufferString(`{"childId":"200","relation":"parent"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", "100")

	handler.Grant(c)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body=%s", recorder.Code, http.StatusCreated, recorder.Body.String())
	}
	if len(app.addCalls) != 1 {
		t.Fatalf("AddGuardian calls = %d, want 1", len(app.addCalls))
	}
	if app.addCalls[0].UserID != "100" {
		t.Fatalf("AddGuardian user_id = %s, want 100", app.addCalls[0].UserID)
	}
}

func TestGuardianshipHandlerGrantRejectsDifferentUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	app := &guardianshipAppStub{}
	handler := NewGuardianshipHandler(app, &guardianshipQueryStub{})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/identity/guardians/grant", bytes.NewBufferString(`{"userId":"999","childId":"200","relation":"parent"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", "100")

	handler.Grant(c)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
	if len(app.addCalls) != 0 {
		t.Fatalf("AddGuardian should not be called")
	}
}

func TestGuardianshipHandlerListDefaultsToCurrentUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	query := &guardianshipQueryStub{
		listChildrenResult: []*appguard.GuardianshipResult{{
			ID:            1,
			UserID:        "100",
			ChildID:       "200",
			Relation:      "parent",
			EstablishedAt: time.Now().Format(time.RFC3339),
		}},
	}
	handler := NewGuardianshipHandler(&guardianshipAppStub{}, query)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/identity/guardians", nil)
	c.Set("user_id", "100")

	handler.List(c)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if len(query.listChildrenCalls) != 1 || query.listChildrenCalls[0] != "100" {
		t.Fatalf("ListChildrenByUserID calls = %#v, want [100]", query.listChildrenCalls)
	}
}

func TestGuardianshipHandlerListRejectsCrossUserQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)

	query := &guardianshipQueryStub{}
	handler := NewGuardianshipHandler(&guardianshipAppStub{}, query)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/identity/guardians?user_id=999", nil)
	c.Set("user_id", "100")

	handler.List(c)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
	if len(query.listChildrenCalls) != 0 {
		t.Fatalf("query service should not be called")
	}
}

func TestGuardianshipHandlerListRejectsChildLookupForNonGuardian(t *testing.T) {
	gin.SetMode(gin.TestMode)

	query := &guardianshipQueryStub{
		getByUserAndChildErr: perrors.WithCode(code.ErrPermissionDenied, "forbidden"),
	}
	handler := NewGuardianshipHandler(&guardianshipAppStub{}, query)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/identity/guardians?child_id=200", nil)
	c.Set("user_id", "100")

	handler.List(c)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
	if len(query.listGuardiansCalls) != 0 {
		t.Fatalf("ListGuardiansByChildID should not be called")
	}
}

type guardianshipAppStub struct {
	addCalls []appguard.AddGuardianDTO
}

func (s *guardianshipAppStub) AddGuardian(_ context.Context, dto appguard.AddGuardianDTO) error {
	s.addCalls = append(s.addCalls, dto)
	return nil
}

func (s *guardianshipAppStub) RemoveGuardian(context.Context, appguard.RemoveGuardianDTO) error {
	return nil
}

type guardianshipQueryStub struct {
	getByUserAndChild     *appguard.GuardianshipResult
	getByUserAndChildErr  error
	listChildrenResult    []*appguard.GuardianshipResult
	listChildrenCalls     []string
	listGuardiansResult   []*appguard.GuardianshipResult
	listGuardiansCalls    []string
	listWithRevokedCalls  []string
	guardWithRevokedCalls []string
}

func (s *guardianshipQueryStub) IsGuardian(context.Context, string, string) (bool, error) {
	return false, nil
}

func (s *guardianshipQueryStub) GetByUserIDAndChildID(context.Context, string, string) (*appguard.GuardianshipResult, error) {
	return s.getByUserAndChild, s.getByUserAndChildErr
}

func (s *guardianshipQueryStub) GetByUserIDAndChildIDIncludingRevoked(context.Context, string, string) (*appguard.GuardianshipResult, error) {
	return s.getByUserAndChild, s.getByUserAndChildErr
}

func (s *guardianshipQueryStub) ListChildrenByUserID(_ context.Context, userID string) ([]*appguard.GuardianshipResult, error) {
	s.listChildrenCalls = append(s.listChildrenCalls, userID)
	return s.listChildrenResult, nil
}

func (s *guardianshipQueryStub) ListChildrenByUserIDIncludingRevoked(_ context.Context, userID string) ([]*appguard.GuardianshipResult, error) {
	s.listWithRevokedCalls = append(s.listWithRevokedCalls, userID)
	return s.listChildrenResult, nil
}

func (s *guardianshipQueryStub) ListGuardiansByChildID(_ context.Context, childID string) ([]*appguard.GuardianshipResult, error) {
	s.listGuardiansCalls = append(s.listGuardiansCalls, childID)
	return s.listGuardiansResult, nil
}

func (s *guardianshipQueryStub) ListGuardiansByChildIDIncludingRevoked(_ context.Context, childID string) ([]*appguard.GuardianshipResult, error) {
	s.guardWithRevokedCalls = append(s.guardWithRevokedCalls, childID)
	return s.listGuardiansResult, nil
}
