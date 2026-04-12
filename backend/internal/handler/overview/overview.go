package overview

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	httpresp "github.com/baihua19941101/cdnManage/internal/http"
	"github.com/baihua19941101/cdnManage/internal/middleware"
	"github.com/baihua19941101/cdnManage/internal/model"
	"github.com/baihua19941101/cdnManage/internal/repository"
	serviceauth "github.com/baihua19941101/cdnManage/internal/service/auth"
)

const (
	timeWindow24h       = "24h"
	timeWindow7d        = "7d"
	timeWindow30d       = "30d"
	uploadSessionAction = "object.upload_archive"
)

var cdnOverviewActions = map[string]struct{}{
	"cdn.refresh_url":       {},
	"cdn.refresh_directory": {},
	"cdn.sync_resources":    {},
}

type Handler struct {
	projects         projectRepository
	projectBuckets   projectBucketRepository
	projectCDNs      projectCDNRepository
	audits           repository.AuditLogRepository
	userProjectRoles userProjectRoleRepository
}

type projectRepository interface {
	List(ctx context.Context, filter repository.ProjectFilter) ([]model.Project, error)
}

type projectBucketRepository interface {
	ListByProjectID(ctx context.Context, projectID uint64) ([]model.ProjectBucket, error)
}

type projectCDNRepository interface {
	ListByProjectID(ctx context.Context, projectID uint64) ([]model.ProjectCDN, error)
}

type userProjectRoleRepository interface {
	ListByUserID(ctx context.Context, userID uint64) ([]model.UserProjectRole, error)
}

type parsedTimeWindow struct {
	key      string
	duration time.Duration
}

type metricsResponse struct {
	TimeWindow string                 `json:"timeWindow"`
	KPIs       metricsKPIResponse     `json:"kpis"`
	Trends     metricsTrendsResponse  `json:"trends"`
	Ratios     metricsRatiosResponse  `json:"ratios"`
	EmptyState metricsEmptyStateModel `json:"emptyState"`
}

type metricsKPIResponse struct {
	ProjectCount        int   `json:"projectCount"`
	BucketCount         int   `json:"bucketCount"`
	CDNCount            int   `json:"cdnCount"`
	UploadSessionTotal  int   `json:"uploadSessionTotal"`
	CDNOperationTotal   int   `json:"cdnOperationTotal"`
	FailureTotal        int   `json:"failureTotal"`
	UploadAvgDurationMs int64 `json:"uploadAvgDurationMs"`
}

type metricsTrendsResponse struct {
	UploadSessions []metricsTrendPoint `json:"uploadSessions"`
	CDNOperations  []metricsTrendPoint `json:"cdnOperations"`
}

type metricsTrendPoint struct {
	Time    string `json:"time"`
	Success int    `json:"success"`
	Failed  int    `json:"failed"`
}

type metricsRatiosResponse struct {
	ProviderResourceShare []metricsRatioPoint `json:"providerResourceShare"`
	OperationTypeShare    []metricsRatioPoint `json:"operationTypeShare"`
}

type metricsRatioPoint struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

type metricsEmptyStateModel struct {
	HasKPIData   bool `json:"hasKpiData"`
	HasTrendData bool `json:"hasTrendData"`
	HasRatioData bool `json:"hasRatioData"`
}

type trendAccumulator struct {
	success int
	failed  int
}

func NewHandler(
	projects projectRepository,
	projectBuckets projectBucketRepository,
	projectCDNs projectCDNRepository,
	audits repository.AuditLogRepository,
	userProjectRoles userProjectRoleRepository,
) *Handler {
	return &Handler{
		projects:         projects,
		projectBuckets:   projectBuckets,
		projectCDNs:      projectCDNs,
		audits:           audits,
		userProjectRoles: userProjectRoles,
	}
}

