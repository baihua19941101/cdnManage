package projects

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	httpresp "github.com/baihua19941101/cdnManage/internal/http"
	"github.com/baihua19941101/cdnManage/internal/middleware"
	"github.com/baihua19941101/cdnManage/internal/model"
	"github.com/baihua19941101/cdnManage/internal/provider"
	"github.com/baihua19941101/cdnManage/internal/repository"
	auditservice "github.com/baihua19941101/cdnManage/internal/service/audit"
	serviceauth "github.com/baihua19941101/cdnManage/internal/service/auth"
	serviceprojects "github.com/baihua19941101/cdnManage/internal/service/projects"
)

type Handler struct {
	service  *serviceprojects.Service
	audits   repository.AuditLogRepository
	recorder *auditservice.Recorder
}

const (
	credentialOperationKeep    = "KEEP"
	credentialOperationReplace = "REPLACE"
)

type projectScopeMiddleware interface {
	Middleware() gin.HandlerFunc
}

type createProjectRequest struct {
	Name        string                 `json:"name" binding:"required"`
	Description string                 `json:"description"`
	Buckets     []projectBucketRequest `json:"buckets"`
	CDNs        []projectCDNRequest    `json:"cdns"`
}

type updateProjectRequest struct {
	Name        string                 `json:"name" binding:"required"`
	Description string                 `json:"description"`
	Buckets     []projectBucketRequest `json:"buckets"`
	CDNs        []projectCDNRequest    `json:"cdns"`
}

type projectResponse struct {
	ID          uint64                  `json:"id"`
	Name        string                  `json:"name"`
	Description string                  `json:"description"`
	CreatedAt   string                  `json:"createdAt"`
	Buckets     []projectBucketResponse `json:"buckets,omitempty"`
	CDNs        []projectCDNResponse    `json:"cdns,omitempty"`
}

type projectBucketRequest struct {
	ID                   uint64 `json:"id"`
	AccessKeyID          string `json:"accessKeyId"`
	AccessKeySecret      string `json:"accessKeySecret"`
	SecurityToken        string `json:"securityToken"`
	ProviderType         string `json:"providerType"`
	BucketName           string `json:"bucketName" binding:"required"`
	Region               string `json:"region"`
	CredentialOperation  string `json:"credentialOperation"`
	Credential           string `json:"credential"`
	CredentialCiphertext string `json:"credentialCiphertext"`
	IsPrimary            bool   `json:"isPrimary"`
}

type projectCDNRequest struct {
	ID                   uint64 `json:"id"`
	AccessKeyID          string `json:"accessKeyId"`
	AccessKeySecret      string `json:"accessKeySecret"`
	SecurityToken        string `json:"securityToken"`
	ProviderType         string `json:"providerType" binding:"required"`
	CDNEndpoint          string `json:"cdnEndpoint" binding:"required"`
	Region               string `json:"region"`
	CredentialOperation  string `json:"credentialOperation"`
	Credential           string `json:"credential"`
	CredentialCiphertext string `json:"credentialCiphertext"`
	PurgeScope           string `json:"purgeScope"`
	IsPrimary            bool   `json:"isPrimary"`
}

type projectBucketResponse struct {
	ID               uint64 `json:"id"`
	ProviderType     string `json:"providerType"`
	BucketName       string `json:"bucketName"`
	Region           string `json:"region"`
	CredentialMasked string `json:"credentialMasked,omitempty"`
	IsPrimary        bool   `json:"isPrimary"`
}

type projectCDNResponse struct {
	ID               uint64 `json:"id"`
	ProviderType     string `json:"providerType"`
	CDNEndpoint      string `json:"cdnEndpoint"`
	Region           string `json:"region"`
	CredentialMasked string `json:"credentialMasked,omitempty"`
	PurgeScope       string `json:"purgeScope"`
	IsPrimary        bool   `json:"isPrimary"`
}

