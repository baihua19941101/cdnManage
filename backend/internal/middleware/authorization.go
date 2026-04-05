package middleware

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	httpresp "github.com/baihua19941101/cdnManage/internal/http"
	"github.com/baihua19941101/cdnManage/internal/model"
)

const CurrentProjectRoleKey = "current_project_role"

const requestContextCurrentProjectRoleKey requestContextKey = "current_project_role"

type PermissionLevel string

const (
	ReadPermission  PermissionLevel = "read"
	WritePermission PermissionLevel = "write"
)

func SetCurrentProjectRole(ctx *gin.Context, role string) {
	ctx.Set(CurrentProjectRoleKey, role)
	requestContext := context.WithValue(ctx.Request.Context(), requestContextCurrentProjectRoleKey, role)
	ctx.Request = ctx.Request.WithContext(requestContext)
}

func CurrentProjectRole(ctx *gin.Context) (string, bool) {
	value, exists := ctx.Get(CurrentProjectRoleKey)
	if !exists {
		return "", false
	}

	role, ok := value.(string)
	return role, ok
}

func RequirePlatformAdmin() gin.HandlerFunc {
	return RequirePlatformWrite()
}

func RequirePlatformRead() gin.HandlerFunc {
	return RequirePlatformPermission(ReadPermission)
}

func RequirePlatformWrite() gin.HandlerFunc {
	return RequirePlatformPermission(WritePermission)
}

func RequirePlatformPermission(level PermissionLevel) gin.HandlerFunc {
	return RequirePlatformPermissionWithAudit(level, nil)
}

func RequirePlatformPermissionWithAudit(level PermissionLevel, auditor *AccessDeniedAuditor) gin.HandlerFunc {
	auditRecorder := currentAccessDeniedAuditor(auditor)
	return func(ctx *gin.Context) {
		platformRole, ok := CurrentPlatformRole(ctx)
		if !ok {
			ctx.Error(httpresp.NewAppError(http.StatusUnauthorized, "authentication_failed", "authenticated user is required", nil))
			ctx.Abort()
			return
		}

		allowed := false
		switch level {
		case ReadPermission:
			allowed = model.CanReadPlatform(platformRole)
		case WritePermission:
			allowed = model.CanWritePlatform(platformRole)
		default:
			ctx.Error(httpresp.NewAppError(http.StatusInternalServerError, "invalid_permission_level", "permission level is invalid", nil))
			ctx.Abort()
			return
		}

		if !allowed {
			details := gin.H{
				"permissionLevel": level,
				"platformRole":    platformRole,
			}
			if auditRecorder != nil {
				auditRecorder.RecordPermissionDenied(ctx, details)
			}
			ctx.Error(httpresp.NewAppError(http.StatusForbidden, "permission_denied", "platform role does not permit this action", details))
			ctx.Abort()
			return
		}

		ctx.Next()
	}
}

func RequireProjectRead() gin.HandlerFunc {
	return RequireProjectPermission(ReadPermission)
}

func RequireProjectWrite() gin.HandlerFunc {
	return RequireProjectPermission(WritePermission)
}

func RequireProjectPermission(level PermissionLevel) gin.HandlerFunc {
	return RequireProjectPermissionWithAudit(level, nil)
}

func RequireProjectPermissionWithAudit(level PermissionLevel, auditor *AccessDeniedAuditor) gin.HandlerFunc {
	auditRecorder := currentAccessDeniedAuditor(auditor)
	return func(ctx *gin.Context) {
		platformRole, ok := CurrentPlatformRole(ctx)
		if !ok {
			ctx.Error(httpresp.NewAppError(http.StatusUnauthorized, "authentication_failed", "authenticated user is required", nil))
			ctx.Abort()
			return
		}

		projectRole, _ := CurrentProjectRole(ctx)
		allowed := false
		switch level {
		case ReadPermission:
			allowed = model.CanReadProject(platformRole, projectRole)
		case WritePermission:
			allowed = model.CanWriteProject(platformRole, projectRole)
		default:
			ctx.Error(httpresp.NewAppError(http.StatusInternalServerError, "invalid_permission_level", "permission level is invalid", nil))
			ctx.Abort()
			return
		}

		if !allowed {
			details := gin.H{
				"permissionLevel": level,
				"platformRole":    platformRole,
				"projectRole":     projectRole,
			}
			if auditRecorder != nil {
				auditRecorder.RecordPermissionDenied(ctx, details)
			}
			ctx.Error(httpresp.NewAppError(http.StatusForbidden, "permission_denied", "insufficient role for requested operation", details))
			ctx.Abort()
			return
		}

		ctx.Next()
	}
}
