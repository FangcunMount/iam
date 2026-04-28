package handler

import (
	"github.com/FangcunMount/iam/pkg/core"
)

// BaseHandler 继承公共的 BaseHandler
type BaseHandler struct {
	*core.BaseHandler
}

// NewBaseHandler 构造基础处理器
func NewBaseHandler() *BaseHandler {
	return &BaseHandler{
		BaseHandler: core.NewBaseHandler(),
	}
}
