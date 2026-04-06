package provider

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"
	"time"

	cdn "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/cdn/v20180606"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	tcerr "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
)

const (
	tencentCDNDefaultRegion = "ap-guangzhou"
	statusSubmitted         = "submitted"
	statusSkipped           = "skipped"
)

var _ CDNProvider = (*TencentCDNProvider)(nil)

type TencentCDNProvider struct{}

func NewTencentCDNProvider() *TencentCDNProvider {
	return &TencentCDNProvider{}
}

func (p *TencentCDNProvider) Type() Type {
	return TypeTencentCloud
}

func (p *TencentCDNProvider) RefreshURLs(ctx context.Context, req RefreshURLsRequest) (TaskResult, error) {
	normalizedURLs, err := buildAbsoluteURLs(req.Endpoint, req.URLs, false)
	if err != nil {
		return TaskResult{}, p.wrapError("refresh_urls", err)
	}

	client, err := p.newCDNClient(req.Credential)
	if err != nil {
		return TaskResult{}, p.wrapError("refresh_urls", err)
	}

	sdkReq := cdn.NewPurgeUrlsCacheRequest()
	sdkReq.Urls = toStringPointers(normalizedURLs)

	resp, err := client.PurgeUrlsCacheWithContext(ctx, sdkReq)
	if err != nil {
		return TaskResult{}, p.wrapError("refresh_urls", err)
	}

	if resp == nil || resp.Response == nil {
		return TaskResult{}, p.wrapError("refresh_urls", errors.New("empty tencent cdn response"))
	}

	return TaskResult{
		ProviderRequestID: deref(resp.Response.RequestId),
		TaskID:            deref(resp.Response.TaskId),
		Status:            statusSubmitted,
		SubmittedAt:       time.Now().UTC(),
		Metadata: map[string]string{
			"operation": "refresh_urls",
			"count":     fmt.Sprintf("%d", len(normalizedURLs)),
		},
	}, nil
}

func (p *TencentCDNProvider) RefreshDirectories(ctx context.Context, req RefreshDirectoriesRequest) (TaskResult, error) {
	normalizedDirectories, err := buildAbsoluteURLs(req.Endpoint, req.Directories, true)
	if err != nil {
		return TaskResult{}, p.wrapError("refresh_directories", err)
	}

	client, err := p.newCDNClient(req.Credential)
	if err != nil {
		return TaskResult{}, p.wrapError("refresh_directories", err)
	}

	sdkReq := cdn.NewPurgePathCacheRequest()
	sdkReq.Paths = toStringPointers(normalizedDirectories)
	flushType := "flush"
	sdkReq.FlushType = &flushType

	resp, err := client.PurgePathCacheWithContext(ctx, sdkReq)
	if err != nil {
		return TaskResult{}, p.wrapError("refresh_directories", err)
	}

	if resp == nil || resp.Response == nil {
		return TaskResult{}, p.wrapError("refresh_directories", errors.New("empty tencent cdn response"))
	}

	return TaskResult{
		ProviderRequestID: deref(resp.Response.RequestId),
		TaskID:            deref(resp.Response.TaskId),
		Status:            statusSubmitted,
		SubmittedAt:       time.Now().UTC(),
		Metadata: map[string]string{
			"operation": "refresh_directories",
			"count":     fmt.Sprintf("%d", len(normalizedDirectories)),
		},
	}, nil
}

func (p *TencentCDNProvider) SyncLatestResources(ctx context.Context, req SyncResourcesRequest) (TaskResult, error) {
	if !req.InvalidateCDN {
		return TaskResult{
			Status:      statusSkipped,
			SubmittedAt: time.Now().UTC(),
			Metadata: map[string]string{
				"operation": "sync_latest_resources",
				"reason":    "cdn invalidation disabled",
			},
		}, nil
	}

	// Tencent CDN 同步策略：将 endpoint + paths 组合为完整 URL 后执行 URL 刷新。
	urls, err := buildAbsoluteURLs(req.Endpoint, req.Paths, false)
	if err != nil {
		return TaskResult{}, p.wrapError("sync_latest_resources", err)
	}

	return p.RefreshURLs(ctx, RefreshURLsRequest{
		Endpoint:   req.Endpoint,
		URLs:       urls,
		Credential: req.Credential,
	})
}

