package storage

import (
	"encoding/json"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"

	httpresp "github.com/baihua19941101/cdnManage/internal/http"
	"github.com/baihua19941101/cdnManage/internal/middleware"
	"github.com/baihua19941101/cdnManage/internal/model"
	"github.com/baihua19941101/cdnManage/internal/provider"
	"github.com/baihua19941101/cdnManage/internal/repository"
	serviceauth "github.com/baihua19941101/cdnManage/internal/service/auth"
	serviceprojects "github.com/baihua19941101/cdnManage/internal/service/projects"
)

type projectScopeMiddleware interface {
	Middleware() gin.HandlerFunc
}

type Handler struct {
	projectService *serviceprojects.Service
	audits         repository.AuditLogRepository
}

type validateConnectionRequest struct {
	BucketName   string `json:"bucketName" binding:"required"`
	Region       string `json:"region"`
	ProviderType string `json:"providerType"`
	Credential   string `json:"credential" binding:"required"`
}

type validateConnectionResponse struct {
	ProviderType string `json:"providerType"`
}

type objectResponse struct {
	Key          string `json:"key"`
	ETag         string `json:"etag,omitempty"`
	ContentType  string `json:"contentType,omitempty"`
	Size         int64  `json:"size"`
	LastModified string `json:"lastModified,omitempty"`
	IsDir        bool   `json:"isDir"`
}

type listObjectsResponse struct {
	Objects []objectResponse `json:"objects"`
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

type renameObjectRequest struct {
	BucketName string `json:"bucketName"`
	SourceKey  string `json:"sourceKey" binding:"required"`
	TargetKey  string `json:"targetKey" binding:"required"`
}

func NewHandler(projectService *serviceprojects.Service, audits repository.AuditLogRepository) *Handler {
	return &Handler{projectService: projectService, audits: audits}
}

func RegisterRoutes(router gin.IRouter, handler *Handler, authenticator *serviceauth.Service, projectScope projectScopeMiddleware) {
	adminGroup := router.Group("/api/v1/storage")
	adminGroup.Use(middleware.Authentication(authenticator))
	adminGroup.Use(middleware.RequirePlatformAdmin())
	adminGroup.POST("/connections/validate", handler.ValidateConnection)

	projectGroup := router.Group("/api/v1/projects/:id/storage")
	projectGroup.Use(middleware.Authentication(authenticator))
	if projectScope != nil {
		projectGroup.Use(projectScope.Middleware())
	}
	projectGroup.GET("/objects", middleware.RequireProjectRead(), handler.ListObjects)
	projectGroup.GET("/download", middleware.RequireProjectRead(), handler.DownloadObject)
	projectGroup.GET("/audits", middleware.RequireProjectRead(), handler.ListAuditLogs)
	projectGroup.POST("/upload", middleware.RequireProjectWrite(), handler.UploadObject)
	projectGroup.DELETE("/objects", middleware.RequireProjectWrite(), handler.DeleteObject)
	projectGroup.PUT("/rename", middleware.RequireProjectWrite(), handler.RenameObject)
}

func (h *Handler) ValidateConnection(ctx *gin.Context) {
	var req validateConnectionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.Error(httpresp.NewAppError(http.StatusBadRequest, "validation_error", "invalid validate storage connection request", gin.H{"error": err.Error()}))
		return
	}

	providerType, err := h.projectService.ValidateBucketConnection(ctx.Request.Context(), serviceprojects.ProjectBucketInput{
		ProviderType: req.ProviderType,
		BucketName:   req.BucketName,
		Region:       req.Region,
		Credential:   req.Credential,
	})
	if err != nil {
		ctx.Error(err)
		return
	}

	httpresp.Success(ctx, validateConnectionResponse{
		ProviderType: providerType,
	})
}

func (h *Handler) ListObjects(ctx *gin.Context) {
	projectID, err := projectIDFromParam(ctx)
	if err != nil {
		ctx.Error(err)
		return
	}

	maxKeys := 0
	if raw := ctx.Query("maxKeys"); raw != "" {
		parsed, parseErr := strconv.Atoi(raw)
		if parseErr != nil || parsed <= 0 {
			ctx.Error(httpresp.NewAppError(http.StatusBadRequest, "validation_error", "maxKeys must be a positive integer", nil))
			return
		}
		maxKeys = parsed
	}

	objects, err := h.projectService.ListBucketObjects(ctx.Request.Context(), projectID, serviceprojects.ListBucketObjectsInput{
		BucketName: ctx.Query("bucketName"),
		Prefix:     ctx.Query("prefix"),
		Marker:     ctx.Query("marker"),
		MaxKeys:    maxKeys,
	})
	if err != nil {
		h.recordAudit(ctx, projectID, "object.list", "object", objectTargetIdentifier(ctx.Query("prefix"), ctx.Query("marker")), model.AuditResultFailure, gin.H{
			"error": err.Error(),
		})
		ctx.Error(err)
		return
	}
	h.recordAudit(ctx, projectID, "object.list", "object", objectTargetIdentifier(ctx.Query("prefix"), ctx.Query("marker")), model.AuditResultSuccess, gin.H{
		"count": len(objects),
	})

	httpresp.Success(ctx, listObjectsResponse{
		Objects: toObjectResponses(objects),
	})
}