func RegisterRoutes(router gin.IRouter, handler *Handler, authenticator *serviceauth.Service) {
	group := router.Group("/api/v1/overview")
	group.Use(middleware.Authentication(authenticator))
	group.Use(middleware.RequirePlatformRead())
	group.GET("/metrics", handler.GetMetrics)
}

func (h *Handler) GetMetrics(ctx *gin.Context) {
	window, err := parseTimeWindow(ctx.Query("timeWindow"))
	if err != nil {
		ctx.Error(err)
		return
	}

	visibleProjects, err := h.listVisibleProjects(ctx)
	if err != nil {
		ctx.Error(err)
		return
	}

	projectIDs := make(map[uint64]struct{}, len(visibleProjects))
	for _, project := range visibleProjects {
		projectIDs[project.ID] = struct{}{}
	}

	bucketCount, cdnCount, providerResourceShare, err := h.countProjectBindings(ctx.Request.Context(), visibleProjects)
	if err != nil {
		ctx.Error(err)
		return
	}

	now := time.Now().UTC()
	createdAfter := now.Add(-window.duration)
	logs, err := h.audits.List(ctx.Request.Context(), repository.AuditLogFilter{
		CreatedAfter:  &createdAfter,
		CreatedBefore: &now,
	})
	if err != nil {
		ctx.Error(err)
		return
	}

	isPlatformAdmin := false
	if role, ok := middleware.CurrentPlatformRole(ctx); ok {
		isPlatformAdmin = model.IsPlatformAdminRole(role)
	}

	filteredLogs := filterVisibleLogs(logs, isPlatformAdmin, projectIDs)
	kpis, trends, operationTypeShare := aggregateMetrics(filteredLogs, window)
	kpis.ProjectCount = len(visibleProjects)
	kpis.BucketCount = bucketCount
	kpis.CDNCount = cdnCount

	response := metricsResponse{
		TimeWindow: window.key,
		KPIs:       kpis,
		Trends:     trends,
		Ratios: metricsRatiosResponse{
			ProviderResourceShare: providerResourceShare,
			OperationTypeShare:    operationTypeShare,
		},
		EmptyState: metricsEmptyStateModel{
			HasKPIData:   hasKPIData(kpis),
			HasTrendData: len(trends.UploadSessions) > 0 || len(trends.CDNOperations) > 0,
			HasRatioData: len(providerResourceShare) > 0 || len(operationTypeShare) > 0,
		},
	}

	httpresp.Success(ctx, response)
}

func parseTimeWindow(raw string) (parsedTimeWindow, error) {
	switch raw {
	case "", timeWindow24h:
		return parsedTimeWindow{key: timeWindow24h, duration: 24 * time.Hour}, nil
	case timeWindow7d:
		return parsedTimeWindow{key: timeWindow7d, duration: 7 * 24 * time.Hour}, nil
	case timeWindow30d:
		return parsedTimeWindow{key: timeWindow30d, duration: 30 * 24 * time.Hour}, nil
	default:
		return parsedTimeWindow{}, httpresp.NewAppError(http.StatusBadRequest, "validation_error", "timeWindow must be one of 24h, 7d, 30d", map[string]interface{}{
			"field":         "timeWindow",
			"allowedValues": []string{timeWindow24h, timeWindow7d, timeWindow30d},
		})
	}
}

