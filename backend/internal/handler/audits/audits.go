package audits

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	httpresp "github.com/baihua19941101/cdnManage/internal/http"
	"github.com/baihua19941101/cdnManage/internal/middleware"
	"github.com/baihua19941101/cdnManage/internal/model"
	"github.com/baihua19941101/cdnManage/internal/repository"
	serviceauth "github.com/baihua19941101/cdnManage/internal/service/auth"
)

type projectScopeMiddleware interface {
	Middleware() gin.HandlerFunc
}

type Handler struct {
	audits           repository.AuditLogRepository
	projects         projectRepository
	userProjectRoles userProjectRoleRepository
}

type projectRepository interface {
	List(ctx context.Context, filter repository.ProjectFilter) ([]model.Project, error)
}

type userProjectRoleRepository interface {
	ListByUserID(ctx context.Context, userID uint64) ([]model.UserProjectRole, error)
}

type auditLogResponse struct {
	ID               uint64                 `json:"id"`
	ActorUserID      uint64                 `json:"actorUserId"`
	ActorUsername    string                 `json:"actorUsername,omitempty"`
	Action           string                 `json:"action"`
	TargetType       string                 `json:"targetType"`
	TargetIdentifier string                 `json:"targetIdentifier"`
	Result           string                 `json:"result"`
	RequestID        string                 `json:"requestId"`
	CreatedAt        string                 `json:"createdAt"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

type listAuditLogsResponse struct {
	Logs []auditLogResponse `json:"logs"`
}

type platformAuditFilterOptionsResponse struct {
	Actions     []string `json:"actions"`
	TargetTypes []string `json:"targetTypes"`
}

type projectFilterOptionProject struct {
	ProjectID   uint64 `json:"projectId"`
	ProjectName string `json:"projectName"`
}

type projectAuditFilterOptionsResponse struct {
	Projects    []projectFilterOptionProject `json:"projects"`
	Actions     []string                     `json:"actions"`
	TargetTypes []string                     `json:"targetTypes"`
}

func NewHandler(audits repository.AuditLogRepository, projects projectRepository, userProjectRoles userProjectRoleRepository) *Handler {
	return &Handler{
		audits:           audits,
		projects:         projects,
		userProjectRoles: userProjectRoles,
	}
}

func RegisterRoutes(router gin.IRouter, handler *Handler, authenticator *serviceauth.Service, projectScope projectScopeMiddleware) {
	platformGroup := router.Group("/api/v1/audits")
	platformGroup.Use(middleware.Authentication(authenticator))
	platformGroup.Use(middleware.RequirePlatformAdmin())
	platformGroup.GET("", handler.ListPlatformAuditLogs)
	platformGroup.GET("/filter-options", handler.GetPlatformAuditFilterOptions)

	projectGroup := router.Group("/api/v1/projects/:id/audits")
	projectGroup.Use(middleware.Authentication(authenticator))
	if projectScope != nil {
		projectGroup.Use(projectScope.Middleware())
	}
	projectGroup.GET("", middleware.RequireProjectWrite(), handler.ListProjectAuditLogs)
	projectGroup.GET("/filter-options", middleware.RequireProjectWrite(), handler.GetProjectAuditFilterOptions)
}

func (h *Handler) ListPlatformAuditLogs(ctx *gin.Context) {
	filter, err := auditLogFilterFromQuery(ctx, nil)
	if err != nil {
		ctx.Error(err)
		return
	}

	logs, err := h.audits.List(ctx.Request.Context(), filter)
	if err != nil {
		ctx.Error(err)
		return
	}

	httpresp.Success(ctx, listAuditLogsResponse{Logs: toAuditLogResponses(logs)})
}

func (h *Handler) ListProjectAuditLogs(ctx *gin.Context) {
	projectID, err := projectIDFromParam(ctx)
	if err != nil {
		ctx.Error(err)
		return
	}

	filter, err := auditLogFilterFromQuery(ctx, &projectID)
	if err != nil {
		ctx.Error(err)
		return
	}

	logs, err := h.audits.List(ctx.Request.Context(), filter)
	if err != nil {
		ctx.Error(err)
		return
	}

	httpresp.Success(ctx, listAuditLogsResponse{Logs: toAuditLogResponses(logs)})
}

func (h *Handler) GetPlatformAuditFilterOptions(ctx *gin.Context) {
	options, err := h.loadAuditFilterOptions(ctx, nil)
	if err != nil {
		ctx.Error(err)
		return
	}

	httpresp.Success(ctx, platformAuditFilterOptionsResponse{
		Actions:     options.actions,
		TargetTypes: options.targetTypes,
	})
}

func (h *Handler) GetProjectAuditFilterOptions(ctx *gin.Context) {
	projectID, err := projectIDFromParam(ctx)
	if err != nil {
		ctx.Error(err)
		return
	}

	options, err := h.loadAuditFilterOptions(ctx, &projectID)
	if err != nil {
		ctx.Error(err)
		return
	}

	projects, err := h.listWritableProjects(ctx)
	if err != nil {
		ctx.Error(err)
		return
	}

	httpresp.Success(ctx, projectAuditFilterOptionsResponse{
		Projects:    projects,
		Actions:     options.actions,
		TargetTypes: options.targetTypes,
	})
}

type auditFilterOptions struct {
	actions     []string
	targetTypes []string
}

func (h *Handler) loadAuditFilterOptions(ctx *gin.Context, projectID *uint64) (auditFilterOptions, error) {
	actions, err := h.audits.ListDistinctActions(ctx.Request.Context(), projectID)
	if err != nil {
		return auditFilterOptions{}, err
	}

	targetTypes, err := h.audits.ListDistinctTargetTypes(ctx.Request.Context(), projectID)
	if err != nil {
		return auditFilterOptions{}, err
	}

	return auditFilterOptions{
		actions:     actions,
		targetTypes: targetTypes,
	}, nil
}

func (h *Handler) listWritableProjects(ctx *gin.Context) ([]projectFilterOptionProject, error) {
	platformRole, ok := middleware.CurrentPlatformRole(ctx)
	if !ok {
		return nil, httpresp.NewAppError(http.StatusUnauthorized, "authentication_failed", "authenticated user is required", nil)
	}

	projects, err := h.projects.List(ctx.Request.Context(), repository.ProjectFilter{})
	if err != nil {
		return nil, err
	}

	if model.IsPlatformAdminRole(platformRole) {
		return toProjectFilterOptionProjects(projects), nil
	}

	userID, ok := middleware.CurrentUserID(ctx)
	if !ok {
		return nil, httpresp.NewAppError(http.StatusUnauthorized, "authentication_failed", "authenticated user is required", nil)
	}

	bindings, err := h.userProjectRoles.ListByUserID(ctx.Request.Context(), userID)
	if err != nil {
		return nil, err
	}

	writableProjectIDs := make(map[uint64]struct{}, len(bindings))
	for _, binding := range bindings {
		if model.CanWriteProject(platformRole, binding.ProjectRole) {
			writableProjectIDs[binding.ProjectID] = struct{}{}
		}
	}

	filteredProjects := make([]model.Project, 0, len(writableProjectIDs))
	for _, project := range projects {
		if _, exists := writableProjectIDs[project.ID]; exists {
			filteredProjects = append(filteredProjects, project)
		}
	}

	return toProjectFilterOptionProjects(filteredProjects), nil
}

func toProjectFilterOptionProjects(projects []model.Project) []projectFilterOptionProject {
	response := make([]projectFilterOptionProject, 0, len(projects))
	for _, project := range projects {
		response = append(response, projectFilterOptionProject{
			ProjectID:   project.ID,
			ProjectName: project.Name,
		})
	}
	sort.Slice(response, func(i, j int) bool {
		return response[i].ProjectID > response[j].ProjectID
	})
	return response
}

func auditLogFilterFromQuery(ctx *gin.Context, projectID *uint64) (repository.AuditLogFilter, error) {
	limit := 20
	if raw := ctx.Query("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			return repository.AuditLogFilter{}, httpresp.NewAppError(http.StatusBadRequest, "validation_error", "limit must be a positive integer", nil)
		}
		limit = parsed
	}

	offset := 0
	if raw := ctx.Query("offset"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			return repository.AuditLogFilter{}, httpresp.NewAppError(http.StatusBadRequest, "validation_error", "offset must be zero or a positive integer", nil)
		}
		offset = parsed
	}

	var actorUserID *uint64
	if raw := ctx.Query("actorUserId"); raw != "" {
		parsed, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return repository.AuditLogFilter{}, httpresp.NewAppError(http.StatusBadRequest, "validation_error", "actorUserId must be a positive integer", nil)
		}
		actorUserID = &parsed
	}

	createdAfter, err := parseOptionalTime(ctx.Query("createdAfter"), "createdAfter")
	if err != nil {
		return repository.AuditLogFilter{}, err
	}
	createdBefore, err := parseOptionalTime(ctx.Query("createdBefore"), "createdBefore")
	if err != nil {
		return repository.AuditLogFilter{}, err
	}

	return repository.AuditLogFilter{
		ProjectID:        projectID,
		ActorUserID:      actorUserID,
		Action:           ctx.Query("action"),
		TargetType:       ctx.Query("targetType"),
		TargetIdentifier: ctx.Query("targetIdentifier"),
		SessionID:        ctx.Query("sessionId"),
		Result:           ctx.Query("result"),
		CreatedAfter:     createdAfter,
		CreatedBefore:    createdBefore,
		Limit:            limit,
		Offset:           offset,
	}, nil
}

func parseOptionalTime(raw, field string) (*time.Time, error) {
	if raw == "" {
		return nil, nil
	}

	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, httpresp.NewAppError(http.StatusBadRequest, "validation_error", field+" must be RFC3339 datetime", nil)
	}

	return &parsed, nil
}

func projectIDFromParam(ctx *gin.Context) (uint64, error) {
	raw := ctx.Param("id")
	projectID, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, httpresp.NewAppError(http.StatusBadRequest, "validation_error", "project id must be a positive integer", nil)
	}
	return projectID, nil
}

func toAuditLogResponses(logs []model.AuditLog) []auditLogResponse {
	response := make([]auditLogResponse, 0, len(logs))
	for _, log := range logs {
		item := auditLogResponse{
			ID:               log.ID,
			ActorUserID:      log.ActorUserID,
			Action:           log.Action,
			TargetType:       log.TargetType,
			TargetIdentifier: log.TargetIdentifier,
			Result:           log.Result,
			RequestID:        log.RequestID,
			CreatedAt:        log.CreatedAt.Format(time.RFC3339),
		}
		if log.ActorUser.Username != "" {
			item.ActorUsername = log.ActorUser.Username
		}
		if len(log.Metadata) > 0 {
			var metadata map[string]interface{}
			if err := json.Unmarshal(log.Metadata, &metadata); err == nil {
				item.Metadata = metadata
			}
		}
		response = append(response, item)
	}
	return response
}
