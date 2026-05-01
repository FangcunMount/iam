package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	cachegovernance "github.com/FangcunMount/iam/v2/internal/apiserver/application/cachegovernance"

	responsedto "github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest/cachegovernance/response"
)

// GovernanceHandler 提供内部只读缓存治理接口。
type GovernanceHandler struct {
	*BaseHandler
	service *cachegovernance.ReadService
}

// NewGovernanceHandler 创建内部缓存治理处理器。
func NewGovernanceHandler(service *cachegovernance.ReadService) *GovernanceHandler {
	return &GovernanceHandler{
		BaseHandler: NewBaseHandler(),
		service:     service,
	}
}

// GetCatalog 返回静态缓存目录。
func (h *GovernanceHandler) GetCatalog(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "cache governance service not initialized"})
		return
	}

	descriptors, err := h.service.Catalog(c.Request.Context())
	if err != nil {
		h.Error(c, err)
		return
	}
	h.Success(c, responsedto.FromCatalog(descriptors))
}

// GetOverview 返回缓存治理总览。
func (h *GovernanceHandler) GetOverview(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "cache governance service not initialized"})
		return
	}

	overview, err := h.service.Overview(c.Request.Context())
	if err != nil {
		h.Error(c, err)
		return
	}
	h.Success(c, responsedto.FromOverview(overview))
}

// GetFamily 返回单个缓存族的治理视图。
func (h *GovernanceHandler) GetFamily(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "cache governance service not initialized"})
		return
	}

	family := cachegovernance.Family(c.Param("family"))
	view, err := h.service.Family(c.Request.Context(), family)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":  err.Error(),
			"family": family,
		})
		return
	}
	h.Success(c, responsedto.FromFamilyView(view))
}
