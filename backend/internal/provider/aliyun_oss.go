package provider

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

const (
	aliyunOSSOpDetect   = "detect_provider"
	aliyunOSSOpList     = "list_objects"
	aliyunOSSOpUpload   = "upload_object"
	aliyunOSSOpDownload = "download_object"
	aliyunOSSOpDelete   = "delete_object"
	aliyunOSSOpRename   = "rename_object"
)

// AliyunOSSProvider implements ObjectStorageProvider using Aliyun OSS SDK.
type AliyunOSSProvider struct{}

func NewAliyunOSSProvider() *AliyunOSSProvider {
	return &AliyunOSSProvider{}
}

func (p *AliyunOSSProvider) Type() Type {
	return TypeAliyun
}

func (p *AliyunOSSProvider) Detect(ctx context.Context, credential CredentialPayload, bucket string) (Type, error) {
	bucket = strings.TrimSpace(bucket)
	region := strings.TrimSpace(customField(credential.CustomFields, "aliyun_region", "oss_region", "region"))

	client, err := p.newClient(credential, region)
	if err != nil {
		return TypeUnknown, err
	}
	bkt, err := client.Bucket(bucket)
	if err != nil {
		return TypeUnknown, p.mapError(aliyunOSSOpDetect, err)
	}

	if _, err := bkt.ListObjects(
		oss.MaxKeys(1),
	); err != nil {
		return TypeUnknown, p.mapError(aliyunOSSOpDetect, err)
	}

	return TypeAliyun, nil
}

func (p *AliyunOSSProvider) ListObjects(ctx context.Context, req ListObjectsRequest) ([]ObjectInfo, error) {
	_ = ctx
	client, bkt, err := p.newBucketClient(req.Credential, req.Bucket, req.Region)
	_ = client
	if err != nil {
		return nil, err
	}

	options := []oss.Option{
		oss.Prefix(strings.TrimSpace(req.Prefix)),
		oss.Delimiter("/"),
	}
	if marker := strings.TrimSpace(req.Marker); marker != "" {
		options = append(options, oss.Marker(marker))
	}
	if req.MaxKeys > 0 {
		options = append(options, oss.MaxKeys(req.MaxKeys))
	}

	result, err := bkt.ListObjects(options...)
	if err != nil {
		return nil, p.mapError(aliyunOSSOpList, err)
	}

	objects := make([]ObjectInfo, 0, len(result.Objects)+len(result.CommonPrefixes))
	for _, item := range result.Objects {
		objects = append(objects, ObjectInfo{
			Key:          item.Key,
			ETag:         strings.Trim(item.ETag, "\""),
			Size:         item.Size,
			LastModified: item.LastModified.UTC(),
			IsDir:        false,
		})
	}
	for _, prefix := range result.CommonPrefixes {
		objects = append(objects, ObjectInfo{
			Key:   prefix,
			IsDir: true,
		})
	}

	return objects, nil
}

func (p *AliyunOSSProvider) UploadObject(ctx context.Context, req UploadObjectRequest) error {
	_ = ctx
	_, bkt, err := p.newBucketClient(req.Credential, req.Bucket, req.Region)
	if err != nil {
		return err
	}

	key := strings.TrimSpace(req.Key)
	if key == "" {
		return NewError(TypeAliyun, ServiceObjectStorage, aliyunOSSOpUpload, ErrCodeInvalidRequest, "object key is required", false, nil)
	}
	if req.Content == nil {
		return NewError(TypeAliyun, ServiceObjectStorage, aliyunOSSOpUpload, ErrCodeInvalidRequest, "object content is required", false, nil)
	}

	options := []oss.Option{}
	if ct := strings.TrimSpace(req.ContentType); ct != "" {
		options = append(options, oss.ContentType(ct))
	}
	if err := bkt.PutObject(key, req.Content, options...); err != nil {
		return p.mapError(aliyunOSSOpUpload, err)
	}
	return nil
}

