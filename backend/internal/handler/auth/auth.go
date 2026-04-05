package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"

	httpresp "github.com/baihua19941101/cdnManage/internal/http"
	"github.com/baihua19941101/cdnManage/internal/middleware"
	"github.com/baihua19941101/cdnManage/internal/model"
	serviceauth "github.com/baihua19941101/cdnManage/internal/service/auth"
)

type Handler struct {
	service *serviceauth.Service
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"currentPassword" binding:"required"`
	NewPassword     string `json:"newPassword" binding:"required,min=8"`
}

type userResponse struct {
	ID           uint64 `json:"id"`
	Username     string `json:"username"`
	Email        string `json:"email"`
	Status       string `json:"status"`
	PlatformRole string `json:"platformRole"`
}

func NewHandler(service *serviceauth.Service) *Handler {
	return &Handler{service: service}
}

func RegisterRoutes(router gin.IRouter, handler *Handler) {
	group := router.Group("/api/v1/auth")
	group.POST("/login", handler.Login)

	protected := group.Group("")
	protected.Use(middleware.Authentication(handler.service))
	protected.GET("/me", handler.Me)
	protected.POST("/change-password", handler.ChangePassword)
}

func (h *Handler) Login(ctx *gin.Context) {
	var req loginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.Error(httpresp.NewAppError(http.StatusBadRequest, "validation_error", "invalid login request", gin.H{"error": err.Error()}))
		return
	}

	result, err := h.service.Login(ctx.Request.Context(), req.Email, req.Password)
	if err != nil {
		ctx.Error(err)
		return
	}

	httpresp.Success(ctx, gin.H{
		"accessToken": result.AccessToken,
		"user":        toUserResponse(result.User),
	})
}

func (h *Handler) Me(ctx *gin.Context) {
	user, ok := middleware.CurrentUser(ctx)
	if !ok {
		ctx.Error(httpresp.NewAppError(http.StatusUnauthorized, "authentication_failed", "authenticated user is required", nil))
		return
	}

	httpresp.Success(ctx, toUserResponse(user))
}

func (h *Handler) ChangePassword(ctx *gin.Context) {
	userID, ok := middleware.CurrentUserID(ctx)
	if !ok {
		ctx.Error(httpresp.NewAppError(http.StatusUnauthorized, "authentication_failed", "authenticated user is required", nil))
		return
	}

	var req changePasswordRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.Error(httpresp.NewAppError(http.StatusBadRequest, "validation_error", "invalid change password request", gin.H{"error": err.Error()}))
		return
	}

	if err := h.service.ChangePassword(ctx.Request.Context(), userID, req.CurrentPassword, req.NewPassword); err != nil {
		ctx.Error(err)
		return
	}

	httpresp.Success(ctx, gin.H{"message": "password updated"})
}

func toUserResponse(user *model.User) userResponse {
	return userResponse{
		ID:           user.ID,
		Username:     user.Username,
		Email:        user.Email,
		Status:       user.Status,
		PlatformRole: user.PlatformRole,
	}
}
