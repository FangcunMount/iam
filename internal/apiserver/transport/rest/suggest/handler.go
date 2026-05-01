package suggest

import (
	"github.com/gin-gonic/gin"

	appsuggest "github.com/FangcunMount/iam/v2/internal/apiserver/application/suggest"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/suggest"
	"github.com/FangcunMount/iam/v2/pkg/core"
)

// Dependencies wires runtime dependencies for the handler.
type Dependencies struct {
	Service        appsuggest.ProfileSuggestor
	AuthMiddleware gin.HandlerFunc
}

// Register registers routes onto the engine.
func Register(engine *gin.Engine, deps Dependencies) {
	if engine == nil || deps.Service == nil || deps.AuthMiddleware == nil {
		return
	}

	group := engine.Group("/api/v2/suggest")
	group.Use(deps.AuthMiddleware)

	h := NewHandler(deps.Service)
	group.GET("/profile", h.Profile)
}

// Handler 提供 suggest 接口
type Handler struct {
	*core.BaseHandler
	svc appsuggest.ProfileSuggestor
}

// NewHandler creates a suggest handler.
func NewHandler(svc appsuggest.ProfileSuggestor) *Handler {
	return &Handler{
		BaseHandler: core.NewBaseHandler(),
		svc:         svc,
	}
}

// Profile 处理档案联想查询
// @Summary 档案联想搜索
// @Description 支持中文/拼音前缀联想，数字关键词走手机号/ID 精确匹配
// @Tags Suggest
// @Accept  json
// @Produce  json
// @Param k query string true "关键词；数字=精确匹配手机号/ID，其他=前缀联想"
// @Success 200 {array} suggest.Term "联想结果（按权重降序，去重）"
// @Failure 400 {object} core.ErrResponse "参数缺失"
// @Router /suggest/profile [get]
// @Security BearerAuth
func (h *Handler) Profile(c *gin.Context) {
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
