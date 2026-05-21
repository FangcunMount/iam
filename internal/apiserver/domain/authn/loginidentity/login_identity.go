package loginidentity

import (
	"strings"
	"time"

	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// LoginIdentity 绑定 IAM 用户到具体的登录标识
type LoginIdentity struct {
	// —— 基础信息 —— //
	ID     meta.ID // 登录身份ID
	UserID meta.ID // 用户ID

	// —— 身份信息 —— //
	Provider         Provider // 提供者
	Realm            string   // 域
	Identifier       string   // 标识
	GlobalIdentifier string   // 全局标识

	// —— 状态信息 —— //
	Status     Status            // 状态
	VerifiedAt *time.Time        // 验证时间
	LinkedAt   time.Time         // 绑定时间
	Profile    map[string]string // 资料
	Meta       map[string]string // 元数据
	CreatedAt  time.Time         // 创建时间
	UpdatedAt  time.Time         // 更新时间
}

// UniqueKey 唯一键
func (i *LoginIdentity) UniqueKey() (Provider, string, string) {
	if i == nil {
		return "", "", ""
	}
	return i.Provider, i.Realm, i.Identifier
}

// IsActive 是否活动
func (i *LoginIdentity) IsActive() bool {
	return i != nil && i.Status == StatusActive
}

// VerifyAt 验证时间
func (i *LoginIdentity) VerifyAt(t time.Time) {
	i.VerifiedAt = &t
}

// Activate 激活
func (i *LoginIdentity) Activate() { i.Status = StatusActive }

// Disable 禁用
func (i *LoginIdentity) Disable() { i.Status = StatusDisabled }

// normalizeRealm 规范化域
func normalizeRealm(realm string) string {
	realm = strings.TrimSpace(realm)
	if realm == "" {
		return RealmDefault
	}
	return realm
}