type refreshURLsRequest struct {
	CDNEndpoint string   `json:"cdnEndpoint"`
	URLs        []string `json:"urls" binding:"required"`
}

type refreshDirectoriesRequest struct {
	CDNEndpoint string   `json:"cdnEndpoint"`
	Directories []string `json:"directories" binding:"required"`
}

type syncResourcesRequest struct {
	CDNEndpoint string   `json:"cdnEndpoint"`
	BucketName  string   `json:"bucketName"`
	Paths       []string `json:"paths" binding:"required"`
}

type cdnTaskResultResponse struct {
	ProviderRequestID string            `json:"providerRequestId,omitempty"`
	TaskID            string            `json:"taskId,omitempty"`
	Status            string            `json:"status,omitempty"`
	SubmittedAt       string            `json:"submittedAt,omitempty"`
	CompletedAt       string            `json:"completedAt,omitempty"`
	Metadata          map[string]string `json:"metadata,omitempty"`
}

type bindingCredentialAuditEntry struct {
	action           string
	targetType       string
	targetIdentifier string
	metadata         gin.H
}

func NewHandler(service *serviceprojects.Service, audits repository.AuditLogRepository) *Handler {
	return &Handler{
		service:  service,
		audits:   audits,
		recorder: auditservice.NewRecorder(audits),
	}
}

func RegisterRoutes(router gin.IRouter, handler *Handler, authenticator *serviceauth.Service, projectScope projectScopeMiddleware) {
	group := router.Group("/api/v1/projects")
	group.Use(middleware.Authentication(authenticator))
	group.Use(middleware.RequirePlatformAdmin())

	group.GET("", handler.List)
	group.POST("", handler.Create)
	group.GET("/:id", handler.Get)
	group.PUT("/:id", handler.Update)
	group.DELETE("/:id", handler.Delete)

	projectGroup := router.Group("/api/v1/projects/:id")
	projectGroup.Use(middleware.Authentication(authenticator))
	if projectScope != nil {
		projectGroup.Use(projectScope.Middleware())
	}
	projectGroup.GET("/cdns", middleware.RequireProjectRead(), handler.GetCDNs)
	projectGroup.PUT("/cdns", middleware.RequireProjectWrite(), handler.UpdateCDNs)
	projectGroup.POST("/cdns/refresh-url", middleware.RequireProjectWrite(), handler.RefreshURLs)
	projectGroup.POST("/cdns/refresh-directory", middleware.RequireProjectWrite(), handler.RefreshDirectories)
	projectGroup.POST("/cdns/sync", middleware.RequireProjectWrite(), handler.SyncResources)
}

func (h *Handler) List(ctx *gin.Context) {
	projects, err := h.service.List(ctx.Request.Context(), repository.ProjectFilter{
		Name: ctx.Query("name"),
	})
	if err != nil {
		ctx.Error(err)
		return
	}

	response := make([]projectResponse, 0, len(projects))
	for _, project := range projects {
		response = append(response, toProjectResponse(&project))
	}

	httpresp.Success(ctx, response)
}

func (h *Handler) Get(ctx *gin.Context) {
	projectID, err := projectIDFromParam(ctx)
	if err != nil {
		ctx.Error(err)
		return
	}

	project, err := h.service.GetByID(ctx.Request.Context(), projectID)
	if err != nil {
		ctx.Error(err)
		return
	}

	httpresp.Success(ctx, toProjectResponse(project))
}

func (h *Handler) Create(ctx *gin.Context) {
	var req createProjectRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.Error(httpresp.NewAppError(http.StatusBadRequest, "validation_error", "invalid create project request", gin.H{"error": err.Error()}))
		return
	}
	if err := validateBucketCredentialRequests(req.Buckets); err != nil {
		ctx.Error(err)
		return
	}

	project, err := h.service.Create(ctx.Request.Context(), serviceprojects.CreateProjectInput{
		Name:        req.Name,
		Description: req.Description,
		Buckets:     toProjectBucketInputs(req.Buckets),
		CDNs:        toProjectCDNInputs(req.CDNs),
	})
	if err != nil {
		ctx.Error(err)
		return
	}

	httpresp.Success(ctx, toProjectResponse(project))
}

