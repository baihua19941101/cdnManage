package users

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	httpresp "github.com/baihua19941101/cdnManage/internal/http"
	"github.com/baihua19941101/cdnManage/internal/middleware"
	"github.com/baihua19941101/cdnManage/internal/model"
	"github.com/baihua19941101/cdnManage/internal/repository"
	serviceauth "github.com/baihua19941101/cdnManage/internal/service/auth"
	serviceusers "github.com/baihua19941101/cdnManage/internal/service/users"
)

type Handler struct {
	service *serviceusers.Service
}

type createUserRequest struct {
	Username     string `json:"username" binding:"required"`
	Email        string `json:"email" binding:"required,email"`
	Password     string `json:"password" binding:"required,min=8"`
	Status       string `json:"status" binding:"required"`
	PlatformRole string `json:"platformRole" binding:"required"`
}

type updateUserRequest struct {
	Username     string `json:"username" binding:"required"`
	Email        string `json:"email" binding:"required,email"`
	Status       string `json:"status" binding:"required"`
	PlatformRole string `json:"platformRole" binding:"required"`
}

type resetPasswordRequest struct {
	NewPassword string `json:"newPassword" binding:"required,min=8"`
}

type replaceBindingsRequest struct {
	Bindings []projectBindingRequest `json:"bindings"`
}

type projectBindingRequest struct {
	ProjectID   uint64 `json:"projectId" binding:"required"`
	ProjectRole string `json:"projectRole" binding:"required"`
}

type userResponse struct {
	ID           uint64                `json:"id"`
	Username     string                `json:"username"`
	Email        string                `json:"email"`
	Status       string                `json:"status"`
	PlatformRole string                `json:"platformRole"`
	ProjectRoles []projectRoleResponse `json:"projectRoles,omitempty"`
}

type projectRoleResponse struct {
	ProjectID   uint64 `json:"projectId"`
	ProjectRole string `json:"projectRole"`
}

func NewHandler(service *serviceusers.Service) *Handler {
	return &Handler{service: service}
}

func RegisterRoutes(router gin.IRouter, handler *Handler, authenticator *serviceauth.Service) {
	group := router.Group("/api/v1/users")
	group.Use(middleware.Authentication(authenticator))
	group.Use(middleware.RequirePlatformAdmin())

	group.GET("", handler.List)
	group.POST("", handler.Create)
	group.PUT("/:id", handler.Update)
	group.PUT("/:id/password", handler.ResetPassword)
	group.DELETE("/:id", handler.Delete)
	group.PUT("/:id/project-bindings", handler.ReplaceProjectBindings)
}

func (h *Handler) List(ctx *gin.Context) {
	users, err := h.service.List(ctx.Request.Context(), repository.UserFilter{})
	if err != nil {
		ctx.Error(err)
		return
	}

	response := make([]userResponse, 0, len(users))
	for _, user := range users {
		response = append(response, toUserResponse(&user))
	}

	httpresp.Success(ctx, response)
}

func (h *Handler) Create(ctx *gin.Context) {
	var req createUserRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.Error(httpresp.NewAppError(http.StatusBadRequest, "validation_error", "invalid create user request", gin.H{"error": err.Error()}))
		return
	}

	user, err := h.service.Create(ctx.Request.Context(), serviceusers.CreateUserInput{
		Username:     req.Username,
		Email:        req.Email,
		Password:     req.Password,
		Status:       req.Status,
		PlatformRole: req.PlatformRole,
	})
	if err != nil {
		ctx.Error(err)
		return
	}

	httpresp.Success(ctx, toUserResponse(user))
}

func (h *Handler) Update(ctx *gin.Context) {
	userID, err := userIDFromParam(ctx)
	if err != nil {
		ctx.Error(err)
		return
	}

	var req updateUserRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.Error(httpresp.NewAppError(http.StatusBadRequest, "validation_error", "invalid update user request", gin.H{"error": err.Error()}))
		return
	}

	user, err := h.service.Update(ctx.Request.Context(), userID, serviceusers.UpdateUserInput{
		Username:     req.Username,
		Email:        req.Email,
		Status:       req.Status,
		PlatformRole: req.PlatformRole,
	})
	if err != nil {
		ctx.Error(err)
		return
	}

	httpresp.Success(ctx, toUserResponse(user))
}

func (h *Handler) Delete(ctx *gin.Context) {
	userID, err := userIDFromParam(ctx)
	if err != nil {
		ctx.Error(err)
		return
	}

	if err := h.service.Delete(ctx.Request.Context(), userID); err != nil {
		ctx.Error(err)
		return
	}

	httpresp.Success(ctx, gin.H{"message": "user deleted"})
}

func (h *Handler) ResetPassword(ctx *gin.Context) {
	userID, err := userIDFromParam(ctx)
	if err != nil {
		ctx.Error(err)
		return
	}

	var req resetPasswordRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.Error(httpresp.NewAppError(http.StatusBadRequest, "validation_error", "invalid reset password request", gin.H{"error": err.Error()}))
		return
	}

	if err := h.service.ResetPassword(ctx.Request.Context(), userID, req.NewPassword); err != nil {
		ctx.Error(err)
		return
	}

	httpresp.Success(ctx, gin.H{"message": "password reset"})
}

func (h *Handler) ReplaceProjectBindings(ctx *gin.Context) {
	userID, err := userIDFromParam(ctx)
	if err != nil {
		ctx.Error(err)
		return
	}

	var req replaceBindingsRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.Error(httpresp.NewAppError(http.StatusBadRequest, "validation_error", "invalid replace project bindings request", gin.H{"error": err.Error()}))
		return
	}

	inputs := make([]serviceusers.ProjectBindingInput, 0, len(req.Bindings))
	for _, binding := range req.Bindings {
		inputs = append(inputs, serviceusers.ProjectBindingInput{
			ProjectID:   binding.ProjectID,
			ProjectRole: binding.ProjectRole,
		})
	}

	roles, err := h.service.ReplaceProjectBindings(ctx.Request.Context(), userID, inputs)
	if err != nil {
		ctx.Error(err)
		return
	}

	response := make([]projectRoleResponse, 0, len(roles))
	for _, role := range roles {
		response = append(response, projectRoleResponse{
			ProjectID:   role.ProjectID,
			ProjectRole: role.ProjectRole,
		})
	}

	httpresp.Success(ctx, gin.H{"bindings": response})
}

func userIDFromParam(ctx *gin.Context) (uint64, error) {
	raw := ctx.Param("id")
	userID, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, httpresp.NewAppError(http.StatusBadRequest, "validation_error", "user id must be a positive integer", nil)
	}
	return userID, nil
}

func toUserResponse(user *model.User) userResponse {
	response := userResponse{
		ID:           user.ID,
		Username:     user.Username,
		Email:        user.Email,
		Status:       user.Status,
		PlatformRole: user.PlatformRole,
	}

	if len(user.ProjectRoles) > 0 {
		response.ProjectRoles = make([]projectRoleResponse, 0, len(user.ProjectRoles))
		for _, role := range user.ProjectRoles {
			response.ProjectRoles = append(response.ProjectRoles, projectRoleResponse{
				ProjectID:   role.ProjectID,
				ProjectRole: role.ProjectRole,
			})
		}
	}

	return response
}
