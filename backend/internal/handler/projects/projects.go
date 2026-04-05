package projects

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	httpresp "github.com/baihua19941101/cdnManage/internal/http"
	"github.com/baihua19941101/cdnManage/internal/middleware"
	"github.com/baihua19941101/cdnManage/internal/model"
	"github.com/baihua19941101/cdnManage/internal/repository"
	serviceauth "github.com/baihua19941101/cdnManage/internal/service/auth"
	serviceprojects "github.com/baihua19941101/cdnManage/internal/service/projects"
)

type Handler struct {
	service *serviceprojects.Service
}

type createProjectRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

type updateProjectRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

type projectResponse struct {
	ID          uint64 `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedAt   string `json:"createdAt"`
}

func NewHandler(service *serviceprojects.Service) *Handler {
	return &Handler{service: service}
}

func RegisterRoutes(router gin.IRouter, handler *Handler, authenticator *serviceauth.Service) {
	group := router.Group("/api/v1/projects")
	group.Use(middleware.Authentication(authenticator))
	group.Use(middleware.RequirePlatformAdmin())

	group.GET("", handler.List)
	group.POST("", handler.Create)
	group.GET("/:id", handler.Get)
	group.PUT("/:id", handler.Update)
	group.DELETE("/:id", handler.Delete)
}

func (h *Handler) List(ctx *gin.Context) {
	projects, err := h.service.List(ctx.Request.Context(), repository.ProjectFilter{
		Name: ctx.Query("name"),
	})
	if err != nil {
		ctx.Error(err)
		return
	}

	response := make([]projectResponse, 0, len(projects))
	for _, project := range projects {
		response = append(response, toProjectResponse(&project))
	}

	httpresp.Success(ctx, response)
}

func (h *Handler) Get(ctx *gin.Context) {
	projectID, err := projectIDFromParam(ctx)
	if err != nil {
		ctx.Error(err)
		return
	}

	project, err := h.service.GetByID(ctx.Request.Context(), projectID)
	if err != nil {
		ctx.Error(err)
		return
	}

	httpresp.Success(ctx, toProjectResponse(project))
}

func (h *Handler) Create(ctx *gin.Context) {
	var req createProjectRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.Error(httpresp.NewAppError(http.StatusBadRequest, "validation_error", "invalid create project request", gin.H{"error": err.Error()}))
		return
	}

	project, err := h.service.Create(ctx.Request.Context(), serviceprojects.CreateProjectInput{
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		ctx.Error(err)
		return
	}

	httpresp.Success(ctx, toProjectResponse(project))
}

func (h *Handler) Update(ctx *gin.Context) {
	projectID, err := projectIDFromParam(ctx)
	if err != nil {
		ctx.Error(err)
		return
	}

	var req updateProjectRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.Error(httpresp.NewAppError(http.StatusBadRequest, "validation_error", "invalid update project request", gin.H{"error": err.Error()}))
		return
	}

	project, err := h.service.Update(ctx.Request.Context(), projectID, serviceprojects.UpdateProjectInput{
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		ctx.Error(err)
		return
	}

	httpresp.Success(ctx, toProjectResponse(project))
}

func (h *Handler) Delete(ctx *gin.Context) {
	projectID, err := projectIDFromParam(ctx)
	if err != nil {
		ctx.Error(err)
		return
	}

	if err := h.service.Delete(ctx.Request.Context(), projectID); err != nil {
		ctx.Error(err)
		return
	}

	httpresp.Success(ctx, gin.H{"message": "project deleted"})
}

func projectIDFromParam(ctx *gin.Context) (uint64, error) {
	raw := ctx.Param("id")
	projectID, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, httpresp.NewAppError(http.StatusBadRequest, "validation_error", "project id must be a positive integer", nil)
	}
	return projectID, nil
}

func toProjectResponse(project *model.Project) projectResponse {
	return projectResponse{
		ID:          project.ID,
		Name:        project.Name,
		Description: project.Description,
		CreatedAt:   project.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
