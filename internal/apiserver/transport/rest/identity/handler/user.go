package handler

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/log"
	appuser "github.com/FangcunMount/iam/v3/internal/apiserver/application/identity/user"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/subject"
	requestdto "github.com/FangcunMount/iam/v3/internal/apiserver/transport/rest/identity/request"
	responsedto "github.com/FangcunMount/iam/v3/internal/apiserver/transport/rest/identity/response"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/FangcunMount/iam/v3/internal/pkg/requestctx"
	"github.com/FangcunMount/iam/v3/pkg/core"
	"github.com/FangcunMount/iam/v3/pkg/tenant"
)

var _ = core.ErrResponse{}

type EffectiveRoleReader interface {
	EffectiveRoleNamesForSubject(ctx context.Context, subject subject.Ref, tenantID string) ([]string, error)
}

// UserHandler 基础用户 REST 处理器
type UserHandler struct {
	*BaseHandler
	userApp        appuser.Creator
	profileApp     appuser.Editor
	userQuery      appuser.Directory
	effectiveRoles EffectiveRoleReader
}

// NewUserHandler 创建用户处理器。角色读取器可为 nil，此时 /identity/me 不返回 roles。
func NewUserHandler(
	userApp appuser.Creator,
	profileApp appuser.Editor,
	userQuery appuser.Directory,
	roles EffectiveRoleReader,
) *UserHandler {
	return &UserHandler{
		BaseHandler:    NewBaseHandler(),
		userApp:        userApp,
		profileApp:     profileApp,
		userQuery:      userQuery,
		effectiveRoles: roles,
	}
}

// GetUserProfile 获取当前用户资料
// @Summary 获取当前用户信息
// @Description 获取当前登录用户的资料信息
// @Tags Identity-Users
// @Accept json
// @Produce json
// @Success 200 {object} responsedto.UserResponse "查询成功"
// @Failure 401 {object} core.ErrResponse "未授权"
// @Failure 500 {object} core.ErrResponse "服务器内部错误"
// @Router /v2/identity/me [get]
// @Security BearerAuth
func (h *UserHandler) GetUserProfile(c *gin.Context) {
	userID, err := requestctx.RequiredUserID(c)
	if err != nil {
		h.Error(c, err)
		return
	}

	u, err := h.userQuery.GetByID(c.Request.Context(), userID)
	if err != nil {
		h.Error(c, err)
		return
	}

	h.Success(c, newUserResponse(u, h.resolveRoles(c, userID)))
}

// PatchUser 更新用户信息（昵称 / 联系方式）
// @Summary 更新当前用户信息
// @Description 部分更新当前登录用户的信息，支持更新昵称和联系方式
// @Tags Identity-Users
// @Accept json
// @Produce json
// @Param request body requestdto.UserUpdateRequest true "更新用户请求"
// @Success 200 {object} responsedto.UserResponse "更新成功"
// @Failure 400 {object} core.ErrResponse "参数错误"
// @Failure 401 {object} core.ErrResponse "未授权"
// @Failure 500 {object} core.ErrResponse "服务器内部错误"
// @Router /v2/identity/me [patch]
// @Security BearerAuth
func (h *UserHandler) PatchUser(c *gin.Context) {
	userID, err := requestctx.RequiredUserID(c)
	if err != nil {
		h.Error(c, err)
		return
	}

	var req requestdto.UserUpdateRequest
	if err := h.BindJSON(c, &req); err != nil {
		h.Error(c, err)
		return
	}

	ctx := c.Request.Context()

	var phonePtr *string
	var emailPtr *string
	if len(req.Contacts) > 0 {
		phoneValue, emailValue := extractContactValues(req.Contacts)
		if phoneValue != "" {
			phonePtr = &phoneValue
		}
		if emailValue != "" {
			emailPtr = &emailValue
		}
	}

	u, err := h.profileApp.PatchProfile(ctx, appuser.PatchUserProfileDTO{
		UserID:   userID,
		Nickname: req.Nickname,
		Phone:    phonePtr,
		Email:    emailPtr,
	})
	if err != nil {
		h.Error(c, err)
		return
	}

	h.Success(c, newUserResponse(u, h.resolveRoles(c, userID)))
}

func extractContactValues(contacts []requestdto.UserContactUpsert) (phone string, email string) {
	for _, contact := range contacts {
		switch strings.ToLower(contact.Type) {
		case "phone":
			if phone == "" {
				phone = strings.TrimSpace(contact.Value)
			}
		case "email":
			if email == "" {
				email = strings.TrimSpace(contact.Value)
			}
		}
	}
	return
}

func (h *UserHandler) resolveRoles(c *gin.Context, userID meta.ID) []string {
	if h.effectiveRoles == nil || userID.IsZero() {
		return nil
	}
	subjectRef, err := subject.NewUserRef(userID)
	if err != nil {
		return nil
	}
	domains := []string{requestctx.TenantIDOrDefault(c)}
	if domains[0] != tenant.PlatformID {
		domains = append(domains, tenant.PlatformID)
	}

	seen := make(map[string]struct{}, 4)
	out := make([]string, 0, 4)
	for idx, dom := range domains {
		raw, err := h.effectiveRoles.EffectiveRoleNamesForSubject(c.Request.Context(), subjectRef, dom)
		if err != nil {
			log.Debugw("me: role name lookup failed", "subject_type", string(subjectRef.Type), "subject_id", subjectRef.ID, "domain", dom, "error", err)
			if idx == 0 {
				return nil
			}
			continue
		}
		if len(raw) == 0 {
			if idx == 0 {
				log.Debugw("me: no roles", "subject_type", string(subjectRef.Type), "subject_id", subjectRef.ID, "domain", dom)
			}
			continue
		}
		for _, r := range raw {
			r = strings.TrimSpace(r)
			if r == "" {
				continue
			}
			if _, exists := seen[r]; exists {
				continue
			}
			seen[r] = struct{}{}
			out = append(out, r)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func newUserResponse(u *appuser.UserResult, roles []string) responsedto.UserResponse {
	if u == nil {
		return responsedto.UserResponse{}
	}

	resp := responsedto.UserResponse{
		ID:       u.ID,
		Status:   u.Status.String(),
		Nickname: userDisplayNickname(u),
		Roles:    roles,
	}

	if strings.TrimSpace(u.Phone) != "" {
		resp.Contacts = append(resp.Contacts, responsedto.VerifiedContactResponse{
			Type:  "phone",
			Value: strings.TrimSpace(u.Phone),
		})
	}

	if strings.TrimSpace(u.Email) != "" {
		resp.Contacts = append(resp.Contacts, responsedto.VerifiedContactResponse{
			Type:  "email",
			Value: strings.TrimSpace(u.Email),
		})
	}

	return resp
}

func userDisplayNickname(u *appuser.UserResult) string {
	if u == nil {
		return ""
	}
	if strings.TrimSpace(u.Nickname) != "" {
		return u.Nickname
	}
	return u.Name
}

func parseProfileID(raw string) (meta.ID, error) {
	return parseRequiredID(raw, "profile id")
}

func parseRequiredID(raw string, field string) (meta.ID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, perrors.WithCode(code.ErrInvalidArgument, "%s cannot be empty", field)
	}

	id, err := meta.ParseID(raw)
	if err != nil {
		return 0, perrors.WithCode(code.ErrInvalidArgument, "invalid %s: %s", field, raw)
	}

	return id, nil
}

func parseOptionalID(raw string, field string) (meta.ID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	return parseRequiredID(raw, field)
}
