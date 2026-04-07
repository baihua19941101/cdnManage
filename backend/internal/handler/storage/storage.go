package storage

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	cryptorand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/baihua19941101/cdnManage/internal/config"
	httpresp "github.com/baihua19941101/cdnManage/internal/http"
	"github.com/baihua19941101/cdnManage/internal/middleware"
	"github.com/baihua19941101/cdnManage/internal/model"
	"github.com/baihua19941101/cdnManage/internal/provider"
	"github.com/baihua19941101/cdnManage/internal/repository"
	auditservice "github.com/baihua19941101/cdnManage/internal/service/audit"
	serviceauth "github.com/baihua19941101/cdnManage/internal/service/auth"
	serviceprojects "github.com/baihua19941101/cdnManage/internal/service/projects"
)

type projectScopeMiddleware interface {
	Middleware() gin.HandlerFunc
}

type Handler struct {
	projectService     *serviceprojects.Service
	audits             repository.AuditLogRepository
	recorder           *auditservice.Recorder
	maxUploadSizeMB    int64
	maxUploadSizeBytes int64
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

type uploadObjectFieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type uploadObjectItemResult struct {
	FileName   string `json:"fileName"`
	Key        string `json:"key,omitempty"`
	Result     string `json:"result"`
	Reason     string `json:"reason,omitempty"`
	ErrorCode  string `json:"errorCode,omitempty"`
	Size       int64  `json:"size,omitempty"`
	LimitBytes int64  `json:"limitBytes,omitempty"`
	LimitMB    int64  `json:"limitMB,omitempty"`
}

type uploadObjectSummary struct {
	Total   int `json:"total"`
	Success int `json:"success"`
	Failure int `json:"failure"`
}

type uploadArchiveSummary struct {
	ArchivesProcessed int `json:"archivesProcessed"`
	Extracted         int `json:"extracted"`
	Uploaded          int `json:"uploaded"`
	Failed            int `json:"failed"`
	Skipped           int `json:"skipped"`
}

type uploadObjectResponse struct {
	Message        string                   `json:"message"`
	Summary        uploadObjectSummary      `json:"summary"`
	Results        []uploadObjectItemResult `json:"results"`
	ArchiveSummary *uploadArchiveSummary    `json:"archiveSummary,omitempty"`
	SessionID      *string                  `json:"sessionId,omitempty"`
	StartedAt      *string                  `json:"startedAt,omitempty"`
	FinishedAt     *string                  `json:"finishedAt,omitempty"`
	DurationMs     *int64                   `json:"durationMs,omitempty"`
	TotalEntries   *int                     `json:"totalEntries,omitempty"`
	SuccessEntries *int                     `json:"successEntries,omitempty"`
	FailedEntries  *int                     `json:"failedEntries,omitempty"`
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

type uploadPolicyResponse struct {
	MaxUploadSizeMB    int64 `json:"maxUploadSizeMB"`
	MaxUploadSizeBytes int64 `json:"maxUploadSizeBytes"`
}

type renameObjectRequest struct {
	BucketName string `json:"bucketName"`
	SourceKey  string `json:"sourceKey" binding:"required"`
	TargetKey  string `json:"targetKey" binding:"required"`
}

func NewHandler(projectService *serviceprojects.Service, audits repository.AuditLogRepository, maxUploadSizeMB int64) *Handler {
	if maxUploadSizeMB <= 0 {
		maxUploadSizeMB = config.DefaultMaxUploadFileSizeMB
	}

	return &Handler{
		projectService:     projectService,
		audits:             audits,
		recorder:           auditservice.NewRecorder(audits),
		maxUploadSizeMB:    maxUploadSizeMB,
		maxUploadSizeBytes: (config.UploadConfig{MaxFileSizeMB: maxUploadSizeMB}).MaxFileSizeBytes(),
	}
}

func RegisterRoutes(router gin.IRouter, handler *Handler, authenticator *serviceauth.Service, projectScope projectScopeMiddleware) {
	authenticatedGroup := router.Group("/api/v1/storage")
	authenticatedGroup.Use(middleware.Authentication(authenticator))
	authenticatedGroup.GET("/upload-policy", handler.GetUploadPolicy)

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

func (h *Handler) GetUploadPolicy(ctx *gin.Context) {
	httpresp.Success(ctx, uploadPolicyResponse{
		MaxUploadSizeMB:    h.maxUploadSizeMB,
		MaxUploadSizeBytes: h.maxUploadSizeBytes,
	})
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

	fileHeaders, err := collectUploadFileHeaders(ctx)
	if err != nil {
		ctx.Error(err)
		return
	}

	key, hasKey := ctx.GetPostForm("key")
	key = strings.TrimSpace(key)
	if hasKey && key == "" {
		ctx.Error(uploadValidationError("key must not be empty", []uploadObjectFieldError{{
			Field:   "key",
			Message: "key must not be empty when provided",
		}}))
		return
	}

	keyPrefix, hasKeyPrefix := ctx.GetPostForm("keyPrefix")
	keyPrefix = strings.TrimSpace(keyPrefix)
	if hasKeyPrefix && keyPrefix == "" {
		ctx.Error(uploadValidationError("keyPrefix must not be empty", []uploadObjectFieldError{{
			Field:   "keyPrefix",
			Message: "keyPrefix must not be empty when provided",
		}}))
		return
	}
	if len(fileHeaders) > 1 && key != "" {
		ctx.Error(uploadValidationError("key is only supported for single file upload", []uploadObjectFieldError{{
			Field:   "key",
			Message: "use keyPrefix for multi-file upload",
		}}))
		return
	}

	bucketName := ctx.PostForm("bucketName")
	results := make([]uploadObjectItemResult, 0, len(fileHeaders))
	summary := uploadObjectSummary{}
	var archiveSummary *uploadArchiveSummary
	var archiveSessionID string
	var archiveStartedAt time.Time
	ensureArchiveSession := func() {
		archiveSummary = ensureArchiveSummary(archiveSummary)
		if archiveSessionID == "" {
			archiveStartedAt = time.Now().UTC()
			archiveSessionID = newArchiveUploadSessionID(archiveStartedAt)
		}
	}

	for _, fileHeader := range fileHeaders {
		fileName := ""
		if fileHeader != nil {
			fileName = strings.TrimSpace(fileHeader.Filename)
		}

		if fileHeader == nil {
			appendUploadResult(&results, &summary, uploadObjectItemResult{
				FileName: fileName,
				Result:   "failure",
				Reason:   "file is required",
			})
			h.recordAudit(ctx, projectID, "object.upload", "object", uploadAuditTarget("", fileName), model.AuditResultFailure, gin.H{
				"error":    "file header is missing",
				"fileName": fileName,
			})
			continue
		}

		archiveFormatByName := detectArchiveFormatByName(fileHeader.Filename)
		file, openErr := fileHeader.Open()
		if openErr != nil {
			reason := "file could not be opened"
			metadata := gin.H{
				"error":    openErr.Error(),
				"fileName": fileName,
			}
			if archiveFormatByName != "" {
				reason = "archive could not be opened"
				metadata["archive"] = true
				ensureArchiveSession()
				archiveSummary.ArchivesProcessed++
				archiveSummary.Failed++
				metadata["sessionId"] = archiveSessionID
			}
			appendUploadResult(&results, &summary, uploadObjectItemResult{
				FileName: fileName,
				Result:   "failure",
				Reason:   reason,
			})
			h.recordAudit(ctx, projectID, "object.upload", "object", uploadAuditTarget("", fileName), model.AuditResultFailure, metadata)
			continue
		}

		archiveFormat, detectErr := detectArchiveFormat(fileHeader.Filename, file)
		if detectErr != nil {
			_ = file.Close()
			metadata := gin.H{
				"error":    detectErr.Error(),
				"fileName": fileName,
			}
			if archiveFormatByName != "" {
				ensureArchiveSession()
				archiveSummary.ArchivesProcessed++
				archiveSummary.Failed++
				metadata["archive"] = true
				metadata["sessionId"] = archiveSessionID
			}
			appendUploadResult(&results, &summary, uploadObjectItemResult{
				FileName: fileName,
				Result:   "failure",
				Reason:   "file could not be read",
			})
			h.recordAudit(ctx, projectID, "object.upload", "object", uploadAuditTarget("", fileName), model.AuditResultFailure, metadata)
			continue
		}

		if archiveFormat != "" {
			ensureArchiveSession()
			archiveSummary.ArchivesProcessed++

			stats := h.uploadArchiveEntries(
				ctx,
				projectID,
				bucketName,
				fileHeader,
				file,
				resolveArchiveUploadBaseKey(key, keyPrefix, len(fileHeaders)),
				archiveFormat,
				archiveSessionID,
				&results,
				&summary,
			)
			archiveSummary.Extracted += stats.Extracted
			archiveSummary.Uploaded += stats.Uploaded
			archiveSummary.Failed += stats.Failed
			archiveSummary.Skipped += stats.Skipped
			_ = file.Close()
			continue
		}

		targetKey, keyErr := resolveUploadObjectKey(fileHeader, key, keyPrefix, len(fileHeaders))
		if keyErr != nil {
			_ = file.Close()
			appendUploadResult(&results, &summary, uploadObjectItemResult{
				FileName: fileName,
				Key:      targetKey,
				Result:   "failure",
				Reason:   keyErr.Error(),
			})
			h.recordAudit(ctx, projectID, "object.upload", "object", uploadAuditTarget(targetKey, fileName), model.AuditResultFailure, gin.H{
				"error":    keyErr.Error(),
				"fileName": fileName,
			})
			continue
		}
		if h.isUploadSizeExceeded(fileHeader.Size) {
			_ = file.Close()
			h.recordUploadTooLarge(ctx, projectID, fileName, targetKey, fileHeader.Size, "", "", "")
			appendUploadResult(&results, &summary, h.uploadTooLargeResult(fileName, targetKey, fileHeader.Size))
			continue
		}

		contentType := fileContentType(fileHeader)
		uploadErr := h.projectService.UploadBucketObject(ctx.Request.Context(), projectID, serviceprojects.UploadBucketObjectInput{
			BucketName:  bucketName,
			Key:         targetKey,
			ContentType: contentType,
			Content:     file,
			Size:        fileHeader.Size,
		})
		_ = file.Close()
		if uploadErr != nil {
			appendUploadResult(&results, &summary, uploadObjectItemResult{
				FileName: fileName,
				Key:      targetKey,
				Result:   "failure",
				Reason:   uploadErr.Error(),
			})
			h.recordAudit(ctx, projectID, "object.upload", "object", uploadAuditTarget(targetKey, fileName), model.AuditResultFailure, gin.H{
				"error":    uploadErr.Error(),
				"size":     fileHeader.Size,
				"fileName": fileName,
			})
			continue
		}

		appendUploadResult(&results, &summary, uploadObjectItemResult{
			FileName: fileName,
			Key:      targetKey,
			Result:   "success",
		})
		h.recordAudit(ctx, projectID, "object.upload", "object", uploadAuditTarget(targetKey, fileName), model.AuditResultSuccess, gin.H{
			"size":     fileHeader.Size,
			"fileName": fileName,
		})
	}

	response := uploadObjectResponse{
		Message: "upload accepted",
		Summary: summary,
		Results: results,
	}
	if archiveSummary != nil {
		finishedAt := time.Now().UTC()
		durationMs := finishedAt.Sub(archiveStartedAt).Milliseconds()
		if durationMs < 0 {
			durationMs = 0
		}
		startedAtText := archiveStartedAt.Format(time.RFC3339)
		finishedAtText := finishedAt.Format(time.RFC3339)
		totalEntries := archiveSummary.Uploaded + archiveSummary.Failed
		successEntries := archiveSummary.Uploaded
		failedEntries := archiveSummary.Failed
		response.ArchiveSummary = archiveSummary
		response.SessionID = &archiveSessionID
		response.StartedAt = &startedAtText
		response.FinishedAt = &finishedAtText
		response.DurationMs = &durationMs
		response.TotalEntries = &totalEntries
		response.SuccessEntries = &successEntries
		response.FailedEntries = &failedEntries

		archiveAuditResult := model.AuditResultSuccess
		if archiveSummary.Failed > 0 {
			archiveAuditResult = model.AuditResultFailure
		}
		h.recordAudit(ctx, projectID, "object.upload_archive", "object", archiveSessionID, archiveAuditResult, gin.H{
			"sessionId":         archiveSessionID,
			"startedAt":         startedAtText,
			"finishedAt":        finishedAtText,
			"durationMs":        durationMs,
			"archivesProcessed": archiveSummary.ArchivesProcessed,
			"extracted":         archiveSummary.Extracted,
			"uploaded":          archiveSummary.Uploaded,
			"failed":            archiveSummary.Failed,
			"skipped":           archiveSummary.Skipped,
			"totalEntries":      totalEntries,
			"successEntries":    successEntries,
			"failedEntries":     failedEntries,
		})
	}

	httpresp.Success(ctx, response)
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
		SessionID:        strings.TrimSpace(ctx.Query("sessionId")),
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

func collectUploadFileHeaders(ctx *gin.Context) ([]*multipart.FileHeader, error) {
	form, err := ctx.MultipartForm()
	if err != nil {
		return nil, uploadValidationError("invalid upload request", []uploadObjectFieldError{{
			Field:   "files",
			Message: "multipart form data is required",
		}})
	}
	if form == nil {
		return nil, uploadValidationError("invalid upload request", []uploadObjectFieldError{{
			Field:   "files",
			Message: "multipart form data is required",
		}})
	}

	files := make([]*multipart.FileHeader, 0)
	files = append(files, form.File["files"]...)
	files = append(files, form.File["file"]...)
	if len(files) == 0 {
		return nil, uploadValidationError("file is required", []uploadObjectFieldError{{
			Field:   "files",
			Message: "at least one file must be provided in files or file",
		}})
	}
	return files, nil
}

func resolveUploadObjectKey(fileHeader *multipart.FileHeader, key, keyPrefix string, fileCount int) (string, error) {
	if fileCount == 1 && key != "" {
		return key, nil
	}

	fileName := ""
	if fileHeader != nil {
		fileName = strings.TrimSpace(fileHeader.Filename)
	}
	if fileName == "" {
		return "", uploadValidationError("object key is required", []uploadObjectFieldError{{
			Field:   "fileName",
			Message: "file name is required when object key is not provided",
		}})
	}

	return joinObjectKey(keyPrefix, fileName), nil
}

func joinObjectKey(keyPrefix, fileName string) string {
	keyPrefix = strings.TrimSpace(keyPrefix)
	fileName = strings.TrimSpace(fileName)
	if keyPrefix == "" {
		return fileName
	}
	return strings.TrimRight(keyPrefix, "/") + "/" + fileName
}

func uploadAuditTarget(key, fileName string) string {
	key = strings.TrimSpace(key)
	if key != "" {
		return key
	}
	fileName = strings.TrimSpace(fileName)
	if fileName != "" {
		return fileName
	}
	return "upload"
}

func appendUploadResult(results *[]uploadObjectItemResult, summary *uploadObjectSummary, item uploadObjectItemResult) {
	if results != nil {
		*results = append(*results, item)
	}
	if summary == nil {
		return
	}

	switch strings.ToLower(strings.TrimSpace(item.Result)) {
	case "success":
		summary.Total++
		summary.Success++
	case "failure":
		summary.Total++
		summary.Failure++
	}
}

const archiveSniffSize = 512

func detectArchiveFormat(fileName string, file multipart.File) (string, error) {
	if contentFormat, err := detectArchiveFormatFromContent(file); err != nil {
		return "", err
	} else if contentFormat != "" {
		return contentFormat, nil
	}
	return detectArchiveFormatByName(fileName), nil
}

func detectArchiveFormatByName(fileName string) string {
	lowerName := strings.ToLower(strings.TrimSpace(fileName))
	switch {
	case strings.HasSuffix(lowerName, ".tar.gz"), strings.HasSuffix(lowerName, ".tgz"):
		return "tar.gz"
	case strings.HasSuffix(lowerName, ".tar"):
		return "tar"
	case strings.HasSuffix(lowerName, ".zip"):
		return "zip"
	default:
		return ""
	}
}

func detectArchiveFormatFromContent(file multipart.File) (string, error) {
	header, err := peekFileHeader(file, archiveSniffSize)
	if err != nil {
		return "", err
	}

	switch {
	case hasZipSignature(header):
		return "zip", nil
	case hasTarUstarHeader(header):
		return "tar", nil
	case hasGzipSignature(header):
		isTarGz, err := gzipContainsTarHeader(file)
		if err != nil {
			return "", err
		}
		if isTarGz {
			return "tar.gz", nil
		}
	}

	return "", nil
}

func peekFileHeader(file multipart.File, size int) ([]byte, error) {
	if file == nil || size <= 0 {
		return nil, nil
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	buffer := make([]byte, size)
	n, err := io.ReadFull(file, buffer)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, err
	}
	if _, seekErr := file.Seek(0, io.SeekStart); seekErr != nil {
		return nil, seekErr
	}

	return buffer[:n], nil
}

func hasZipSignature(header []byte) bool {
	return len(header) >= 2 && header[0] == 'P' && header[1] == 'K'
}

func hasGzipSignature(header []byte) bool {
	return len(header) >= 2 && header[0] == 0x1f && header[1] == 0x8b
}

func hasTarUstarHeader(header []byte) bool {
	if len(header) < archiveSniffSize {
		return false
	}
	return bytes.HasPrefix(header[257:263], []byte("ustar"))
}

func gzipContainsTarHeader(file multipart.File) (bool, error) {
	if file == nil {
		return false, nil
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return false, err
	}
	defer func() {
		_, _ = file.Seek(0, io.SeekStart)
	}()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return false, nil
	}
	defer gzipReader.Close()

	buffer := make([]byte, archiveSniffSize)
	n, readErr := io.ReadFull(gzipReader, buffer)
	if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
		return false, nil
	}

	return hasTarUstarHeader(buffer[:n]), nil
}

func resolveArchiveUploadBaseKey(key, keyPrefix string, fileCount int) string {
	if fileCount == 1 && strings.TrimSpace(key) != "" {
		return strings.TrimSpace(key)
	}
	return strings.TrimSpace(keyPrefix)
}

func ensureArchiveSummary(summary *uploadArchiveSummary) *uploadArchiveSummary {
	if summary != nil {
		return summary
	}
	return &uploadArchiveSummary{}
}

func newArchiveUploadSessionID(startedAt time.Time) string {
	startedAt = startedAt.UTC()
	suffix := make([]byte, 4)
	if _, err := cryptorand.Read(suffix); err != nil {
		return fmt.Sprintf("archive-%d-%d", startedAt.UnixMilli(), startedAt.UnixNano())
	}
	return fmt.Sprintf("archive-%d-%s", startedAt.UnixMilli(), hex.EncodeToString(suffix))
}

func (h *Handler) uploadArchiveEntries(
	ctx *gin.Context,
	projectID uint64,
	bucketName string,
	fileHeader *multipart.FileHeader,
	file multipart.File,
	baseKey string,
	archiveFormat string,
	sessionID string,
	results *[]uploadObjectItemResult,
	summary *uploadObjectSummary,
) uploadArchiveSummary {
	switch archiveFormat {
	case "zip":
		return h.uploadZipEntries(ctx, projectID, bucketName, fileHeader, file, baseKey, sessionID, results, summary)
	case "tar":
		return h.uploadTarEntries(ctx, projectID, bucketName, fileHeader.Filename, tar.NewReader(file), baseKey, sessionID, results, summary)
	case "tar.gz":
		gzipReader, err := gzip.NewReader(file)
		if err != nil {
			return h.recordArchiveFailure(ctx, projectID, fileHeader.Filename, "", "", "archive could not be decompressed", err, sessionID, results, summary)
		}
		defer gzipReader.Close()

		return h.uploadTarEntries(ctx, projectID, bucketName, fileHeader.Filename, tar.NewReader(gzipReader), baseKey, sessionID, results, summary)
	default:
		return h.recordArchiveFailure(ctx, projectID, fileHeader.Filename, "", "", "archive format is not supported", nil, sessionID, results, summary)
	}
}

func (h *Handler) uploadZipEntries(
	ctx *gin.Context,
	projectID uint64,
	bucketName string,
	fileHeader *multipart.FileHeader,
	file multipart.File,
	baseKey string,
	sessionID string,
	results *[]uploadObjectItemResult,
	summary *uploadObjectSummary,
) uploadArchiveSummary {
	reader, err := zip.NewReader(file, fileHeader.Size)
	if err != nil {
		return h.recordArchiveFailure(ctx, projectID, fileHeader.Filename, "", "", "archive could not be decompressed", err, sessionID, results, summary)
	}

	stats := uploadArchiveSummary{}
	for _, entry := range reader.File {
		rawName := strings.TrimSpace(entry.Name)
		if entry.FileInfo().IsDir() {
			stats.Skipped++
			continue
		}
		if !entry.FileInfo().Mode().IsRegular() {
			failure := h.recordArchiveFailure(ctx, projectID, fileHeader.Filename, rawName, "", "archive entry type is not supported", nil, sessionID, results, summary)
			stats.Failed += failure.Failed
			continue
		}

		stats.Extracted++
		entryPath, sanitizeErr := sanitizeArchiveEntryPath(rawName)
		if sanitizeErr != nil {
			failure := h.recordArchiveFailure(ctx, projectID, fileHeader.Filename, rawName, "", sanitizeErr.Error(), nil, sessionID, results, summary)
			stats.Failed += failure.Failed
			continue
		}

		entryReader, openErr := entry.Open()
		if openErr != nil {
			failure := h.recordArchiveFailure(ctx, projectID, fileHeader.Filename, rawName, "", "archive entry could not be opened", openErr, sessionID, results, summary)
			stats.Failed += failure.Failed
			continue
		}

		targetKey := joinObjectKey(baseKey, entryPath)
		entrySize := int64(entry.UncompressedSize64)
		if h.isUploadSizeExceeded(entrySize) {
			_ = entryReader.Close()
			appendUploadResult(results, summary, h.uploadTooLargeResult(archiveResultFileName(fileHeader.Filename, entryPath), targetKey, entrySize))
			h.recordUploadTooLarge(ctx, projectID, archiveResultFileName(fileHeader.Filename, entryPath), targetKey, entrySize, fileHeader.Filename, rawName, sessionID)
			stats.Failed++
			continue
		}

		uploadErr := h.projectService.UploadBucketObject(ctx.Request.Context(), projectID, serviceprojects.UploadBucketObjectInput{
			BucketName:  bucketName,
			Key:         targetKey,
			ContentType: archiveEntryContentType(entryPath),
			Content:     entryReader,
			Size:        entrySize,
		})
		_ = entryReader.Close()
		if uploadErr != nil {
			failure := h.recordArchiveFailure(ctx, projectID, fileHeader.Filename, rawName, targetKey, uploadErr.Error(), nil, sessionID, results, summary)
			stats.Failed += failure.Failed
			continue
		}

		appendUploadResult(results, summary, uploadObjectItemResult{
			FileName: archiveResultFileName(fileHeader.Filename, entryPath),
			Key:      targetKey,
			Result:   "success",
		})
		stats.Uploaded++
		h.recordAudit(ctx, projectID, "object.upload", "object", uploadAuditTarget(targetKey, archiveResultFileName(fileHeader.Filename, entryPath)), model.AuditResultSuccess, gin.H{
			"size":        int64(entry.UncompressedSize64),
			"fileName":    entryPath,
			"archiveFile": strings.TrimSpace(fileHeader.Filename),
			"sessionId":   sessionID,
		})
	}

	return stats
}

func (h *Handler) uploadTarEntries(
	ctx *gin.Context,
	projectID uint64,
	bucketName string,
	archiveName string,
	reader *tar.Reader,
	baseKey string,
	sessionID string,
	results *[]uploadObjectItemResult,
	summary *uploadObjectSummary,
) uploadArchiveSummary {
	stats := uploadArchiveSummary{}

	for {
		header, err := reader.Next()
		if err == io.EOF {
			return stats
		}
		if err != nil {
			failure := h.recordArchiveFailure(ctx, projectID, archiveName, "", "", "archive could not be decompressed", err, sessionID, results, summary)
			stats.Failed += failure.Failed
			return stats
		}

		rawName := strings.TrimSpace(header.Name)
		if header.FileInfo().IsDir() {
			stats.Skipped++
			continue
		}
		if !header.FileInfo().Mode().IsRegular() {
			failure := h.recordArchiveFailure(ctx, projectID, archiveName, rawName, "", "archive entry type is not supported", nil, sessionID, results, summary)
			stats.Failed += failure.Failed
			continue
		}

		stats.Extracted++
		entryPath, sanitizeErr := sanitizeArchiveEntryPath(rawName)
		if sanitizeErr != nil {
			failure := h.recordArchiveFailure(ctx, projectID, archiveName, rawName, "", sanitizeErr.Error(), nil, sessionID, results, summary)
			stats.Failed += failure.Failed
			continue
		}

		targetKey := joinObjectKey(baseKey, entryPath)
		if h.isUploadSizeExceeded(header.Size) {
			appendUploadResult(results, summary, h.uploadTooLargeResult(archiveResultFileName(archiveName, entryPath), targetKey, header.Size))
			h.recordUploadTooLarge(ctx, projectID, archiveResultFileName(archiveName, entryPath), targetKey, header.Size, archiveName, rawName, sessionID)
			stats.Failed++
			continue
		}

		uploadErr := h.projectService.UploadBucketObject(ctx.Request.Context(), projectID, serviceprojects.UploadBucketObjectInput{
			BucketName:  bucketName,
			Key:         targetKey,
			ContentType: archiveEntryContentType(entryPath),
			Content:     reader,
			Size:        header.Size,
		})
		if uploadErr != nil {
			failure := h.recordArchiveFailure(ctx, projectID, archiveName, rawName, targetKey, uploadErr.Error(), nil, sessionID, results, summary)
			stats.Failed += failure.Failed
			continue
		}

		appendUploadResult(results, summary, uploadObjectItemResult{
			FileName: archiveResultFileName(archiveName, entryPath),
			Key:      targetKey,
			Result:   "success",
		})
		stats.Uploaded++
		h.recordAudit(ctx, projectID, "object.upload", "object", uploadAuditTarget(targetKey, archiveResultFileName(archiveName, entryPath)), model.AuditResultSuccess, gin.H{
			"size":        header.Size,
			"fileName":    entryPath,
			"archiveFile": strings.TrimSpace(archiveName),
			"sessionId":   sessionID,
		})
	}
}

func (h *Handler) recordArchiveFailure(
	ctx *gin.Context,
	projectID uint64,
	archiveName string,
	entryName string,
	targetKey string,
	reason string,
	err error,
	sessionID string,
	results *[]uploadObjectItemResult,
	summary *uploadObjectSummary,
) uploadArchiveSummary {
	fileName := archiveResultFileName(archiveName, entryName)
	appendUploadResult(results, summary, uploadObjectItemResult{
		FileName: fileName,
		Key:      strings.TrimSpace(targetKey),
		Result:   "failure",
		Reason:   reason,
	})

	metadata := gin.H{
		"fileName": fileName,
		"archive":  true,
	}
	if strings.TrimSpace(archiveName) != "" {
		metadata["archiveFile"] = strings.TrimSpace(archiveName)
	}
	if strings.TrimSpace(entryName) != "" {
		metadata["archiveEntry"] = strings.TrimSpace(entryName)
	}
	if strings.TrimSpace(sessionID) != "" {
		metadata["sessionId"] = strings.TrimSpace(sessionID)
	}
	if err != nil {
		metadata["error"] = err.Error()
	} else {
		metadata["error"] = reason
	}

	h.recordAudit(ctx, projectID, "object.upload", "object", uploadAuditTarget(targetKey, fileName), model.AuditResultFailure, metadata)
	return uploadArchiveSummary{Failed: 1}
}

func sanitizeArchiveEntryPath(entryName string) (string, error) {
	normalized := strings.TrimSpace(entryName)
	if normalized == "" {
		return "", uploadValidationError("archive entry path is empty", nil)
	}

	normalized = strings.ReplaceAll(normalized, "\\", "/")
	if strings.Contains(normalized, "\x00") {
		return "", uploadValidationError("archive entry path contains invalid characters", nil)
	}
	if hasWindowsDrivePrefix(normalized) {
		return "", uploadValidationError("archive entry path must be relative", nil)
	}
	if path.IsAbs(normalized) {
		return "", uploadValidationError("archive entry path must be relative", nil)
	}

	cleanPath := path.Clean(normalized)
	switch {
	case cleanPath == ".", cleanPath == "":
		return "", uploadValidationError("archive entry path is empty", nil)
	case cleanPath == "..", strings.HasPrefix(cleanPath, "../"):
		return "", uploadValidationError("archive entry path must not escape the archive root", nil)
	case path.IsAbs(cleanPath):
		return "", uploadValidationError("archive entry path must be relative", nil)
	}

	return strings.TrimPrefix(cleanPath, "./"), nil
}

func hasWindowsDrivePrefix(value string) bool {
	if len(value) < 2 || value[1] != ':' {
		return false
	}
	first := value[0]
	return (first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z')
}

func archiveEntryContentType(entryPath string) string {
	contentType := strings.TrimSpace(mime.TypeByExtension(strings.ToLower(path.Ext(entryPath))))
	if contentType == "" {
		return "application/octet-stream"
	}
	return contentType
}

func archiveResultFileName(archiveName, entryName string) string {
	archiveName = strings.TrimSpace(archiveName)
	entryName = strings.TrimSpace(entryName)
	switch {
	case archiveName == "":
		return entryName
	case entryName == "":
		return archiveName
	default:
		return archiveName + "!" + entryName
	}
}

func (h *Handler) isUploadSizeExceeded(size int64) bool {
	return h != nil && size > 0 && h.maxUploadSizeBytes > 0 && size > h.maxUploadSizeBytes
}

func (h *Handler) uploadTooLargeResult(fileName, targetKey string, size int64) uploadObjectItemResult {
	return uploadObjectItemResult{
		FileName:   strings.TrimSpace(fileName),
		Key:        strings.TrimSpace(targetKey),
		Result:     "failure",
		Reason:     fmt.Sprintf("file size exceeds limit of %d MB", h.maxUploadSizeMB),
		ErrorCode:  "upload_file_too_large",
		Size:       size,
		LimitBytes: h.maxUploadSizeBytes,
		LimitMB:    h.maxUploadSizeMB,
	}
}

func (h *Handler) recordUploadTooLarge(ctx *gin.Context, projectID uint64, fileName, targetKey string, size int64, archiveName, entryName, sessionID string) {
	metadata := gin.H{
		"error":     "file size exceeds upload limit",
		"errorCode": "upload_file_too_large",
		"fileName":  strings.TrimSpace(fileName),
		"size":      size,
		"limit":     h.maxUploadSizeBytes,
		"limitMB":   h.maxUploadSizeMB,
	}
	if strings.TrimSpace(archiveName) != "" {
		metadata["archive"] = true
		metadata["archiveFile"] = strings.TrimSpace(archiveName)
	}
	if strings.TrimSpace(entryName) != "" {
		metadata["archiveEntry"] = strings.TrimSpace(entryName)
	}
	if strings.TrimSpace(sessionID) != "" {
		metadata["sessionId"] = strings.TrimSpace(sessionID)
	}

	h.recordAudit(ctx, projectID, "object.upload", "object", uploadAuditTarget(targetKey, fileName), model.AuditResultFailure, metadata)
}

func uploadValidationError(message string, fields []uploadObjectFieldError) error {
	details := gin.H{}
	if len(fields) > 0 {
		details["fields"] = fields
	}
	return httpresp.NewAppError(http.StatusBadRequest, "validation_error", message, details)
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
	if h == nil || h.recorder == nil {
		return
	}
	userID, ok := middleware.CurrentUserID(ctx)
	if !ok {
		return
	}
	_ = h.recorder.Record(ctx.Request.Context(), auditservice.RecordInput{
		ActorUserID:      userID,
		ProjectID:        &projectID,
		Action:           action,
		TargetType:       targetType,
		TargetIdentifier: targetIdentifier,
		Result:           result,
		RequestID:        httpresp.GetRequestID(ctx),
		Metadata:         map[string]interface{}(details),
	})
}
