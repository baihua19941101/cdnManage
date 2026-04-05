package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	httpresp "github.com/baihua19941101/cdnManage/internal/http"
	"github.com/baihua19941101/cdnManage/internal/model"
	"github.com/baihua19941101/cdnManage/internal/repository"
)

func TestRequirePlatformWriteAllowsPlatformAdmin(t *testing.T) {
	router := newMiddlewareTestRouter()
	router.POST("/platform/write", injectIdentity(model.PlatformRoleAdmin, 1001), RequirePlatformWrite(), func(ctx *gin.Context) {
		httpresp.Success(ctx, gin.H{"ok": true})
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/platform/write", nil)

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestRequireProjectWriteAllowsProjectAdmin(t *testing.T) {
	router := newMiddlewareTestRouter()
	router.POST("/projects/:id/write", injectIdentity(model.PlatformRoleStandard, 1002), func(ctx *gin.Context) {
		SetCurrentProjectRole(ctx, model.ProjectRoleAdmin)
		ctx.Next()
	}, RequireProjectWrite(), func(ctx *gin.Context) {
		role, ok := CurrentProjectRole(ctx)
		require.True(t, ok)
		require.Equal(t, model.ProjectRoleAdmin, role)
		httpresp.Success(ctx, gin.H{"projectRole": role})
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/projects/12/write", nil)

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestRequireProjectWriteDeniesProjectReadOnlyAndWritesAudit(t *testing.T) {
	auditRepo := &memoryAuditLogRepository{}
	SetDefaultAccessDeniedAuditor(NewAccessDeniedAuditor(auditRepo))
	t.Cleanup(func() {
		SetDefaultAccessDeniedAuditor(nil)
	})

	router := newMiddlewareTestRouter()
	router.POST("/projects/:id/write", injectIdentity(model.PlatformRoleStandard, 1003), func(ctx *gin.Context) {
		SetCurrentProjectID(ctx, 22)
		SetCurrentProjectRole(ctx, model.ProjectRoleReadOnly)
		ctx.Next()
	}, RequireProjectWrite(), func(ctx *gin.Context) {
		httpresp.Success(ctx, gin.H{"unexpected": true})
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/projects/22/write", nil)

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusForbidden, recorder.Code)

	logs, err := auditRepo.List(context.Background(), repository.AuditLogFilter{
		ActorUserID: uint64Pointer(1003),
		Action:      actionPermissionDenied,
		Result:      model.AuditResultDenied,
	})
	require.NoError(t, err)
	require.Len(t, logs, 1)
	require.Equal(t, "permission", logs[0].TargetType)
	require.Equal(t, "POST /projects/:id/write", logs[0].TargetIdentifier)
	require.Equal(t, model.AuditResultDenied, logs[0].Result)
	require.NotEmpty(t, logs[0].RequestID)

	var metadata map[string]any
	require.NoError(t, json.Unmarshal(logs[0].Metadata, &metadata))
	require.Equal(t, string(WritePermission), metadata["permissionLevel"])
	require.Equal(t, model.PlatformRoleStandard, metadata["platformRole"])
	require.Equal(t, model.ProjectRoleReadOnly, metadata["projectRole"])
}

func TestRequireProjectWriteDeniesStorageMutationAndWritesAudit(t *testing.T) {
	auditRepo := &memoryAuditLogRepository{}
	SetDefaultAccessDeniedAuditor(NewAccessDeniedAuditor(auditRepo))
	t.Cleanup(func() {
		SetDefaultAccessDeniedAuditor(nil)
	})

	router := newMiddlewareTestRouter()
	router.PUT("/projects/:id/storage/rename", injectIdentity(model.PlatformRoleStandard, 1010), func(ctx *gin.Context) {
		SetCurrentProjectID(ctx, 52)
		SetCurrentProjectRole(ctx, model.ProjectRoleReadOnly)
		ctx.Next()
	}, RequireProjectWrite(), func(ctx *gin.Context) {
		httpresp.Success(ctx, gin.H{"unexpected": true})
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/projects/52/storage/rename", nil)

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusForbidden, recorder.Code)

	logs, err := auditRepo.List(context.Background(), repository.AuditLogFilter{
		ActorUserID: uint64Pointer(1010),
		Action:      actionPermissionDenied,
		Result:      model.AuditResultDenied,
	})
	require.NoError(t, err)
	require.Len(t, logs, 1)
	require.Equal(t, "permission", logs[0].TargetType)
	require.Equal(t, "PUT /projects/:id/storage/rename", logs[0].TargetIdentifier)
}

func TestProjectScopeResolverInjectsProjectRoleForAuthorizedUser(t *testing.T) {
	resolver := NewProjectScopeResolver(&memoryUserProjectRoleRepository{
		bindings: []model.UserProjectRole{
			{UserID: 1004, ProjectID: 31, ProjectRole: model.ProjectRoleAdmin},
		},
	}, nil, time.Minute)

	router := newMiddlewareTestRouter()
	router.GET("/projects/:id/resource", injectIdentity(model.PlatformRoleStandard, 1004), resolver.Middleware(), func(ctx *gin.Context) {
		projectID, ok := CurrentProjectID(ctx)
		require.True(t, ok)
		projectRole, ok := CurrentProjectRole(ctx)
		require.True(t, ok)
		httpresp.Success(ctx, gin.H{
			"projectID":   projectID,
			"projectRole": projectRole,
		})
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/projects/31/resource", nil)

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"projectRole":"project_admin"`)
}

func TestProjectScopeResolverDeniesUnauthorizedProjectAndWritesAudit(t *testing.T) {
	auditRepo := &memoryAuditLogRepository{}
	SetDefaultAccessDeniedAuditor(NewAccessDeniedAuditor(auditRepo))
	t.Cleanup(func() {
		SetDefaultAccessDeniedAuditor(nil)
	})

	resolver := NewProjectScopeResolver(&memoryUserProjectRoleRepository{
		bindings: []model.UserProjectRole{
			{UserID: 1005, ProjectID: 41, ProjectRole: model.ProjectRoleReadOnly},
		},
	}, nil, time.Minute)

	router := newMiddlewareTestRouter()
	router.GET("/projects/:id/resource", injectIdentity(model.PlatformRoleStandard, 1005), resolver.Middleware(), func(ctx *gin.Context) {
		httpresp.Success(ctx, gin.H{"unexpected": true})
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/projects/42/resource", nil)

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusForbidden, recorder.Code)

	logs, err := auditRepo.List(context.Background(), repository.AuditLogFilter{
		ActorUserID: uint64Pointer(1005),
		Action:      actionProjectScopeDenied,
		Result:      model.AuditResultDenied,
	})
	require.NoError(t, err)
	require.Len(t, logs, 1)
	require.Equal(t, uint64(42), *logs[0].ProjectID)
	require.Equal(t, "project", logs[0].TargetType)
	require.Equal(t, "42", logs[0].TargetIdentifier)
	require.NotEmpty(t, logs[0].RequestID)
}

func newMiddlewareTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestID(), ErrorHandler())
	return router
}

func injectIdentity(platformRole string, userID uint64) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Set(CurrentUserIDKey, userID)
		ctx.Set(CurrentPlatformRoleKey, platformRole)
		ctx.Next()
	}
}

type memoryAuditLogRepository struct {
	logs []model.AuditLog
}

func (r *memoryAuditLogRepository) Create(_ context.Context, log *model.AuditLog) error {
	copied := *log
	r.logs = append(r.logs, copied)
	return nil
}

func (r *memoryAuditLogRepository) List(_ context.Context, filter repository.AuditLogFilter) ([]model.AuditLog, error) {
	var result []model.AuditLog
	for _, log := range r.logs {
		if filter.ActorUserID != nil && log.ActorUserID != *filter.ActorUserID {
			continue
		}
		if filter.ProjectID != nil {
			if log.ProjectID == nil || *log.ProjectID != *filter.ProjectID {
				continue
			}
		}
		if filter.Action != "" && log.Action != filter.Action {
			continue
		}
		if filter.Result != "" && log.Result != filter.Result {
			continue
		}
		result = append(result, log)
	}
	return result, nil
}

type memoryUserProjectRoleRepository struct {
	bindings []model.UserProjectRole
}

func (r *memoryUserProjectRoleRepository) ListByUserID(_ context.Context, userID uint64) ([]model.UserProjectRole, error) {
	var result []model.UserProjectRole
	for _, binding := range r.bindings {
		if binding.UserID == userID {
			result = append(result, binding)
		}
	}
	return result, nil
}

func uint64Pointer(value uint64) *uint64 {
	return &value
}

func uintToString(value uint64) string {
	return fmt.Sprintf("%d", value)
}
