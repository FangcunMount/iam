package requestctx

import "github.com/gin-gonic/gin"

func SetClaims(c *gin.Context, claims any) {
	if c == nil || claims == nil {
		return
	}
	c.Set(KeyClaims, claims)
}

func Claims(c *gin.Context) (any, bool) {
	if c == nil {
		return nil, false
	}
	claims, exists := c.Get(KeyClaims)
	if !exists || claims == nil {
		return nil, false
	}
	return claims, true
}