func (p *AliyunOSSProvider) DownloadObject(ctx context.Context, req DownloadObjectRequest) (io.ReadCloser, ObjectMeta, error) {
	_ = ctx
	_, bkt, err := p.newBucketClient(req.Credential, req.Bucket, req.Region)
	if err != nil {
		return nil, ObjectMeta{}, err
	}

	key := strings.TrimSpace(req.Key)
	if key == "" {
		return nil, ObjectMeta{}, NewError(TypeAliyun, ServiceObjectStorage, aliyunOSSOpDownload, ErrCodeInvalidRequest, "object key is required", false, nil)
	}

	metaHeader, err := bkt.GetObjectDetailedMeta(key)
	if err != nil {
		return nil, ObjectMeta{}, p.mapError(aliyunOSSOpDownload, err)
	}
	body, err := bkt.GetObject(key)
	if err != nil {
		return nil, ObjectMeta{}, p.mapError(aliyunOSSOpDownload, err)
	}

	meta := ObjectMeta{
		ContentLength: parseAliyunContentLength(metaHeader.Get("Content-Length")),
		ContentType:   strings.TrimSpace(metaHeader.Get("Content-Type")),
		ETag:          strings.Trim(metaHeader.Get("ETag"), "\""),
		LastModified:  parseAliyunLastModified(metaHeader.Get("Last-Modified")),
	}
	return body, meta, nil
}

func (p *AliyunOSSProvider) DeleteObject(ctx context.Context, req DeleteObjectRequest) error {
	_ = ctx
	_, bkt, err := p.newBucketClient(req.Credential, req.Bucket, req.Region)
	if err != nil {
		return err
	}

	key := strings.TrimSpace(req.Key)
	if key == "" {
		return NewError(TypeAliyun, ServiceObjectStorage, aliyunOSSOpDelete, ErrCodeInvalidRequest, "object key is required", false, nil)
	}
	if err := bkt.DeleteObject(key); err != nil {
		return p.mapError(aliyunOSSOpDelete, err)
	}
	return nil
}

func (p *AliyunOSSProvider) RenameObject(ctx context.Context, req RenameObjectRequest) error {
	_ = ctx
	_, bkt, err := p.newBucketClient(req.Credential, req.Bucket, req.Region)
	if err != nil {
		return err
	}

	sourceKey := strings.TrimSpace(req.SourceKey)
	targetKey := strings.TrimSpace(req.TargetKey)
	if sourceKey == "" || targetKey == "" {
		return NewError(TypeAliyun, ServiceObjectStorage, aliyunOSSOpRename, ErrCodeInvalidRequest, "source key and target key are required", false, nil)
	}
	if sourceKey == targetKey {
		return nil
	}

	if _, err := bkt.CopyObject(sourceKey, targetKey); err != nil {
		return p.mapError(aliyunOSSOpRename, err)
	}
	if err := bkt.DeleteObject(sourceKey); err != nil {
		return p.mapError(aliyunOSSOpRename, err)
	}
	return nil
}

func (p *AliyunOSSProvider) newBucketClient(credential CredentialPayload, bucket, region string) (*oss.Client, *oss.Bucket, error) {
	client, err := p.newClient(credential, region)
	if err != nil {
		return nil, nil, err
	}

	bucket = strings.TrimSpace(bucket)
	if bucket == "" {
		return nil, nil, NewError(TypeAliyun, ServiceObjectStorage, "build_client", ErrCodeInvalidRequest, "bucket is required", false, nil)
	}

	bkt, err := client.Bucket(bucket)
	if err != nil {
		return nil, nil, p.mapError("build_client", err)
	}
	return client, bkt, nil
}

func (p *AliyunOSSProvider) newClient(credential CredentialPayload, region string) (*oss.Client, error) {
	accessKeyID := strings.TrimSpace(credential.AccessKeyID)
	accessKeySecret := strings.TrimSpace(credential.AccessKeySecret)
	if accessKeyID == "" || accessKeySecret == "" {
		return nil, NewError(TypeAliyun, ServiceObjectStorage, "build_client", ErrCodeInvalidCredentials, "access key id and access key secret are required", false, nil)
	}

	endpoint := resolveAliyunOSSEndpoint(credential.CustomFields, region)
	if endpoint == "" {
		return nil, NewError(TypeAliyun, ServiceObjectStorage, "build_client", ErrCodeInvalidRequest, "oss endpoint or region is required", false, nil)
	}

	options := []oss.ClientOption{}
	if token := strings.TrimSpace(credential.SecurityToken); token != "" {
		options = append(options, oss.SecurityToken(token))
	}

	client, err := oss.New(endpoint, accessKeyID, accessKeySecret, options...)
	if err != nil {
		return nil, p.mapError("build_client", err)
	}
	return client, nil
}

