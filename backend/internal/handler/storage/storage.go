package storage

import (
	"net/http"

	"github.com/gin-gonic/gin"

	httpresp "github.com/baihua19941101/cdnManage/internal/http"
	"github.com/baihua19941101/cdnManage/internal/middleware"
	serviceauth "github.com/baihua19941101/cdnManage/internal/service/auth"
	serviceprojects "github.com/baihua19941101/cdnManage/internal/service/projects"
)

type Handler struct {
	projectService *serviceprojects.Service
}

type validateConnectionRequest struct {
	BucketName   string `json:"bucketName" binding:"required"`
	Region       string `json:"region"`
	ProviderType string `json:"providerType"`
	Credential   string `json:"credential" binding:"required"`
}

type validateConnectionResponse struct {
	ProviderType string `json:"providerType"`
}

func NewHandler(projectService *serviceprojects.Service) *Handler {
	return &Handler{projectService: projectService}
}

func RegisterRoutes(router gin.IRouter, handler *Handler, authenticator *serviceauth.Service) {
	group := router.Group("/api/v1/storage")
	group.Use(middleware.Authentication(authenticator))
	group.Use(middleware.RequirePlatformAdmin())
	group.POST("/connections/validate", handler.ValidateConnection)
}

func (h *Handler) ValidateConnection(ctx *gin.Context) {
	var req validateConnectionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.Error(httpresp.NewAppError(http.StatusBadRequest, "validation_error", "invalid validate storage connection request", gin.H{"error": err.Error()}))
		return
	}

	providerType, err := h.projectService.ValidateBucketConnection(ctx.Request.Context(), serviceprojects.ProjectBucketInput{
		ProviderType: req.ProviderType,
		BucketName:   req.BucketName,
		Region:       req.Region,
		Credential:   req.Credential,
	})
	if err != nil {
		ctx.Error(err)
		return
	}

	httpresp.Success(ctx, validateConnectionResponse{
		ProviderType: providerType,
	})
}
