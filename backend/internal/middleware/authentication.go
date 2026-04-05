package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	httpresp "github.com/baihua19941101/cdnManage/internal/http"
	"github.com/baihua19941101/cdnManage/internal/model"
)

const (
	CurrentUserKey         = "current_user"
	CurrentUserIDKey       = "current_user_id"
	CurrentPlatformRoleKey = "current_platform_role"
)

type requestContextKey string

const (
	requestContextCurrentUserKey         requestContextKey = "current_user"
	requestContextCurrentUserIDKey       requestContextKey = "current_user_id"
	requestContextCurrentPlatformRoleKey requestContextKey = "current_platform_role"
)

type Authenticator interface {
	Authenticate(ctx context.Context, token string) (*model.User, error)
}

func Authentication(authenticator Authenticator) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		token, err := bearerToken(ctx)
		if err != nil {
			ctx.Error(err)
			ctx.Abort()
			return
		}

		user, err := authenticator.Authenticate(ctx.Request.Context(), token)
		if err != nil {
			ctx.Error(err)
			ctx.Abort()
			return
		}

		ctx.Set(CurrentUserKey, user)
		ctx.Set(CurrentUserIDKey, user.ID)
		ctx.Set(CurrentPlatformRoleKey, user.PlatformRole)
		requestContext := context.WithValue(ctx.Request.Context(), requestContextCurrentUserKey, user)
		requestContext = context.WithValue(requestContext, requestContextCurrentUserIDKey, user.ID)
		requestContext = context.WithValue(requestContext, requestContextCurrentPlatformRoleKey, user.PlatformRole)
		ctx.Request = ctx.Request.WithContext(requestContext)
		ctx.Next()
	}
}

func CurrentUser(ctx *gin.Context) (*model.User, bool) {
	value, exists := ctx.Get(CurrentUserKey)
	if !exists {
		return nil, false
	}

	user, ok := value.(*model.User)
	return user, ok
}

func CurrentUserID(ctx *gin.Context) (uint64, bool) {
	value, exists := ctx.Get(CurrentUserIDKey)
	if !exists {
		return 0, false
	}

	userID, ok := value.(uint64)
	return userID, ok
}

func CurrentPlatformRole(ctx *gin.Context) (string, bool) {
	value, exists := ctx.Get(CurrentPlatformRoleKey)
	if !exists {
		return "", false
	}

	role, ok := value.(string)
	return role, ok
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
