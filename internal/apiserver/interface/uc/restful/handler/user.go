package handler

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/log"
	appuser "github.com/FangcunMount/iam/internal/apiserver/application/uc/user"
	requestdto "github.com/FangcunMount/iam/internal/apiserver/interface/uc/restful/request"
	responsedto "github.com/FangcunMount/iam/internal/apiserver/interface/uc/restful/response"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/middleware/authn"
	_ "github.com/FangcunMount/iam/pkg/core" // imported for swagger
	"github.com/FangcunMount/iam/pkg/tenant"
)

// UserHandler 基础用户 REST 处理器
type UserHandler struct {
	*BaseHandler
	userApp    appuser.UserApplicationService
	profileApp appuser.UserProfileApplicationService
	userQuery  appuser.UserQueryApplicationService
	casbin     authn.CasbinEnforcer
}

// NewUserHandler 创建用户处理器。casbin 可为 nil，此时 /identity/me 不返回 roles。
func NewUserHandler(
	userApp appuser.UserApplicationService,
	profileApp appuser.UserProfileApplicationService,
	userQuery appuser.UserQueryApplicationService,
	casbin authn.CasbinEnforcer,
) *UserHandler {
	return &UserHandler{
		BaseHandler: NewBaseHandler(),
		userApp:     userApp,
		profileApp:  profileApp,
		userQuery:   userQuery,
		casbin:      casbin,
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
// @Router /identity/me [get]
// @Security BearerAuth
func (h *UserHandler) GetUserProfile(c *gin.Context) {
	rawUserID, exists := c.Get("user_id")
	if !exists {
		h.ErrorWithCode(c, code.ErrTokenInvalid, "user id not found in context")
		return
	}

	userID, err := toUserID(rawUserID)
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
// @Router /identity/me [patch]
// @Security BearerAuth
func (h *UserHandler) PatchUser(c *gin.Context) {
	rawUserID, exists := c.Get("user_id")
	if !exists {
		h.ErrorWithCode(c, code.ErrTokenInvalid, "user id not found in context")
		return
	}

	userID, err := toUserID(rawUserID)
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

func (h *UserHandler) resolveRoles(c *gin.Context, userID string) []string {
	if h.casbin == nil || strings.TrimSpace(userID) == "" {
		return nil
	}
	sub := "user:" + userID
	domains := []string{authn.TenantIDFromGin(c)}
	if domains[0] != tenant.PlatformID {
		domains = append(domains, tenant.PlatformID)
	}

	seen := make(map[string]struct{}, 4)
	out := make([]string, 0, 4)
	for idx, dom := range domains {
		raw, err := h.casbin.GetRolesForUser(c.Request.Context(), sub, dom)
		if err != nil {
			log.Debugw("me: GetRolesForUser failed", "sub", sub, "domain", dom, "error", err)
			if idx == 0 {
				return nil
			}
			continue
		}
		if len(raw) == 0 {
			if idx == 0 {
				log.Debugw("me: no casbin roles", "sub", sub, "domain", dom)
			}
			continue
		}
		for _, r := range raw {
			r = authn.NormalizeRoleName(r)
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
		Nickname: u.Name,
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

func parseUserID(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", perrors.WithCode(code.ErrInvalidArgument, "user id cannot be empty")
	}

	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return "", perrors.WithCode(code.ErrInvalidArgument, "invalid user id: %s", raw)
	}

	return strconv.FormatUint(id, 10), nil
}

func toUserID(value interface{}) (string, error) {
	switch v := value.(type) {
	case uint64:
		return strconv.FormatUint(v, 10), nil
	case uint32:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint:
		return strconv.FormatUint(uint64(v), 10), nil
	case int64:
		if v < 0 {
			return "", perrors.WithCode(code.ErrInvalidArgument, "negative id: %d", v)
		}
		return strconv.FormatUint(uint64(v), 10), nil
	case int32:
		if v < 0 {
			return "", perrors.WithCode(code.ErrInvalidArgument, "negative id: %d", v)
		}
		return strconv.FormatUint(uint64(v), 10), nil
	case int:
		if v < 0 {
			return "", perrors.WithCode(code.ErrInvalidArgument, "negative id: %d", v)
		}
		return strconv.FormatUint(uint64(v), 10), nil
	case float64:
		if v < 0 {
			return "", perrors.WithCode(code.ErrInvalidArgument, "negative id: %f", v)
		}
		return strconv.FormatUint(uint64(v), 10), nil
	case string:
		return parseUserID(v)
	default:
		return "", perrors.WithCode(code.ErrInvalidArgument, "unsupported user id type: %T", value)
	}
}