func (h *Handler) UploadObject(ctx *gin.Context) {
	projectID, err := projectIDFromParam(ctx)
	if err != nil {
		ctx.Error(err)
		return
	}

	fileHeader, err := ctx.FormFile("file")
	if err != nil {
		ctx.Error(httpresp.NewAppError(http.StatusBadRequest, "validation_error", "file is required", nil))
		return
	}

	key := strings.TrimSpace(ctx.PostForm("key"))
	if key == "" {
		key = fileHeader.Filename
	}
	if key == "" {
		ctx.Error(httpresp.NewAppError(http.StatusBadRequest, "validation_error", "object key is required", nil))
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		ctx.Error(httpresp.NewAppError(http.StatusBadRequest, "validation_error", "file could not be opened", nil))
		return
	}
	defer file.Close()

	contentType := fileContentType(fileHeader)
	err = h.projectService.UploadBucketObject(ctx.Request.Context(), projectID, serviceprojects.UploadBucketObjectInput{
		BucketName:  ctx.PostForm("bucketName"),
		Key:         key,
		ContentType: contentType,
		Content:     file,
		Size:        fileHeader.Size,
	})
	if err != nil {
		h.recordAudit(ctx, projectID, "object.upload", "object", key, model.AuditResultFailure, gin.H{"error": err.Error()})
		ctx.Error(err)
		return
	}
	h.recordAudit(ctx, projectID, "object.upload", "object", key, model.AuditResultSuccess, gin.H{
		"size": fileHeader.Size,
	})
	httpresp.Success(ctx, gin.H{"message": "upload accepted"})
}

func (h *Handler) DownloadObject(ctx *gin.Context) {
	projectID, err := projectIDFromParam(ctx)
	if err != nil {
		ctx.Error(err)
		return
	}

	key := strings.TrimSpace(ctx.Query("key"))
	if key == "" {
		ctx.Error(httpresp.NewAppError(http.StatusBadRequest, "validation_error", "object key is required", nil))
		return
	}

	reader, meta, err := h.projectService.DownloadBucketObject(ctx.Request.Context(), projectID, serviceprojects.DownloadBucketObjectInput{
		BucketName: ctx.Query("bucketName"),
		Key:        key,
	})
	if err != nil {
		h.recordAudit(ctx, projectID, "object.download", "object", key, model.AuditResultFailure, gin.H{"error": err.Error()})
		ctx.Error(err)
		return
	}
	defer reader.Close()

	h.recordAudit(ctx, projectID, "object.download", "object", key, model.AuditResultSuccess, nil)
	extraHeaders := map[string]string{
		"Content-Disposition": `attachment; filename="` + key + `"`,
	}
	contentType := meta.ContentType
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/octet-stream"
	}
	ctx.DataFromReader(http.StatusOK, meta.ContentLength, contentType, reader, extraHeaders)
}

func (h *Handler) ListAuditLogs(ctx *gin.Context) {
	projectID, err := projectIDFromParam(ctx)
	if err != nil {
		ctx.Error(err)
		return
	}

	limit := 20
	if raw := ctx.Query("limit"); raw != "" {
		parsed, parseErr := strconv.Atoi(raw)
		if parseErr != nil || parsed <= 0 {
			ctx.Error(httpresp.NewAppError(http.StatusBadRequest, "validation_error", "limit must be a positive integer", nil))
			return
		}
		limit = parsed
	}

	offset := 0
	if raw := ctx.Query("offset"); raw != "" {
		parsed, parseErr := strconv.Atoi(raw)
		if parseErr != nil || parsed < 0 {
			ctx.Error(httpresp.NewAppError(http.StatusBadRequest, "validation_error", "offset must be zero or a positive integer", nil))
			return
		}
		offset = parsed
	}

	logs, err := h.audits.List(ctx.Request.Context(), repository.AuditLogFilter{
		ProjectID:        &projectID,
		Action:           ctx.Query("action"),
		TargetType:       "object",
		TargetIdentifier: ctx.Query("path"),
		Result:           ctx.Query("result"),
		Limit:            limit,
		Offset:           offset,
	})
	if err != nil {
		ctx.Error(err)
		return
	}

	httpresp.Success(ctx, listAuditLogsResponse{
		Logs: toAuditLogResponses(logs),
	})
}

