package suggest

import (
	stderrors "errors"

	pkgerrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/gin-gonic/gin"

	appsuggest "github.com/FangcunMount/iam/v2/internal/apiserver/application/suggest"
	domainsuggest "github.com/FangcunMount/iam/v2/internal/apiserver/domain/suggest"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/pkg/core"
)

// Dependencies wires runtime dependencies for the handler.
type Dependencies struct {
	Service     appsuggest.ProfileSuggestor
	Middlewares []gin.HandlerFunc
	RateLimit   appsuggest.RateLimitConfig
}

// Register registers routes onto the engine.
func Register(engine *gin.Engine, deps Dependencies) {
	if engine == nil || deps.Service == nil || len(deps.Middlewares) == 0 {
		return
	}

	group := engine.Group("/api/v2/suggest")
	group.Use(deps.Middlewares...)

	lim := NewPerOperatorRateLimiterFromConfig(deps.RateLimit)
	h := NewHandler(deps.Service, lim)
	group.GET("/profile", h.Profile)
}

// Handler 提供 suggest 接口
type Handler struct {
	*core.BaseHandler
	svc    appsuggest.ProfileSuggestor
	limits *PerOperatorRateLimiter
}

// NewHandler creates a suggest handler.
func NewHandler(svc appsuggest.ProfileSuggestor, limits *PerOperatorRateLimiter) *Handler {
	return &Handler{
		BaseHandler: core.NewBaseHandler(),
		svc:         svc,
		limits:      limits,
	}
}

// Profile 处理档案联想查询
// @Summary 档案联想搜索
// @Description 基于索引召回并按当前用户数据权限过滤；数字关键词支持档案 ID，手机号搜索需额外授权
// @Tags Suggest
// @Accept  json
// @Produce  json
// @Param k query string true "关键词；纯数字可为档案 ID 或手机号（手机号需授权）"
// @Param limit           query    int  false "返回条数上限"
// @Success 200 {array} ProfileSuggestResponseItem "联想结果（按权重降序，去重）"
// @Failure 400 {object} core.ErrResponse "参数缺失"
// @Failure 401 {object} core.ErrResponse "未认证"
// @Failure 403 {object} core.ErrResponse "无搜索权限"
// @Failure 429 {object} core.ErrResponse "请求过于频繁"
// @Router /suggest/profile [get]
// @Security BearerAuth
func (h *Handler) Profile(c *gin.Context) {
	var query struct {
		K     string `form:"k" binding:"required"`
		Limit int    `form:"limit"`
	}
	if err := h.BindQuery(c, &query); err != nil {
		return
	}

	principal, ok := OperatingPrincipalFromGin(c)
	if !ok {
		h.UnauthorizedResponse(c, "missing operating principal")
		return
	}

	if h.limits != nil {
		kw := domainsuggest.NewKeyword(query.K)
		mobile := kw.IsDigits() && domainsuggest.LooksLikeMobile(kw.String())
		if !h.limits.Allow(principal.OperatorID, mobile) {
			h.Error(c, pkgerrors.WithCode(code.ErrRateLimited, "%s", "suggest rate limit exceeded"))
			return
		}
	}

	list, err := h.svc.SuggestProfile(c, appsuggest.SuggestProfileRequest{
		Principal: principal,
		Keyword:   query.K,
		Limit:     query.Limit,
	})
	if err != nil {
		if stderrors.Is(err, appsuggest.ErrUnauthenticated) {
			h.UnauthorizedResponse(c, "unauthenticated operating principal")
			return
		}
		h.Error(c, err)
		return
	}
	if list == nil {
		list = []appsuggest.ProfileSuggestItem{}
	}

	h.Success(c, toProfileSuggestResponseItems(list))
}
