package handler

import (
	"github.com/gin-gonic/gin"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	sessionapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/session"
	resp "github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest/authn/response"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/requestctx"
)

// SessionAdminHandler 暴露管理员会话控制接口。
type SessionAdminHandler struct {
	*BaseHandler
	service sessionapp.SessionApplicationService
}

// NewSessionAdminHandler 创建管理员会话处理器。
func NewSessionAdminHandler(service sessionapp.SessionApplicationService) *SessionAdminHandler {
	return &SessionAdminHandler{
		BaseHandler: NewBaseHandler(),
		service:     service,
	}
}

// RevokeSession 撤销单个会话。
func (h *SessionAdminHandler) RevokeSession(c *gin.Context) {
	if h == nil || h.service == nil {
		h.Error(c, perrors.WithCode(code.ErrInternalServerError, "session service not initialized"))
		return
	}
	sessionID := c.Param("sessionId")
	if err := h.service.RevokeSession(c.Request.Context(), sessionID, c.Query("reason"), currentActor(c)); err != nil {
		h.Error(c, err)
		return
	}
	h.Success(c, resp.MessageResponse{Message: "session revoked"})
}

// RevokeLoginIdentitySessions 撤销某登录身份全部会话。
func (h *SessionAdminHandler) RevokeLoginIdentitySessions(c *gin.Context) {
	if h == nil || h.service == nil {
		h.Error(c, perrors.WithCode(code.ErrInternalServerError, "session service not initialized"))
		return
	}
	loginIdentityID := c.Param("loginIdentityId")
	if err := h.service.RevokeAllSessionsByLoginIdentity(c.Request.Context(), loginIdentityID, c.Query("reason"), currentActor(c)); err != nil {
		h.Error(c, err)
		return
	}
	h.Success(c, resp.MessageResponse{Message: "login identity sessions revoked"})
}

// RevokeUserSessions 撤销某用户全部会话。
func (h *SessionAdminHandler) RevokeUserSessions(c *gin.Context) {
	if h == nil || h.service == nil {
		h.Error(c, perrors.WithCode(code.ErrInternalServerError, "session service not initialized"))
		return
	}
	userID := c.Param("userId")
	if err := h.service.RevokeAllSessionsByUser(c.Request.Context(), userID, c.Query("reason"), currentActor(c)); err != nil {
		h.Error(c, err)
		return
	}
	h.Success(c, resp.MessageResponse{Message: "user sessions revoked"})
}

func currentActor(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if actor, ok := requestctx.LoginIdentityID(c); ok {
		return actor.String()
	}
	if actor, ok := requestctx.UserID(c); ok {
		return actor.String()
	}
	return "admin"
}