func (h *Handler) Update(ctx *gin.Context) {
	projectID, err := projectIDFromParam(ctx)
	if err != nil {
		ctx.Error(err)
		return
	}

	var req updateProjectRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.Error(httpresp.NewAppError(http.StatusBadRequest, "validation_error", "invalid update project request", gin.H{"error": err.Error()}))
		return
	}

	bindingAuditEntries := buildBindingCredentialAuditEntries(req)

	project, err := h.service.Update(ctx.Request.Context(), projectID, serviceprojects.UpdateProjectInput{
		Name:        req.Name,
		Description: req.Description,
		Buckets:     toProjectBucketInputs(req.Buckets),
		CDNs:        toProjectCDNInputs(req.CDNs),
	})
	if err != nil {
		h.recordBindingCredentialAudits(ctx, projectID, bindingAuditEntries, model.AuditResultFailure, err)
		ctx.Error(err)
		return
	}

	h.recordBindingCredentialAudits(ctx, projectID, bindingAuditEntries, model.AuditResultSuccess, nil)

	httpresp.Success(ctx, toProjectResponse(project))
}

func (h *Handler) Delete(ctx *gin.Context) {
	projectID, err := projectIDFromParam(ctx)
	if err != nil {
		ctx.Error(err)
		return
	}

	if err := h.service.Delete(ctx.Request.Context(), projectID); err != nil {
		ctx.Error(err)
		return
	}

	httpresp.Success(ctx, gin.H{"message": "project deleted"})
}

func (h *Handler) GetCDNs(ctx *gin.Context) {
	projectID, err := projectIDFromParam(ctx)
	if err != nil {
		ctx.Error(err)
		return
	}

	cdns, err := h.service.GetCDNs(ctx.Request.Context(), projectID)
	if err != nil {
		ctx.Error(err)
		return
	}

	response := make([]projectCDNResponse, 0, len(cdns))
	for _, cdn := range cdns {
		response = append(response, projectCDNResponse{
			ID:               cdn.ID,
			ProviderType:     cdn.ProviderType,
			CDNEndpoint:      cdn.CDNEndpoint,
			Region:           cdn.Region,
			CredentialMasked: cdn.CredentialCiphertext,
			PurgeScope:       cdn.PurgeScope,
			IsPrimary:        cdn.IsPrimary,
		})
	}

	httpresp.Success(ctx, gin.H{"cdns": response})
}