func (h *Handler) listVisibleProjects(ctx *gin.Context) ([]model.Project, error) {
	platformRole, ok := middleware.CurrentPlatformRole(ctx)
	if !ok {
		return nil, httpresp.NewAppError(http.StatusUnauthorized, "authentication_failed", "authenticated user is required", nil)
	}

	projects, err := h.projects.List(ctx.Request.Context(), repository.ProjectFilter{})
	if err != nil {
		return nil, err
	}

	if model.IsPlatformAdminRole(platformRole) {
		return projects, nil
	}

	userID, ok := middleware.CurrentUserID(ctx)
	if !ok {
		return nil, httpresp.NewAppError(http.StatusUnauthorized, "authentication_failed", "authenticated user is required", nil)
	}

	bindings, err := h.userProjectRoles.ListByUserID(ctx.Request.Context(), userID)
	if err != nil {
		return nil, err
	}

	allowedProjectIDs := make(map[uint64]struct{}, len(bindings))
	for _, binding := range bindings {
		allowedProjectIDs[binding.ProjectID] = struct{}{}
	}

	filteredProjects := make([]model.Project, 0, len(allowedProjectIDs))
	for _, project := range projects {
		if _, exists := allowedProjectIDs[project.ID]; exists {
			filteredProjects = append(filteredProjects, project)
		}
	}

	return filteredProjects, nil
}

func (h *Handler) countProjectBindings(ctx context.Context, projects []model.Project) (int, int, []metricsRatioPoint, error) {
	bucketCount := 0
	cdnCount := 0
	providerCounts := map[string]int{}

	for _, project := range projects {
		buckets, err := h.projectBuckets.ListByProjectID(ctx, project.ID)
		if err != nil {
			return 0, 0, nil, err
		}
		cdns, err := h.projectCDNs.ListByProjectID(ctx, project.ID)
		if err != nil {
			return 0, 0, nil, err
		}

		bucketCount += len(buckets)
		cdnCount += len(cdns)
		for _, bucket := range buckets {
			providerCounts[normalizeProviderName(bucket.ProviderType)]++
		}
		for _, cdn := range cdns {
			providerCounts[normalizeProviderName(cdn.ProviderType)]++
		}
	}

	return bucketCount, cdnCount, countsToRatioPoints(providerCounts), nil
}

func filterVisibleLogs(logs []model.AuditLog, isPlatformAdmin bool, projectIDs map[uint64]struct{}) []model.AuditLog {
	if isPlatformAdmin {
		return logs
	}

	filtered := make([]model.AuditLog, 0, len(logs))
	for _, log := range logs {
		if log.ProjectID == nil {
			continue
		}
		if _, exists := projectIDs[*log.ProjectID]; !exists {
			continue
		}
		filtered = append(filtered, log)
	}

	return filtered
}

func aggregateMetrics(logs []model.AuditLog, window parsedTimeWindow) (metricsKPIResponse, metricsTrendsResponse, []metricsRatioPoint) {
	kpis := metricsKPIResponse{}
	uploadTrendMap := make(map[string]*trendAccumulator)
	cdnTrendMap := make(map[string]*trendAccumulator)
	operationCounts := make(map[string]int)

	var uploadDurationTotal int64
	var uploadDurationCount int64

	for _, log := range logs {
		actionName := strings.TrimSpace(log.Action)
		if actionName != "" {
			operationCounts[actionName]++
		}

		if log.Action == uploadSessionAction {
			kpis.UploadSessionTotal++
			if log.Result == model.AuditResultFailure {
				kpis.FailureTotal++
			}
			if duration, ok := durationMsFromMetadata(log.Metadata); ok {
				uploadDurationTotal += duration
				uploadDurationCount++
			}

			key := trendBucketKey(log.CreatedAt, window)
			addTrend(uploadTrendMap, key, log.Result)
			continue
		}

		if _, exists := cdnOverviewActions[log.Action]; exists {
			kpis.CDNOperationTotal++
			if log.Result == model.AuditResultFailure {
				kpis.FailureTotal++
			}

			key := trendBucketKey(log.CreatedAt, window)
			addTrend(cdnTrendMap, key, log.Result)
		}
	}

	if uploadDurationCount > 0 {
		kpis.UploadAvgDurationMs = uploadDurationTotal / uploadDurationCount
	}

	operationTypeShare := countsToTopRatioPoints(operationCounts, 5, "other")

	return kpis, metricsTrendsResponse{
		UploadSessions: buildTrendPoints(uploadTrendMap),
		CDNOperations:  buildTrendPoints(cdnTrendMap),
	}, operationTypeShare
}

