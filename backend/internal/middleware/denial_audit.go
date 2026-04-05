package middleware

import (
	"fmt"

	"github.com/gin-gonic/gin"

	httpresp "github.com/baihua19941101/cdnManage/internal/http"
	"github.com/baihua19941101/cdnManage/internal/model"
	auditservice "github.com/baihua19941101/cdnManage/internal/service/audit"
)

const (
	actionPermissionDenied   = "security.permission_denied"
	actionProjectScopeDenied = "security.project_scope_denied"
)

type AccessDeniedAuditor struct {
	recorder *auditservice.Recorder
}

var defaultAccessDeniedAuditor *AccessDeniedAuditor

func NewAccessDeniedAuditor(recorder *auditservice.Recorder) *AccessDeniedAuditor {
	return &AccessDeniedAuditor{recorder: recorder}
}

func SetDefaultAccessDeniedAuditor(auditor *AccessDeniedAuditor) {
	defaultAccessDeniedAuditor = auditor
}

func currentAccessDeniedAuditor(auditor *AccessDeniedAuditor) *AccessDeniedAuditor {
	if auditor != nil {
		return auditor
	}

	return defaultAccessDeniedAuditor
}

func (a *AccessDeniedAuditor) RecordPermissionDenied(ctx *gin.Context, details gin.H) {
	a.record(ctx, actionPermissionDenied, "permission", requestTargetIdentifier(ctx), nil, details)
}

func (a *AccessDeniedAuditor) RecordProjectScopeDenied(ctx *gin.Context, projectID uint64, details gin.H) {
	targetIdentifier := fmt.Sprintf("%d", projectID)
	a.record(ctx, actionProjectScopeDenied, "project", targetIdentifier, &projectID, details)
}

func (a *AccessDeniedAuditor) record(ctx *gin.Context, action, targetType, targetIdentifier string, projectID *uint64, details gin.H) {
	if a == nil || a.recorder == nil {
		return
	}

	userID, ok := CurrentUserID(ctx)
	if !ok {
		return
	}

	_ = a.recorder.Record(ctx.Request.Context(), auditservice.RecordInput{
		ActorUserID:      userID,
		ProjectID:        projectID,
		Action:           action,
		TargetType:       targetType,
		TargetIdentifier: targetIdentifier,
		Result:           model.AuditResultDenied,
		RequestID:        httpresp.GetRequestID(ctx),
		Metadata:         map[string]interface{}(details),
	})
}

func requestTargetIdentifier(ctx *gin.Context) string {
	if fullPath := ctx.FullPath(); fullPath != "" {
		return fmt.Sprintf("%s %s", ctx.Request.Method, fullPath)
	}

	return fmt.Sprintf("%s %s", ctx.Request.Method, ctx.Request.URL.Path)
}
