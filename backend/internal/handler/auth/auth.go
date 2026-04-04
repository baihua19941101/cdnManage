package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	httpresp "github.com/baihua19941101/cdnManage/internal/http"
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
	group.GET("/me", handler.Me)
	group.POST("/change-password", handler.ChangePassword)
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
	token, err := bearerToken(ctx)
	if err != nil {
		ctx.Error(err)
		return
	}

	user, err := h.service.Me(ctx.Request.Context(), token)
	if err != nil {
		ctx.Error(err)
		return
	}

	httpresp.Success(ctx, toUserResponse(user))
}

func (h *Handler) ChangePassword(ctx *gin.Context) {
	token, err := bearerToken(ctx)
	if err != nil {
		ctx.Error(err)
		return
	}

	var req changePasswordRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.Error(httpresp.NewAppError(http.StatusBadRequest, "validation_error", "invalid change password request", gin.H{"error": err.Error()}))
		return
	}

	if err := h.service.ChangePassword(ctx.Request.Context(), token, req.CurrentPassword, req.NewPassword); err != nil {
		ctx.Error(err)
		return
	}

	httpresp.Success(ctx, gin.H{"message": "password updated"})
}

func bearerToken(ctx *gin.Context) (string, error) {
	header := strings.TrimSpace(ctx.GetHeader("Authorization"))
	if header == "" {
		return "", httpresp.NewAppError(http.StatusUnauthorized, "authentication_failed", "authorization header is required", nil)
	}

	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
		return "", httpresp.NewAppError(http.StatusUnauthorized, "authentication_failed", "invalid authorization header", nil)
	}

	return strings.TrimSpace(parts[1]), nil
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
