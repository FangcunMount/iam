package cache

import cachegovernance "github.com/FangcunMount/iam/internal/apiserver/application/cachegovernance"

// Families 返回当前 IAM 缓存目录快照。
func Families() []FamilyDescriptor {
	return cachegovernance.Families()
}

// GetFamily 返回指定 family 的静态描述。
func GetFamily(family Family) (FamilyDescriptor, bool) {
	return cachegovernance.GetFamily(family)
}
