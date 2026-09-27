package rechargeportalhttp

import "github.com/gin-gonic/gin"

func RegisterRoutes(group gin.IRoutes, handler *Handler) {
	if group == nil || handler == nil {
		panic("recharge portal routes: required dependency is nil")
	}
	group.POST("/preview", handler.Preview)
	group.POST("/preflight", handler.Preflight)
	group.POST("/redeem", handler.Redeem)
	group.GET("/result", handler.Result)
}
