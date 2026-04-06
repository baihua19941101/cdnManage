package provider

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	cos "github.com/tencentyun/cos-go-sdk-v5"
)

const (
	tencentCOSOpDetect   = "detect_provider"
	tencentCOSOpList     = "list_objects"
	tencentCOSOpUpload   = "upload_object"
	tencentCOSOpDownload = "download_object"
	tencentCOSOpDelete   = "delete_object"
	tencentCOSOpRename   = "rename_object"
)

// TencentCOSProvider implements ObjectStorageProvider using Tencent Cloud COS SDK.
type TencentCOSProvider struct{}

func NewTencentCOSProvider() *TencentCOSProvider {
	return &TencentCOSProvider{}
}

func (p *TencentCOSProvider) Type() Type {
	return TypeTencentCloud
}

func (p *TencentCOSProvider) Detect(ctx context.Context, credential CredentialPayload, bucket string) (Type, error) {
	client, err := p.newClient(credential, strings.TrimSpace(bucket), strings.TrimSpace(credential.CustomFields["region"]))
	if err != nil {
		return TypeUnknown, err
	}

	checkCtx := ctx
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		checkCtx, cancel = context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
	}

	if _, err := client.Bucket.Head(checkCtx); err != nil {
		return TypeUnknown, p.mapError(tencentCOSOpDetect, err)
	}

	return TypeTencentCloud, nil
}