func addTrend(target map[string]*trendAccumulator, key string, result string) {
	if key == "" {
		return
	}
	if _, exists := target[key]; !exists {
		target[key] = &trendAccumulator{}
	}
	if result == model.AuditResultFailure {
		target[key].failed++
		return
	}
	target[key].success++
}

func buildTrendPoints(source map[string]*trendAccumulator) []metricsTrendPoint {
	if len(source) == 0 {
		return []metricsTrendPoint{}
	}

	keys := make([]string, 0, len(source))
	for key := range source {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	points := make([]metricsTrendPoint, 0, len(keys))
	for _, key := range keys {
		acc := source[key]
		points = append(points, metricsTrendPoint{
			Time:    key,
			Success: acc.success,
			Failed:  acc.failed,
		})
	}
	return points
}

func trendBucketKey(value time.Time, window parsedTimeWindow) string {
	if value.IsZero() {
		return ""
	}

	if window.key == timeWindow24h {
		return value.Local().Format("2006-01-02 15:00")
	}
	return value.Local().Format("2006-01-02")
}

func durationMsFromMetadata(metadata []byte) (int64, bool) {
	if len(metadata) == 0 {
		return 0, false
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(metadata, &payload); err != nil {
		return 0, false
	}

	raw, exists := payload["durationMs"]
	if !exists {
		return 0, false
	}

	switch value := raw.(type) {
	case float64:
		if value < 0 {
			return 0, false
		}
		return int64(value), true
	case int64:
		if value < 0 {
			return 0, false
		}
		return value, true
	case int:
		if value < 0 {
			return 0, false
		}
		return int64(value), true
	default:
		return 0, false
	}
}

func hasKPIData(kpis metricsKPIResponse) bool {
	if kpis.ProjectCount > 0 || kpis.BucketCount > 0 || kpis.CDNCount > 0 {
		return true
	}
	if kpis.UploadSessionTotal > 0 || kpis.CDNOperationTotal > 0 || kpis.FailureTotal > 0 {
		return true
	}
	return kpis.UploadAvgDurationMs > 0
}

func normalizeProviderName(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "unknown"
	}
	return value
}

func countsToRatioPoints(counts map[string]int) []metricsRatioPoint {
	if len(counts) == 0 {
		return []metricsRatioPoint{}
	}

	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	result := make([]metricsRatioPoint, 0, len(keys))
	for _, key := range keys {
		if counts[key] <= 0 {
			continue
		}
		result = append(result, metricsRatioPoint{
			Name:  key,
			Value: counts[key],
		})
	}
	return result
}

func countsToTopRatioPoints(counts map[string]int, topN int, otherName string) []metricsRatioPoint {
	if len(counts) == 0 {
		return []metricsRatioPoint{}
	}

	type item struct {
		name  string
		value int
	}
	items := make([]item, 0, len(counts))
	for name, value := range counts {
		if strings.TrimSpace(name) == "" || value <= 0 {
			continue
		}
		items = append(items, item{name: name, value: value})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].value == items[j].value {
			return items[i].name < items[j].name
		}
		return items[i].value > items[j].value
	})

	if topN <= 0 || len(items) <= topN {
		result := make([]metricsRatioPoint, 0, len(items))
		for _, each := range items {
			result = append(result, metricsRatioPoint{
				Name:  each.name,
				Value: each.value,
			})
		}
		return result
	}

	result := make([]metricsRatioPoint, 0, topN+1)
	other := 0
	for index, each := range items {
		if index < topN {
			result = append(result, metricsRatioPoint{
				Name:  each.name,
				Value: each.value,
			})
			continue
		}
		other += each.value
	}

	if other > 0 {
		result = append(result, metricsRatioPoint{
			Name:  otherName,
			Value: other,
		})
	}
	return result
}