func (p *TencentCDNProvider) newCDNClient(cred CredentialPayload) (*cdn.Client, error) {
	if strings.TrimSpace(cred.AccessKeyID) == "" || strings.TrimSpace(cred.AccessKeySecret) == "" {
		return nil, NewError(
			p.Type(),
			ServiceCDN,
			"init_client",
			ErrCodeInvalidCredentials,
			"access key id and access key secret are required",
			false,
			nil,
		)
	}

	var sdkCred common.CredentialIface
	if strings.TrimSpace(cred.SecurityToken) != "" {
		sdkCred = common.NewTokenCredential(cred.AccessKeyID, cred.AccessKeySecret, cred.SecurityToken)
	} else {
		sdkCred = common.NewCredential(cred.AccessKeyID, cred.AccessKeySecret)
	}

	cp := profile.NewClientProfile()
	if customEndpoint := strings.TrimSpace(customField(cred.CustomFields, "tencent_api_endpoint", "api_endpoint", "endpoint")); customEndpoint != "" {
		cp.HttpProfile.Endpoint = customEndpoint
	}

	region := strings.TrimSpace(customField(cred.CustomFields, "tencent_region", "cdn_region", "region"))
	if region == "" {
		region = tencentCDNDefaultRegion
	}

	client, err := cdn.NewClient(sdkCred, region, cp)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (p *TencentCDNProvider) wrapError(operation string, err error) error {
	if err == nil {
		return nil
	}

	if perr, ok := err.(*Error); ok {
		return perr
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return NewError(p.Type(), ServiceCDN, operation, ErrCodeTimeout, "cdn request timeout", true, err)
	}
	if errors.Is(err, context.Canceled) {
		return NewError(p.Type(), ServiceCDN, operation, ErrCodeOperationFailed, "cdn request canceled", false, err)
	}

	var sdkErr *tcerr.TencentCloudSDKError
	if errors.As(err, &sdkErr) {
		code := mapTencentCDNErrorCode(sdkErr.GetCode())
		retryable := isTencentCDNRetryableCode(sdkErr.GetCode())
		msg := strings.TrimSpace(sdkErr.GetMessage())
		if msg == "" {
			msg = "tencent cdn sdk error"
		}
		if requestID := strings.TrimSpace(sdkErr.GetRequestId()); requestID != "" {
			msg = fmt.Sprintf("%s (request_id=%s)", msg, requestID)
		}
		return NewError(p.Type(), ServiceCDN, operation, code, msg, retryable, err)
	}

	return NewError(p.Type(), ServiceCDN, operation, ErrCodeOperationFailed, err.Error(), false, err)
}

func mapTencentCDNErrorCode(code string) ErrorCode {
	c := strings.TrimSpace(code)
	lower := strings.ToLower(c)

	switch {
	case strings.Contains(lower, "authfailure"),
		strings.Contains(lower, "unauthorizedoperation"),
		strings.Contains(lower, "invalidcredential"),
		strings.Contains(lower, "signature"):
		return ErrCodeInvalidCredentials
	case strings.Contains(lower, "requestlimitexceeded"),
		strings.Contains(lower, "limitexceeded"),
		strings.Contains(lower, "toomanyrequests"):
		return ErrCodeRateLimited
	case strings.Contains(lower, "resourcenotfound"):
		return ErrCodeNotFound
	case strings.Contains(lower, "invalidparameter"),
		strings.Contains(lower, "missingparameter"),
		strings.Contains(lower, "param"):
		return ErrCodeInvalidRequest
	case strings.Contains(lower, "timeout"):
		return ErrCodeTimeout
	case strings.Contains(lower, "internalerror"):
		return ErrCodeConnectionFailed
	default:
		return ErrCodeOperationFailed
	}
}

func isTencentCDNRetryableCode(code string) bool {
	lower := strings.ToLower(strings.TrimSpace(code))
	return strings.Contains(lower, "requestlimitexceeded") ||
		strings.Contains(lower, "limitexceeded") ||
		strings.Contains(lower, "toomanyrequests") ||
		strings.Contains(lower, "internalerror") ||
		strings.Contains(lower, "timeout")
}

func buildAbsoluteURLs(endpoint string, values []string, asDirectory bool) ([]string, error) {
	if len(values) == 0 {
		return nil, errors.New("no refresh targets provided")
	}

	baseURL, hasBase, err := parseOptionalBaseURL(endpoint)
	if err != nil {
		return nil, err
	}

	result := make([]string, 0, len(values))
	for _, raw := range values {
		item := strings.TrimSpace(raw)
		if item == "" {
			continue
		}

		var absolute string
		if isAbsoluteHTTPURL(item) {
			absolute = item
		} else {
			if !hasBase {
				return nil, fmt.Errorf("relative path %q requires endpoint", item)
			}
			absolute = joinBaseAndPath(baseURL, item)
		}

		if asDirectory && !strings.HasSuffix(absolute, "/") {
			absolute += "/"
		}
		result = append(result, absolute)
	}

	if len(result) == 0 {
		return nil, errors.New("no valid refresh targets provided")
	}
	return result, nil
}

func parseOptionalBaseURL(endpoint string) (*url.URL, bool, error) {
	e := strings.TrimSpace(endpoint)
	if e == "" {
		return nil, false, nil
	}

	if !strings.Contains(e, "://") {
		e = "https://" + e
	}

	u, err := url.Parse(e)
	if err != nil {
		return nil, false, fmt.Errorf("invalid endpoint %q: %w", endpoint, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, false, fmt.Errorf("endpoint %q must use http or https", endpoint)
	}
	if strings.TrimSpace(u.Host) == "" {
		return nil, false, fmt.Errorf("endpoint %q must include host", endpoint)
	}
	if u.Path == "" {
		u.Path = "/"
	}
	return u, true, nil
}

func isAbsoluteHTTPURL(s string) bool {
	lower := strings.ToLower(strings.TrimSpace(s))
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}

func joinBaseAndPath(base *url.URL, p string) string {
	cleanPath := strings.TrimSpace(p)
	if cleanPath == "" {
		return base.String()
	}

	baseCopy := *base
	if strings.HasPrefix(cleanPath, "/") {
		baseCopy.Path = path.Clean(cleanPath)
	} else {
		baseCopy.Path = path.Join(strings.TrimSuffix(baseCopy.Path, "/"), cleanPath)
	}
	if !strings.HasPrefix(baseCopy.Path, "/") {
		baseCopy.Path = "/" + baseCopy.Path
	}
	return baseCopy.String()
}

func toStringPointers(values []string) []*string {
	out := make([]*string, 0, len(values))
	for i := range values {
		v := values[i]
		out = append(out, &v)
	}
	return out
}

func customField(fields map[string]string, keys ...string) string {
	if len(fields) == 0 {
		return ""
	}
	for _, k := range keys {
		if v, ok := fields[k]; ok {
			return v
		}
	}
	return ""
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