func (p *TencentCOSProvider) ListObjects(ctx context.Context, req ListObjectsRequest) ([]ObjectInfo, error) {
	client, err := p.newClient(req.Credential, req.Bucket, req.Region)
	if err != nil {
		return nil, err
	}

	opt := &cos.BucketGetOptions{
		Prefix: strings.TrimSpace(req.Prefix),
		Marker: strings.TrimSpace(req.Marker),
	}
	if req.MaxKeys > 0 {
		opt.MaxKeys = req.MaxKeys
	}

	result, _, err := client.Bucket.Get(ctx, opt)
	if err != nil {
		return nil, p.mapError(tencentCOSOpList, err)
	}

	objects := make([]ObjectInfo, 0, len(result.Contents)+len(result.CommonPrefixes))
	for _, item := range result.Contents {
		objects = append(objects, ObjectInfo{
			Key:          item.Key,
			ETag:         strings.Trim(item.ETag, "\""),
			Size:         item.Size,
			LastModified: parseCOSLastModified(item.LastModified),
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

func (p *TencentCOSProvider) UploadObject(ctx context.Context, req UploadObjectRequest) error {
	client, err := p.newClient(req.Credential, req.Bucket, req.Region)
	if err != nil {
		return err
	}

	key := strings.TrimSpace(req.Key)
	if key == "" {
		return NewError(TypeTencentCloud, ServiceObjectStorage, tencentCOSOpUpload, ErrCodeInvalidRequest, "object key is required", false, nil)
	}
	if req.Content == nil {
		return NewError(TypeTencentCloud, ServiceObjectStorage, tencentCOSOpUpload, ErrCodeInvalidRequest, "object content is required", false, nil)
	}

	opt := &cos.ObjectPutOptions{}
	if ct := strings.TrimSpace(req.ContentType); ct != "" {
		opt.ObjectPutHeaderOptions = &cos.ObjectPutHeaderOptions{ContentType: ct}
	}

	if _, err := client.Object.Put(ctx, key, req.Content, opt); err != nil {
		return p.mapError(tencentCOSOpUpload, err)
	}
	return nil
}

func (p *TencentCOSProvider) DownloadObject(ctx context.Context, req DownloadObjectRequest) (io.ReadCloser, ObjectMeta, error) {
	client, err := p.newClient(req.Credential, req.Bucket, req.Region)
	if err != nil {
		return nil, ObjectMeta{}, err
	}

	key := strings.TrimSpace(req.Key)
	if key == "" {
		return nil, ObjectMeta{}, NewError(TypeTencentCloud, ServiceObjectStorage, tencentCOSOpDownload, ErrCodeInvalidRequest, "object key is required", false, nil)
	}

	resp, err := client.Object.Get(ctx, key, nil)
	if err != nil {
		return nil, ObjectMeta{}, p.mapError(tencentCOSOpDownload, err)
	}

	meta := ObjectMeta{
		ContentLength: resp.ContentLength,
		ContentType:   strings.TrimSpace(resp.Header.Get("Content-Type")),
		ETag:          strings.Trim(resp.Header.Get("ETag"), "\""),
	}
	if lastModified := strings.TrimSpace(resp.Header.Get("Last-Modified")); lastModified != "" {
		meta.LastModified = parseCOSLastModified(lastModified)
	}
	return resp.Body, meta, nil
}

func (p *TencentCOSProvider) DeleteObject(ctx context.Context, req DeleteObjectRequest) error {
	client, err := p.newClient(req.Credential, req.Bucket, req.Region)
	if err != nil {
		return err
	}

	key := strings.TrimSpace(req.Key)
	if key == "" {
		return NewError(TypeTencentCloud, ServiceObjectStorage, tencentCOSOpDelete, ErrCodeInvalidRequest, "object key is required", false, nil)
	}

	if _, err := client.Object.Delete(ctx, key); err != nil {
		return p.mapError(tencentCOSOpDelete, err)
	}
	return nil
}

func (p *TencentCOSProvider) RenameObject(ctx context.Context, req RenameObjectRequest) error {
	client, err := p.newClient(req.Credential, req.Bucket, req.Region)
	if err != nil {
		return err
	}

	sourceKey := strings.TrimSpace(req.SourceKey)
	targetKey := strings.TrimSpace(req.TargetKey)
	if sourceKey == "" || targetKey == "" {
		return NewError(TypeTencentCloud, ServiceObjectStorage, tencentCOSOpRename, ErrCodeInvalidRequest, "source key and target key are required", false, nil)
	}
	if sourceKey == targetKey {
		return nil
	}

	sourceURL := fmt.Sprintf("%s.cos.%s.myqcloud.com/%s", strings.TrimSpace(req.Bucket), strings.TrimSpace(req.Region), encodeCOSObjectKey(sourceKey))
	if _, _, err := client.Object.Copy(ctx, targetKey, sourceURL, nil); err != nil {
		return p.mapError(tencentCOSOpRename, err)
	}
	if _, err := client.Object.Delete(ctx, sourceKey); err != nil {
		return p.mapError(tencentCOSOpRename, err)
	}

	return nil
}

func (p *TencentCOSProvider) newClient(credential CredentialPayload, bucket, region string) (*cos.Client, error) {
	secretID := strings.TrimSpace(credential.AccessKeyID)
	secretKey := strings.TrimSpace(credential.AccessKeySecret)
	bucket = strings.TrimSpace(bucket)
	region = strings.TrimSpace(region)
	if region == "" {
		region = strings.TrimSpace(credential.CustomFields["region"])
	}

	if secretID == "" || secretKey == "" {
		return nil, NewError(TypeTencentCloud, ServiceObjectStorage, "build_client", ErrCodeInvalidCredentials, "access key id and access key secret are required", false, nil)
	}
	if bucket == "" || region == "" {
		return nil, NewError(TypeTencentCloud, ServiceObjectStorage, "build_client", ErrCodeInvalidRequest, "bucket and region are required", false, nil)
	}

	bucketURL, err := url.Parse(fmt.Sprintf("https://%s.cos.%s.myqcloud.com", bucket, region))
	if err != nil {
		return nil, NewError(TypeTencentCloud, ServiceObjectStorage, "build_client", ErrCodeInvalidRequest, "invalid bucket or region", false, err)
	}

	baseURL := &cos.BaseURL{BucketURL: bucketURL}
	httpClient := &http.Client{Transport: &cos.AuthorizationTransport{
		SecretID:     secretID,
		SecretKey:    secretKey,
		SessionToken: strings.TrimSpace(credential.SecurityToken),
	}}

	return cos.NewClient(baseURL, httpClient), nil
}

func (p *TencentCOSProvider) mapError(operation string, err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return NewError(TypeTencentCloud, ServiceObjectStorage, operation, ErrCodeTimeout, "request to tencent cos timed out", true, err)
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return NewError(TypeTencentCloud, ServiceObjectStorage, operation, ErrCodeTimeout, "request to tencent cos timed out", true, err)
	}

	var urlErr *url.Error
	if errors.As(err, &urlErr) && urlErr.Timeout() {
		return NewError(TypeTencentCloud, ServiceObjectStorage, operation, ErrCodeTimeout, "request to tencent cos timed out", true, err)
	}

	var cosErr *cos.ErrorResponse
	if errors.As(err, &cosErr) {
		statusCode := 0
		if cosErr.Response != nil {
			statusCode = cosErr.Response.StatusCode
		}
		switch {
		case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden || isCOSCredentialErrorCode(cosErr.Code):
			return NewError(TypeTencentCloud, ServiceObjectStorage, operation, ErrCodeInvalidCredentials, "invalid credentials for tencent cos", false, err)
		case statusCode == http.StatusNotFound || isCOSNotFoundErrorCode(cosErr.Code):
			return NewError(TypeTencentCloud, ServiceObjectStorage, operation, ErrCodeNotFound, "resource not found on tencent cos", false, err)
		case statusCode == http.StatusRequestTimeout || statusCode == http.StatusGatewayTimeout:
			return NewError(TypeTencentCloud, ServiceObjectStorage, operation, ErrCodeTimeout, "request to tencent cos timed out", true, err)
		default:
			return NewError(TypeTencentCloud, ServiceObjectStorage, operation, ErrCodeOperationFailed, "tencent cos operation failed", statusCode >= 500, err)
		}
	}

	return NewError(TypeTencentCloud, ServiceObjectStorage, operation, ErrCodeOperationFailed, "tencent cos operation failed", false, err)
}

func isCOSCredentialErrorCode(code string) bool {
	switch strings.TrimSpace(code) {
	case "SignatureDoesNotMatch", "InvalidAccessKeyId", "AccessDenied", "AuthFailure", "InvalidSecretId", "InvalidSecretKey":
		return true
	default:
		return false
	}
}

func isCOSNotFoundErrorCode(code string) bool {
	switch strings.TrimSpace(code) {
	case "NoSuchBucket", "NoSuchKey", "NoSuchResource", "ResourceNotFound":
		return true
	default:
		return false
	}
}

func parseCOSLastModified(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}

	for _, layout := range []string{time.RFC3339, time.RFC3339Nano, time.RFC1123, time.RFC1123Z, "Mon, 02 Jan 2006 15:04:05 GMT", "2006-01-02T15:04:05.000Z"} {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed.UTC()
		}
	}
	return time.Time{}
}

func encodeCOSObjectKey(key string) string {
	segments := strings.Split(key, "/")
	for i := range segments {
		segments[i] = url.PathEscape(segments[i])
	}
	return strings.Join(segments, "/")
}

var _ ObjectStorageProvider = (*TencentCOSProvider)(nil)
