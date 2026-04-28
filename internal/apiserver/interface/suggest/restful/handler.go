package restful

import (
	"github.com/gin-gonic/gin"

	appsuggest "github.com/FangcunMount/iam/internal/apiserver/application/suggest"
	"github.com/FangcunMount/iam/internal/apiserver/domain/suggest"
	"github.com/FangcunMount/iam/pkg/core"
)

// Dependencies wires runtime dependencies for the handler.
type Dependencies struct {
	Service        *appsuggest.Service
	AuthMiddleware gin.HandlerFunc
}

var deps Dependencies

// Provide sets dependencies before Register is called.
func Provide(d Dependencies) {
	deps = d
}

// Register registers routes onto the engine.
func Register(engine *gin.Engine) {
	if engine == nil || deps.Service == nil || deps.AuthMiddleware == nil {
		return
	}

	group := engine.Group("/api/v1/suggest")
	group.Use(deps.AuthMiddleware)

	h := NewHandler(deps.Service)
	group.GET("/child", h.Child)
}

// Handler 提供 suggest 接口
type Handler struct {
	*core.BaseHandler
	svc *appsuggest.Service
}

// NewHandler creates a suggest handler.
func NewHandler(svc *appsuggest.Service) *Handler {
	return &Handler{
		BaseHandler: core.NewBaseHandler(),
		svc:         svc,
	}
}

// Child 处理儿童联想查询
// @Summary 儿童联想搜索
// @Description 支持中文/拼音前缀联想，数字关键词走手机号/ID 精确匹配
// @Tags Suggest
// @Accept  json
// @Produce  json
// @Param k query string true "关键词；数字=精确匹配手机号/ID，其他=前缀联想"
// @Success 200 {array} suggest.Term "联想结果（按权重降序，去重）"
// @Failure 400 {object} core.ErrResponse "参数缺失"
// @Router /suggest/child [get]
// @Security BearerAuth
func (h *Handler) Child(c *gin.Context) {
	var query struct {
		K string `form:"k" binding:"required"`
	}
	if err := h.BindQuery(c, &query); err != nil {
		return
	}

	list := h.svc.Suggest(c, query.K)
	if list == nil {
		list = []suggest.Term{}
	}

	h.Success(c, list)
}
