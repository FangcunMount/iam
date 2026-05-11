package casbin

import (
	"sync"

	"github.com/casbin/casbin/v2"
)

type enforcerHolder struct {
	enforcer *casbin.CachedEnforcer
	mu       sync.RWMutex
}

func newEnforcerHolder(enforcer *casbin.CachedEnforcer) *enforcerHolder {
	return &enforcerHolder{enforcer: enforcer}
}