func resolveAliyunOSSEndpoint(fields map[string]string, region string) string {
	endpoint := strings.TrimSpace(customField(
		fields,
		"aliyun_oss_endpoint",
		"aliyun_endpoint",
		"oss_endpoint",
		"api_endpoint",
		"endpoint",
	))
	if endpoint == "" {
		region = strings.TrimSpace(region)
		if region == "" {
			region = strings.TrimSpace(customField(fields, "aliyun_region", "oss_region", "region"))
		}
		if region == "" {
			return ""
		}
		endpoint = fmt.Sprintf("https://oss-%s.aliyuncs.com", region)
	}

	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		endpoint = "https://" + endpoint
	}
	return endpoint
}

func parseAliyunContentLength(raw string) int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0
	}
	return value
}

func parseAliyunLastModified(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	if parsed, err := http.ParseTime(raw); err == nil {
		return parsed.UTC()
	}
	for _, layout := range []string{time.RFC3339, time.RFC3339Nano, "2006-01-02T15:04:05.000Z"} {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed.UTC()
		}
	}
	return time.Time{}
}

func (p *AliyunOSSProvider) mapError(operation string, err error) error {
	if err == nil {
		return nil
	}
	if existing, ok := err.(*Error); ok {
		return existing
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return NewError(TypeAliyun, ServiceObjectStorage, operation, ErrCodeTimeout, "request to aliyun oss timed out", true, err)
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return NewError(TypeAliyun, ServiceObjectStorage, operation, ErrCodeTimeout, "request to aliyun oss timed out", true, err)
	}

	var serviceErr oss.ServiceError
	if errors.As(err, &serviceErr) {
		switch {
		case serviceErr.StatusCode == http.StatusUnauthorized || serviceErr.StatusCode == http.StatusForbidden || isAliyunCredentialErrorCode(serviceErr.Code):
			return NewError(TypeAliyun, ServiceObjectStorage, operation, ErrCodeInvalidCredentials, "invalid credentials for aliyun oss", false, err)
		case serviceErr.StatusCode == http.StatusNotFound || isAliyunNotFoundErrorCode(serviceErr.Code):
			return NewError(TypeAliyun, ServiceObjectStorage, operation, ErrCodeNotFound, "resource not found on aliyun oss", false, err)
		case serviceErr.StatusCode == http.StatusRequestTimeout || serviceErr.StatusCode == http.StatusGatewayTimeout:
			return NewError(TypeAliyun, ServiceObjectStorage, operation, ErrCodeTimeout, "request to aliyun oss timed out", true, err)
		default:
			return NewError(TypeAliyun, ServiceObjectStorage, operation, ErrCodeOperationFailed, "aliyun oss operation failed", serviceErr.StatusCode >= 500, err)
		}
	}

	return NewError(TypeAliyun, ServiceObjectStorage, operation, ErrCodeOperationFailed, "aliyun oss operation failed", false, err)
}

func isAliyunCredentialErrorCode(code string) bool {
	switch strings.TrimSpace(code) {
	case "InvalidAccessKeyId", "SignatureDoesNotMatch", "AccessDenied", "SecurityTokenExpired", "InvalidSecurityToken":
		return true
	default:
		return false
	}
}

func isAliyunNotFoundErrorCode(code string) bool {
	switch strings.TrimSpace(code) {
	case "NoSuchBucket", "NoSuchKey", "NoSuchUpload":
		return true
	default:
		return false
	}
}

var _ ObjectStorageProvider = (*AliyunOSSProvider)(nil)
