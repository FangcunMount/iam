package request

import (
	"encoding/json"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// SignUpWithWeChatMiniProgramRequest 微信小程序账号开通请求。
// 该接口会原子性地创建 User + Account + WeChatAccount。
type SignUpWithWeChatMiniProgramRequest struct {
	Name     string                 `json:"name" binding:"required"`   // 用户名
	Phone    string                 `json:"phone,omitempty"`           // 手机号（可选）
	Email    string                 `json:"email,omitempty"`           // 邮箱（可选）
	AppID    string                 `json:"appId" binding:"required"`  // 微信应用ID
	JsCode   string                 `json:"jsCode" binding:"required"` // 微信登录凭证（临时登录凭证）
	Nickname *string                `json:"nickname,omitempty"`        // 微信昵称（可选）
	Avatar   *string                `json:"avatar,omitempty"`          // 微信头像（可选）
	Meta     map[string]interface{} `json:"meta,omitempty"`            // 微信元数据（可选）
}

// EnsureMockConsumerReq 内部 mock C 端账户 ensure 请求。
type EnsureMockConsumerReq struct {
	Name     string            `json:"name" binding:"required"`
	Phone    string            `json:"phone" binding:"required"`
	Email    string            `json:"email" binding:"required"`
	Password string            `json:"password" binding:"required"`
	Profile  map[string]string `json:"profile,omitempty"`
	Meta     map[string]string `json:"meta,omitempty"`
}

func (r *EnsureMockConsumerReq) Validate() error {
	if strings.TrimSpace(r.Name) == "" {
		return perrors.WithCode(code.ErrInvalidArgument, "name is required")
	}
	if strings.TrimSpace(r.Phone) == "" {
		return perrors.WithCode(code.ErrInvalidArgument, "phone is required")
	}
	if strings.TrimSpace(r.Email) == "" {
		return perrors.WithCode(code.ErrInvalidArgument, "email is required")
	}
	if strings.TrimSpace(r.Password) == "" {
		return perrors.WithCode(code.ErrInvalidArgument, "password is required")
	}
	return nil
}

// Validate 保留方法以兼容现有代码，但实际验证由 binding 标签处理。
func (r *SignUpWithWeChatMiniProgramRequest) Validate() error {
	// binding 标签已经处理了基本验证
	// 这里可以添加额外的业务逻辑验证（如果需要）
	return nil
}

// MetaJSON encodes signup meta.
func (r *SignUpWithWeChatMiniProgramRequest) MetaJSON() (map[string]string, error) {
	if r.Meta == nil {
		return nil, nil
	}
	result := make(map[string]string, len(r.Meta))
	for k, v := range r.Meta {
		if str, ok := v.(string); ok {
			result[k] = str
		}
	}
	return result, nil
}

// UpsertWeChatProfileReq payload.
type UpsertWeChatProfileReq struct {
	Nickname *string                `json:"nickname,omitempty"`
	Avatar   *string                `json:"avatar,omitempty"`
	Meta     map[string]interface{} `json:"meta,omitempty"`
}

func (r *UpsertWeChatProfileReq) Validate() error {
	if (r.Nickname == nil || strings.TrimSpace(*r.Nickname) == "") &&
		(r.Avatar == nil || strings.TrimSpace(*r.Avatar) == "") &&
		len(r.Meta) == 0 {
		return perrors.WithCode(code.ErrInvalidArgument, "at least one field must be provided")
	}
	return nil
}

// SetWeChatUnionIDReq payload.
type SetWeChatUnionIDReq struct {
	UnionID string `json:"unionId"`
}

func (r *SetWeChatUnionIDReq) Validate() error {
	if strings.TrimSpace(r.UnionID) == "" {
		return perrors.WithCode(code.ErrInvalidArgument, "unionId cannot be empty")
	}
	return nil
}

func encodeMapToJSON(m map[string]interface{}) ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	if len(m) == 0 {
		return []byte("{}"), nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "invalid json object")
	}
	return b, nil
}

// MetaJSON encodes UpsertWeChatProfile meta.
func (r *UpsertWeChatProfileReq) MetaJSON() ([]byte, error) {
	return encodeMapToJSON(r.Meta)
}