func (h *Handler) UpdateCDNs(ctx *gin.Context) {
	projectID, err := projectIDFromParam(ctx)
	if err != nil {
		ctx.Error(err)
		return
	}

	var req struct {
		CDNs []projectCDNRequest `json:"cdns" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.Error(httpresp.NewAppError(http.StatusBadRequest, "validation_error", "invalid update project cdns request", gin.H{"error": err.Error()}))
		return
	}
	bindingAuditEntries := buildCDNCredentialAuditEntries(req.CDNs)

	cdns, err := h.service.UpdateCDNs(ctx.Request.Context(), projectID, toProjectCDNInputs(req.CDNs))
	if err != nil {
		h.recordBindingCredentialAudits(ctx, projectID, bindingAuditEntries, model.AuditResultFailure, err)
		ctx.Error(err)
		return
	}
	h.recordBindingCredentialAudits(ctx, projectID, bindingAuditEntries, model.AuditResultSuccess, nil)

	response := make([]projectCDNResponse, 0, len(cdns))
	for _, cdn := range cdns {
		response = append(response, projectCDNResponse{
			ID:               cdn.ID,
			ProviderType:     cdn.ProviderType,
			CDNEndpoint:      cdn.CDNEndpoint,
			Region:           cdn.Region,
			CredentialMasked: cdn.CredentialCiphertext,
			PurgeScope:       cdn.PurgeScope,
			IsPrimary:        cdn.IsPrimary,
		})
	}

	httpresp.Success(ctx, gin.H{"cdns": response})
}

func (h *Handler) RefreshURLs(ctx *gin.Context) {
	projectID, err := projectIDFromParam(ctx)
	if err != nil {
		ctx.Error(err)
		return
	}

	var req refreshURLsRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.Error(httpresp.NewAppError(http.StatusBadRequest, "validation_error", "invalid refresh urls request", gin.H{"error": err.Error()}))
		return
	}

	result, err := h.service.RefreshURLs(ctx.Request.Context(), projectID, serviceprojects.RefreshURLsInput{
		CDNEndpoint: req.CDNEndpoint,
		URLs:        req.URLs,
	})
	if err != nil {
		h.recordAudit(ctx, projectID, "cdn.refresh_url", endpointOrPrimary(req.CDNEndpoint), "failure", gin.H{
			"urls":  req.URLs,
			"error": err.Error(),
		})
		ctx.Error(err)
		return
	}

	h.recordAudit(ctx, projectID, "cdn.refresh_url", endpointOrPrimary(req.CDNEndpoint), "success", gin.H{
		"urls":              req.URLs,
		"providerRequestId": result.ProviderRequestID,
		"taskId":            result.TaskID,
		"status":            result.Status,
	})
	httpresp.Success(ctx, toCDNTaskResultResponse(result))
}

func (h *Handler) RefreshDirectories(ctx *gin.Context) {
	projectID, err := projectIDFromParam(ctx)
	if err != nil {
		ctx.Error(err)
		return
	}

	var req refreshDirectoriesRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.Error(httpresp.NewAppError(http.StatusBadRequest, "validation_error", "invalid refresh directories request", gin.H{"error": err.Error()}))
		return
	}

	result, err := h.service.RefreshDirectories(ctx.Request.Context(), projectID, serviceprojects.RefreshDirectoriesInput{
		CDNEndpoint: req.CDNEndpoint,
		Directories: req.Directories,
	})
	if err != nil {
		h.recordAudit(ctx, projectID, "cdn.refresh_directory", endpointOrPrimary(req.CDNEndpoint), "failure", gin.H{
			"directories": req.Directories,
			"error":       err.Error(),
		})
		ctx.Error(err)
		return
	}

	h.recordAudit(ctx, projectID, "cdn.refresh_directory", endpointOrPrimary(req.CDNEndpoint), "success", gin.H{
		"directories":       req.Directories,
		"providerRequestId": result.ProviderRequestID,
		"taskId":            result.TaskID,
		"status":            result.Status,
	})
	httpresp.Success(ctx, toCDNTaskResultResponse(result))
}

func (h *Handler) SyncResources(ctx *gin.Context) {
	projectID, err := projectIDFromParam(ctx)
	if err != nil {
		ctx.Error(err)
		return
	}

	var req syncResourcesRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.Error(httpresp.NewAppError(http.StatusBadRequest, "validation_error", "invalid sync resources request", gin.H{"error": err.Error()}))
		return
	}

	result, err := h.service.SyncResources(ctx.Request.Context(), projectID, serviceprojects.SyncResourcesInput{
		CDNEndpoint: req.CDNEndpoint,
		BucketName:  req.BucketName,
		Paths:       req.Paths,
	})
	if err != nil {
		h.recordAudit(ctx, projectID, "cdn.sync_resources", endpointOrPrimary(req.CDNEndpoint), "failure", gin.H{
			"bucketName": req.BucketName,
			"paths":      req.Paths,
			"error":      err.Error(),
		})
		ctx.Error(err)
		return
	}

	h.recordAudit(ctx, projectID, "cdn.sync_resources", endpointOrPrimary(req.CDNEndpoint), "success", gin.H{
		"bucketName":        req.BucketName,
		"paths":             req.Paths,
		"providerRequestId": result.ProviderRequestID,
		"taskId":            result.TaskID,
		"status":            result.Status,
	})
	httpresp.Success(ctx, toCDNTaskResultResponse(result))
}

func projectIDFromParam(ctx *gin.Context) (uint64, error) {
	raw := ctx.Param("id")
	projectID, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, httpresp.NewAppError(http.StatusBadRequest, "validation_error", "project id must be a positive integer", nil)
	}
	return projectID, nil
}

func toProjectResponse(project *model.Project) projectResponse {
	response := projectResponse{
		ID:          project.ID,
		Name:        project.Name,
		Description: project.Description,
		CreatedAt:   project.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	if len(project.Buckets) > 0 {
		response.Buckets = make([]projectBucketResponse, 0, len(project.Buckets))
		for _, bucket := range project.Buckets {
			response.Buckets = append(response.Buckets, projectBucketResponse{
				ID:               bucket.ID,
				ProviderType:     bucket.ProviderType,
				BucketName:       bucket.BucketName,
				Region:           bucket.Region,
				CredentialMasked: bucket.CredentialCiphertext,
				IsPrimary:        bucket.IsPrimary,
			})
		}
	}

	if len(project.CDNs) > 0 {
		response.CDNs = make([]projectCDNResponse, 0, len(project.CDNs))
		for _, cdn := range project.CDNs {
			response.CDNs = append(response.CDNs, projectCDNResponse{
				ID:               cdn.ID,
				ProviderType:     cdn.ProviderType,
				CDNEndpoint:      cdn.CDNEndpoint,
				Region:           cdn.Region,
				CredentialMasked: cdn.CredentialCiphertext,
				PurgeScope:       cdn.PurgeScope,
				IsPrimary:        cdn.IsPrimary,
			})
		}
	}

	return response
}

func toProjectBucketInputs(requests []projectBucketRequest) []serviceprojects.ProjectBucketInput {
	result := make([]serviceprojects.ProjectBucketInput, 0, len(requests))
	for _, bucket := range requests {
		result = append(result, serviceprojects.ProjectBucketInput{
			ID:                   bucket.ID,
			ProviderType:         bucket.ProviderType,
			BucketName:           bucket.BucketName,
			Region:               bucket.Region,
			CredentialOperation:  bucket.CredentialOperation,
			Credential:           coalesceCredential(bucket.Credential, bucket.AccessKeyID, bucket.AccessKeySecret, bucket.SecurityToken, bucket.Region),
			CredentialCiphertext: bucket.CredentialCiphertext,
			IsPrimary:            bucket.IsPrimary,
		})
	}
	return result
}

func validateBucketCredentialRequests(buckets []projectBucketRequest) error {
	for _, bucket := range buckets {
		if strings.TrimSpace(coalesceCredential(bucket.Credential, bucket.AccessKeyID, bucket.AccessKeySecret, bucket.SecurityToken, bucket.Region)) == "" &&
			strings.TrimSpace(bucket.CredentialCiphertext) == "" {
			return httpresp.NewAppError(http.StatusBadRequest, "validation_error", "bucket credential is required", nil)
		}
	}
	return nil
}

func toProjectCDNInputs(requests []projectCDNRequest) []serviceprojects.ProjectCDNInput {
	result := make([]serviceprojects.ProjectCDNInput, 0, len(requests))
	for _, cdn := range requests {
		result = append(result, serviceprojects.ProjectCDNInput{
			ID:                   cdn.ID,
			ProviderType:         cdn.ProviderType,
			CDNEndpoint:          cdn.CDNEndpoint,
			Region:               cdn.Region,
			CredentialOperation:  cdn.CredentialOperation,
			Credential:           coalesceCredential(cdn.Credential, cdn.AccessKeyID, cdn.AccessKeySecret, cdn.SecurityToken, cdn.Region),
			CredentialCiphertext: cdn.CredentialCiphertext,
			PurgeScope:           cdn.PurgeScope,
			IsPrimary:            cdn.IsPrimary,
		})
	}
	return result
}

func coalesceCredential(raw, accessKeyID, accessKeySecret, securityToken, region string) string {
	trimmedRaw := strings.TrimSpace(raw)
	if trimmedRaw != "" {
		return trimmedRaw
	}

	ak := strings.TrimSpace(accessKeyID)
	sk := strings.TrimSpace(accessKeySecret)
	if ak == "" || sk == "" {
		return ""
	}

	payload := map[string]interface{}{
		"accessKeyId":     ak,
		"accessKeySecret": sk,
	}
	if token := strings.TrimSpace(securityToken); token != "" {
		payload["securityToken"] = token
	}
	if reg := strings.TrimSpace(region); reg != "" {
		payload["customFields"] = map[string]string{"region": reg}
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func toCDNTaskResultResponse(result provider.TaskResult) cdnTaskResultResponse {
	response := cdnTaskResultResponse{
		ProviderRequestID: result.ProviderRequestID,
		TaskID:            result.TaskID,
		Status:            result.Status,
		Metadata:          result.Metadata,
	}
	if !result.SubmittedAt.IsZero() {
		response.SubmittedAt = result.SubmittedAt.Format(time.RFC3339)
	}
	if result.CompletedAt != nil {
		response.CompletedAt = result.CompletedAt.Format(time.RFC3339)
	}
	return response
}

func endpointOrPrimary(endpoint string) string {
	if endpoint == "" {
		return "primary"
	}
	return endpoint
}

func buildBindingCredentialAuditEntries(req updateProjectRequest) []bindingCredentialAuditEntry {
	total := len(req.Buckets) + len(req.CDNs)
	entries := make([]bindingCredentialAuditEntry, 0, total)

	for index, bucket := range req.Buckets {
		operation := resolveCredentialOperation(bucket.ID, bucket.CredentialOperation)
		entries = append(entries, bindingCredentialAuditEntry{
			action:           credentialAuditAction(operation),
			targetType:       "bucket",
			targetIdentifier: bindingTargetIdentifier(bucket.ID, strings.TrimSpace(bucket.BucketName), index),
			metadata: gin.H{
				"bindingIndex":         index,
				"bindingID":            bucket.ID,
				"bindingName":          strings.TrimSpace(bucket.BucketName),
				"providerType":         strings.TrimSpace(bucket.ProviderType),
				"credentialOperation":  operation,
				"credentialModeRaw":    strings.TrimSpace(bucket.CredentialOperation),
				"credentialModeSource": credentialOperationSource(bucket.ID, bucket.CredentialOperation),
			},
		})
	}

	for index, cdn := range req.CDNs {
		operation := resolveCredentialOperation(cdn.ID, cdn.CredentialOperation)
		entries = append(entries, bindingCredentialAuditEntry{
			action:           credentialAuditAction(operation),
			targetType:       "cdn",
			targetIdentifier: bindingTargetIdentifier(cdn.ID, strings.TrimSpace(cdn.CDNEndpoint), index),
			metadata: gin.H{
				"bindingIndex":         index,
				"bindingID":            cdn.ID,
				"bindingName":          strings.TrimSpace(cdn.CDNEndpoint),
				"providerType":         strings.TrimSpace(cdn.ProviderType),
				"credentialOperation":  operation,
				"credentialModeRaw":    strings.TrimSpace(cdn.CredentialOperation),
				"credentialModeSource": credentialOperationSource(cdn.ID, cdn.CredentialOperation),
			},
		})
	}

	return entries
}

func buildCDNCredentialAuditEntries(cdns []projectCDNRequest) []bindingCredentialAuditEntry {
	entries := make([]bindingCredentialAuditEntry, 0, len(cdns))
	for index, cdn := range cdns {
		operation := resolveCredentialOperation(cdn.ID, cdn.CredentialOperation)
		entries = append(entries, bindingCredentialAuditEntry{
			action:           credentialAuditAction(operation),
			targetType:       "cdn",
			targetIdentifier: bindingTargetIdentifier(cdn.ID, strings.TrimSpace(cdn.CDNEndpoint), index),
			metadata: gin.H{
				"bindingIndex":         index,
				"bindingID":            cdn.ID,
				"bindingName":          strings.TrimSpace(cdn.CDNEndpoint),
				"providerType":         strings.TrimSpace(cdn.ProviderType),
				"credentialOperation":  operation,
				"credentialModeRaw":    strings.TrimSpace(cdn.CredentialOperation),
				"credentialModeSource": credentialOperationSource(cdn.ID, cdn.CredentialOperation),
			},
		})
	}
	return entries
}

func (h *Handler) recordBindingCredentialAudits(ctx *gin.Context, projectID uint64, entries []bindingCredentialAuditEntry, result string, err error) {
	for _, entry := range entries {
		details := cloneAuditMetadata(entry.metadata)
		if result == model.AuditResultFailure && err != nil {
			for key, value := range failureAuditSummary(err) {
				details[key] = value
			}
		}
		h.recordAuditWithTarget(ctx, projectID, entry.action, entry.targetType, entry.targetIdentifier, result, details)
	}
}

func failureAuditSummary(err error) gin.H {
	if err == nil {
		return nil
	}
	summary := gin.H{"error": err.Error()}
	var appErr *httpresp.AppError
	if errors.As(err, &appErr) {
		summary["errorCode"] = appErr.Code
		summary["errorMessage"] = appErr.Message
	}
	return summary
}

func cloneAuditMetadata(metadata gin.H) gin.H {
	if len(metadata) == 0 {
		return gin.H{}
	}
	cloned := make(gin.H, len(metadata))
	for key, value := range metadata {
		cloned[key] = value
	}
	return cloned
}

func credentialOperationSource(bindingID uint64, raw string) string {
	if strings.TrimSpace(raw) != "" {
		return "request"
	}
	if bindingID > 0 {
		return "default_existing_binding"
	}
	return "empty"
}

func resolveCredentialOperation(bindingID uint64, operation string) string {
	normalized := strings.ToUpper(strings.TrimSpace(operation))
	if normalized == "" && bindingID > 0 {
		return credentialOperationKeep
	}
	return normalized
}

func credentialAuditAction(operation string) string {
	switch operation {
	case credentialOperationKeep:
		return "project.binding.credential.keep"
	case credentialOperationReplace:
		return "project.binding.credential.replace"
	default:
		return "project.binding.credential.unknown"
	}
}

func bindingTargetIdentifier(bindingID uint64, bindingName string, index int) string {
	if bindingID > 0 {
		return fmt.Sprintf("id:%d", bindingID)
	}
	if bindingName != "" {
		return bindingName
	}
	return fmt.Sprintf("index:%d", index)
}

func (h *Handler) recordAudit(ctx *gin.Context, projectID uint64, action, targetIdentifier, result string, details gin.H) {
	h.recordAuditWithTarget(ctx, projectID, action, "cdn", targetIdentifier, result, details)
}

func (h *Handler) recordAuditWithTarget(ctx *gin.Context, projectID uint64, action, targetType, targetIdentifier, result string, details gin.H) {
	if h.recorder == nil {
		return
	}

	actorUserID, ok := middleware.CurrentUserID(ctx)
	if !ok || actorUserID == 0 {
		return
	}

	projectIDValue := projectID
	_ = h.recorder.Record(ctx.Request.Context(), auditservice.RecordInput{
		ActorUserID:      actorUserID,
		ProjectID:        &projectIDValue,
		Action:           action,
		TargetType:       targetType,
		TargetIdentifier: targetIdentifier,
		Result:           result,
		RequestID:        httpresp.GetRequestID(ctx),
		Metadata:         map[string]interface{}(details),
	})
}
