package loginidentity

// Provider names the source namespace used to find a login identity.
type Provider string

const (
	ProviderUsername    Provider = "username"
	ProviderPhone       Provider = "phone"
	ProviderWechatMinip Provider = "wechat_minip"
	ProviderWecom       Provider = "wecom"
)

const (
	RealmDefault = "default"
	RealmGlobal  = "global"
)

func (p Provider) String() string { return string(p) }

func (p Provider) Validate() bool {
	switch p {
	case ProviderUsername, ProviderPhone, ProviderWechatMinip, ProviderWecom:
		return true
	default:
		return false
	}
}
