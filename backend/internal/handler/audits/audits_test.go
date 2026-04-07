package audits

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/baihua19941101/cdnManage/internal/middleware"
	"github.com/baihua19941101/cdnManage/internal/model"
	"github.com/baihua19941101/cdnManage/internal/repository"
)

func TestListPlatformAuditLogsAllowsPlatformAdmin(t *testing.T) {
	repo := &memoryAuditLogRepository{
		logs: []model.AuditLog{
			{BaseModel: model.BaseModel{ID: 1}, ActorUserID: 1001, Action: "object.upload", TargetType: "object", TargetIdentifier: "assets/app.js", Result: model.AuditResultSuccess, RequestID: "req-1"},
			{BaseModel: model.BaseModel{ID: 2}, ActorUserID: 1002, Action: "cdn.refresh_url", TargetType: "cdn", TargetIdentifier: "cdn.example.com", Result: model.AuditResultSuccess, RequestID: "req-2"},
		},
	}
	handler := NewHandler(repo)
	router := newAuditTestRouter()
	router.GET("/api/v1/audits", injectIdentity(model.PlatformRoleAdmin, 9001), middleware.RequirePlatformAdmin(), handler.ListPlatformAuditLogs)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/audits?actorUserId=1001", nil)
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"actorUserId":1001`)
	require.NotContains(t, recorder.Body.String(), `"actorUserId":1002`)
}

func TestListProjectAuditLogsAllowsProjectAdminWithinScope(t *testing.T) {
	projectID := uint64(42)
	repo := &memoryAuditLogRepository{
		logs: []model.AuditLog{
			{BaseModel: model.BaseModel{ID: 1}, ActorUserID: 1001, ProjectID: &projectID, Action: "object.upload", TargetType: "object", TargetIdentifier: "assets/app.js", Result: model.AuditResultSuccess, RequestID: "req-1"},
			{BaseModel: model.BaseModel{ID: 2}, ActorUserID: 1002, ProjectID: uint64Pointer(99), Action: "object.delete", TargetType: "object", TargetIdentifier: "assets/old.js", Result: model.AuditResultSuccess, RequestID: "req-2"},
		},
	}
	handler := NewHandler(repo)
	router := newAuditTestRouter()
	router.GET("/api/v1/projects/:id/audits",
		injectIdentity(model.PlatformRoleStandard, 9002),
		injectProjectScope(projectID, model.ProjectRoleAdmin),
		middleware.RequireProjectWrite(),
		handler.ListProjectAuditLogs,
	)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/projects/42/audits?action=object.upload", nil)
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"id":1`)
	require.NotContains(t, recorder.Body.String(), `"id":2`)
}

func TestListProjectAuditLogsDeniesProjectReadOnlyUser(t *testing.T) {
	handler := NewHandler(&memoryAuditLogRepository{})
	router := newAuditTestRouter()
	router.GET("/api/v1/projects/:id/audits",
		injectIdentity(model.PlatformRoleStandard, 9003),
		injectProjectScope(52, model.ProjectRoleReadOnly),
		middleware.RequireProjectWrite(),
		handler.ListProjectAuditLogs,
	)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/projects/52/audits", nil)
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusForbidden, recorder.Code)
}

func TestListProjectAuditLogsFiltersBySessionID(t *testing.T) {
	projectID := uint64(42)
	repo := &memoryAuditLogRepository{
		logs: []model.AuditLog{
			{
				BaseModel:        model.BaseModel{ID: 1},
				ActorUserID:      1001,
				ProjectID:        &projectID,
				Action:           "object.upload",
				TargetType:       "object",
				TargetIdentifier: "assets/a.js",
				Result:           model.AuditResultSuccess,
				RequestID:        "req-1",
				Metadata:         []byte(`{"sessionId":"archive-1"}`),
			},
			{
				BaseModel:        model.BaseModel{ID: 2},
				ActorUserID:      1001,
				ProjectID:        &projectID,
				Action:           "object.upload_archive",
				TargetType:       "object",
				TargetIdentifier: "archive-2",
				Result:           model.AuditResultFailure,
				RequestID:        "req-2",
				Metadata:         []byte(`{"sessionId":"archive-2"}`),
			},
		},
	}
	handler := NewHandler(repo)
	router := newAuditTestRouter()
	router.GET("/api/v1/projects/:id/audits",
		injectIdentity(model.PlatformRoleStandard, 9004),
		injectProjectScope(projectID, model.ProjectRoleAdmin),
		middleware.RequireProjectWrite(),
		handler.ListProjectAuditLogs,
	)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/projects/42/audits?sessionId=archive-2", nil)
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotContains(t, recorder.Body.String(), `"id":1`)
	require.Contains(t, recorder.Body.String(), `"id":2`)
}

func newAuditTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.RequestID(), middleware.ErrorHandler())
	return router
}

func injectIdentity(platformRole string, userID uint64) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Set(middleware.CurrentUserIDKey, userID)
		ctx.Set(middleware.CurrentPlatformRoleKey, platformRole)
		ctx.Next()
	}
}

func injectProjectScope(projectID uint64, projectRole string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		middleware.SetCurrentProjectID(ctx, projectID)
		middleware.SetCurrentProjectRole(ctx, projectRole)
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
		if filter.ProjectID != nil {
			if log.ProjectID == nil || *log.ProjectID != *filter.ProjectID {
				continue
			}
		}
		if filter.ActorUserID != nil && log.ActorUserID != *filter.ActorUserID {
			continue
		}
		if filter.Action != "" && log.Action != filter.Action {
			continue
		}
		if filter.SessionID != "" {
			if metadataSessionID(log.Metadata) != filter.SessionID {
				continue
			}
		}
		result = append(result, log)
	}
	return result, nil
}

func metadataSessionID(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var metadata map[string]any
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return ""
	}
	if value, ok := metadata["sessionId"].(string); ok {
		return value
	}
	return ""
}

func uint64Pointer(value uint64) *uint64 {
	return &value
}
