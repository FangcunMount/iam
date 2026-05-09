package credential

import "github.com/FangcunMount/iam/v2/internal/pkg/meta"

// ==================== 规格对象（Specification）====================
// 用于封装创建或修改凭据的业务规则参数

// BindSpec 凭据绑定规范。
type BindSpec struct {
	LoginIdentityID meta.ID        // 登录身份ID（新模型）
	Type            CredentialType // 凭据类型
	Material        []byte         // 凭据材料（仅 password）
	Algo            *string        // 算法（仅 password）
	ParamsJSON      []byte         // 参数JSON（低频元数据）
}

// RotateSpec 凭据轮换规范
// 描述如何轮换凭据材料（主要用于密码更新）
type RotateSpec struct {
	CredentialID meta.ID // 凭据ID
	NewMaterial  []byte  // 新的密钥材料
	NewAlgo      *string // 新的算法（可选）
}
