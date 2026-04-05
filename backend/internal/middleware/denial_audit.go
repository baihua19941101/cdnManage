package middleware

import (
	"encoding/json"
	"fmt"

	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"

	httpresp "github.com/baihua19941101/cdnManage/internal/http"
	"github.com/baihua19941101/cdnManage/internal/model"
	"github.com/baihua19941101/cdnManage/internal/repository"
)

const (
	actionPermissionDenied   = "security.permission_denied"
	actionProjectScopeDenied = "security.project_scope_denied"
)

type AccessDeniedAuditor struct {
	audits repository.AuditLogRepository
}

func NewAccessDeniedAuditor(audits repository.AuditLogRepository) *AccessDeniedAuditor {
	return &AccessDeniedAuditor{audits: audits}
}

func (a *AccessDeniedAuditor) RecordPermissionDenied(ctx *gin.Context, details gin.H) {
	a.record(ctx, actionPermissionDenied, "permission", requestTargetIdentifier(ctx), nil, details)
}

func (a *AccessDeniedAuditor) RecordProjectScopeDenied(ctx *gin.Context, projectID uint64, details gin.H) {
	targetIdentifier := fmt.Sprintf("%d", projectID)
	a.record(ctx, actionProjectScopeDenied, "project", targetIdentifier, &projectID, details)
}

func (a *AccessDeniedAuditor) record(ctx *gin.Context, action, targetType, targetIdentifier string, projectID *uint64, details gin.H) {
	if a == nil || a.audits == nil {
		return
	}

	userID, ok := CurrentUserID(ctx)
	if !ok {
		return
	}

	metadata, err := json.Marshal(details)
	if err != nil {
		return
	}

	_ = a.audits.Create(ctx.Request.Context(), &model.AuditLog{
		ActorUserID:      userID,
		ProjectID:        projectID,
		Action:           action,
		TargetType:       targetType,
		TargetIdentifier: targetIdentifier,
		Result:           model.AuditResultDenied,
		RequestID:        httpresp.GetRequestID(ctx),
		Metadata:         datatypes.JSON(metadata),
	})
}

func requestTargetIdentifier(ctx *gin.Context) string {
	if fullPath := ctx.FullPath(); fullPath != "" {
		return fmt.Sprintf("%s %s", ctx.Request.Method, fullPath)
	}

	return fmt.Sprintf("%s %s", ctx.Request.Method, ctx.Request.URL.Path)
}
