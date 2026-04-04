package health

import (
	"net/http"

	"github.com/gin-gonic/gin"

	httpresp "github.com/baihua19941101/cdnManage/internal/http"
)

type response struct {
	Status string `json:"status"`
}

func RegisterRoutes(router gin.IRouter) {
	router.GET("/healthz", Handle)
}

func Handle(ctx *gin.Context) {
	httpresp.JSON(ctx, http.StatusOK, "ok", "service healthy", response{Status: "healthy"})
}
