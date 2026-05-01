package credential

import "github.com/FangcunMount/iam/v2/internal/pkg/meta"

// ==================== 规格对象（Specification）====================
// 用于封装创建或修改凭据的业务规则参数

// BindSpec 凭据绑定规范
// 描述如何将认证凭据绑定到账号
type BindSpec struct {
	AccountID     meta.ID        // 账号ID
	Type          CredentialType // 凭据类型
	IDP           *string        // IDP类型："wechat"|"wecom"|"phone" | nil(本地)
	IDPIdentifier string         // IDP标识符：unionid | openid@appid | userid | +E164 | ""(password)
	AppID         *string        // 应用ID
	Material      []byte         // 凭据材料（仅 password）
	Algo          *string        // 算法（仅 password）
	ParamsJSON    []byte         // 参数JSON（低频元数据）
}

// RotateSpec 凭据轮换规范
// 描述如何轮换凭据材料（主要用于密码更新）
type RotateSpec struct {
	CredentialID meta.ID // 凭据ID
	NewMaterial  []byte  // 新的密钥材料
	NewAlgo      *string // 新的算法（可选）
}