func (h *Handler) DeleteObject(ctx *gin.Context) {
	projectID, err := projectIDFromParam(ctx)
	if err != nil {
		ctx.Error(err)
		return
	}

	key := strings.TrimSpace(ctx.Query("key"))
	if key == "" {
		ctx.Error(httpresp.NewAppError(http.StatusBadRequest, "validation_error", "object key is required", nil))
		return
	}

	err = h.projectService.DeleteBucketObject(ctx.Request.Context(), projectID, serviceprojects.DeleteBucketObjectInput{
		BucketName: ctx.Query("bucketName"),
		Key:        key,
	})
	if err != nil {
		h.recordAudit(ctx, projectID, "object.delete", "object", key, model.AuditResultFailure, gin.H{"error": err.Error()})
		ctx.Error(err)
		return
	}

	h.recordAudit(ctx, projectID, "object.delete", "object", key, model.AuditResultSuccess, nil)
	httpresp.Success(ctx, gin.H{"message": "object deleted"})
}

func (h *Handler) RenameObject(ctx *gin.Context) {
	projectID, err := projectIDFromParam(ctx)
	if err != nil {
		ctx.Error(err)
		return
	}

	var req renameObjectRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.Error(httpresp.NewAppError(http.StatusBadRequest, "validation_error", "invalid rename object request", gin.H{"error": err.Error()}))
		return
	}

	err = h.projectService.RenameBucketObject(ctx.Request.Context(), projectID, serviceprojects.RenameBucketObjectInput{
		BucketName: req.BucketName,
		SourceKey:  req.SourceKey,
		TargetKey:  req.TargetKey,
	})
	if err != nil {
		h.recordAudit(ctx, projectID, "object.rename", "object", req.SourceKey+" -> "+req.TargetKey, model.AuditResultFailure, gin.H{"error": err.Error()})
		ctx.Error(err)
		return
	}

	h.recordAudit(ctx, projectID, "object.rename", "object", req.SourceKey+" -> "+req.TargetKey, model.AuditResultSuccess, nil)
	httpresp.Success(ctx, gin.H{"message": "object renamed"})
}

func projectIDFromParam(ctx *gin.Context) (uint64, error) {
	projectID, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return 0, httpresp.NewAppError(http.StatusBadRequest, "validation_error", "project id must be a positive integer", nil)
	}
	return projectID, nil
}

func toObjectResponses(objects []provider.ObjectInfo) []objectResponse {
	resp := make([]objectResponse, 0, len(objects))
	for _, object := range objects {
		lastModified := ""
		if !object.LastModified.IsZero() {
			lastModified = object.LastModified.Format(time.RFC3339)
		}
		resp = append(resp, objectResponse{
			Key:          object.Key,
			ETag:         object.ETag,
			ContentType:  object.ContentType,
			Size:         object.Size,
			LastModified: lastModified,
			IsDir:        object.IsDir,
		})
	}
	return resp
}

func fileContentType(fileHeader *multipart.FileHeader) string {
	if fileHeader == nil {
		return "application/octet-stream"
	}
	contentType := strings.TrimSpace(fileHeader.Header.Get("Content-Type"))
	if contentType == "" {
		return "application/octet-stream"
	}
	return contentType
}

func objectTargetIdentifier(prefix, marker string) string {
	if prefix == "" && marker == "" {
		return "list"
	}
	if marker == "" {
		return "list:" + prefix
	}
	return "list:" + prefix + ":" + marker
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

func (h *Handler) recordAudit(ctx *gin.Context, projectID uint64, action, targetType, targetIdentifier, result string, details gin.H) {
	if h == nil || h.audits == nil {
		return
	}
	userID, ok := middleware.CurrentUserID(ctx)
	if !ok {
		return
	}

	var metadata datatypes.JSON
	if details != nil {
		raw, err := json.Marshal(details)
		if err == nil {
			metadata = datatypes.JSON(raw)
		}
	}

	_ = h.audits.Create(ctx.Request.Context(), &model.AuditLog{
		ActorUserID:      userID,
		ProjectID:        &projectID,
		Action:           action,
		TargetType:       targetType,
		TargetIdentifier: targetIdentifier,
		Result:           result,
		RequestID:        httpresp.GetRequestID(ctx),
		Metadata:         metadata,
	})
}
