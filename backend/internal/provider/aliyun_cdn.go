package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	sdkerrors "github.com/aliyun/alibaba-cloud-sdk-go/sdk/errors"
	aliyuncdn "github.com/aliyun/alibaba-cloud-sdk-go/services/cdn"
)

const (
	aliyunCDNDefaultRegion = "cn-hangzhou"
)

var _ CDNProvider = (*AliyunCDNProvider)(nil)

type AliyunCDNProvider struct{}

func NewAliyunCDNProvider() *AliyunCDNProvider {
	return &AliyunCDNProvider{}
}

func (p *AliyunCDNProvider) Type() Type {
	return TypeAliyun
}

func (p *AliyunCDNProvider) RefreshURLs(ctx context.Context, req RefreshURLsRequest) (TaskResult, error) {
	normalizedURLs, err := buildAbsoluteURLs(req.Endpoint, req.URLs, false)
	if err != nil {
		return TaskResult{}, p.wrapError("refresh_urls", err)
	}

	client, err := p.newCDNClient(req.Credential)
	if err != nil {
		return TaskResult{}, p.wrapError("refresh_urls", err)
	}

	sdkReq := aliyuncdn.CreateRefreshObjectCachesRequest()
	sdkReq.ObjectType = "File"
	sdkReq.ObjectPath = strings.Join(normalizedURLs, "\n")
	if token := strings.TrimSpace(req.Credential.SecurityToken); token != "" {
		sdkReq.SecurityToken = token
	}

	resp, err := client.RefreshObjectCaches(sdkReq)
	if err != nil {
		return TaskResult{}, p.wrapError("refresh_urls", err)
	}
	if resp == nil {
		return TaskResult{}, p.wrapError("refresh_urls", errors.New("empty aliyun cdn response"))
	}

	return TaskResult{
		ProviderRequestID: strings.TrimSpace(resp.RequestId),
		TaskID:            strings.TrimSpace(resp.RefreshTaskId),
		Status:            statusSubmitted,
		SubmittedAt:       time.Now().UTC(),
		Metadata: map[string]string{
			"operation": "refresh_urls",
			"count":     fmt.Sprintf("%d", len(normalizedURLs)),
		},
	}, nil
}

func (p *AliyunCDNProvider) RefreshDirectories(ctx context.Context, req RefreshDirectoriesRequest) (TaskResult, error) {
	normalizedDirectories, err := buildAbsoluteURLs(req.Endpoint, req.Directories, true)
	if err != nil {
		return TaskResult{}, p.wrapError("refresh_directories", err)
	}

	client, err := p.newCDNClient(req.Credential)
	if err != nil {
		return TaskResult{}, p.wrapError("refresh_directories", err)
	}

	sdkReq := aliyuncdn.CreateRefreshObjectCachesRequest()
	sdkReq.ObjectType = "Directory"
	sdkReq.ObjectPath = strings.Join(normalizedDirectories, "\n")
	if token := strings.TrimSpace(req.Credential.SecurityToken); token != "" {
		sdkReq.SecurityToken = token
	}

	resp, err := client.RefreshObjectCaches(sdkReq)
	if err != nil {
		return TaskResult{}, p.wrapError("refresh_directories", err)
	}
	if resp == nil {
		return TaskResult{}, p.wrapError("refresh_directories", errors.New("empty aliyun cdn response"))
	}

	return TaskResult{
		ProviderRequestID: strings.TrimSpace(resp.RequestId),
		TaskID:            strings.TrimSpace(resp.RefreshTaskId),
		Status:            statusSubmitted,
		SubmittedAt:       time.Now().UTC(),
		Metadata: map[string]string{
			"operation": "refresh_directories",
			"count":     fmt.Sprintf("%d", len(normalizedDirectories)),
		},
	}, nil
}

func (p *AliyunCDNProvider) SyncLatestResources(ctx context.Context, req SyncResourcesRequest) (TaskResult, error) {
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

func (p *AliyunCDNProvider) newCDNClient(cred CredentialPayload) (*aliyuncdn.Client, error) {
	accessKeyID := strings.TrimSpace(cred.AccessKeyID)
	accessKeySecret := strings.TrimSpace(cred.AccessKeySecret)
	if accessKeyID == "" || accessKeySecret == "" {
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

	region := strings.TrimSpace(customField(cred.CustomFields, "aliyun_region", "cdn_region", "region"))
	if region == "" {
		region = aliyunCDNDefaultRegion
	}

	var (
		client *aliyuncdn.Client
		err    error
	)
	if token := strings.TrimSpace(cred.SecurityToken); token != "" {
		client, err = aliyuncdn.NewClientWithStsToken(region, accessKeyID, accessKeySecret, token)
	} else {
		client, err = aliyuncdn.NewClientWithAccessKey(region, accessKeyID, accessKeySecret)
	}
	if err != nil {
		return nil, err
	}

	return client, nil
}

func (p *AliyunCDNProvider) wrapError(operation string, err error) error {
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
	if isURLNormalizationError(err) {
		return NewError(p.Type(), ServiceCDN, operation, ErrCodeInvalidRequest, err.Error(), false, err)
	}

	var sdkErr sdkerrors.Error
	if errors.As(err, &sdkErr) {
		codeText := strings.TrimSpace(sdkErr.ErrorCode())
		mappedCode := mapAliyunCDNErrorCode(codeText)
		retryable := isAliyunCDNRetryableCode(codeText)
		msg := strings.TrimSpace(sdkErr.Message())
		if msg == "" {
			msg = "aliyun cdn sdk error"
		}
		if serverErr, ok := sdkErr.(*sdkerrors.ServerError); ok {
			if requestID := strings.TrimSpace(serverErr.RequestId()); requestID != "" {
				msg = fmt.Sprintf("%s (request_id=%s)", msg, requestID)
			}
		}
		return NewError(p.Type(), ServiceCDN, operation, mappedCode, msg, retryable, err)
	}

	return NewError(p.Type(), ServiceCDN, operation, ErrCodeOperationFailed, err.Error(), false, err)
}

func mapAliyunCDNErrorCode(code string) ErrorCode {
	lower := strings.ToLower(strings.TrimSpace(code))

	switch {
	case strings.Contains(lower, "signature"),
		strings.Contains(lower, "invalidaccesskey"),
		strings.Contains(lower, "accessdenied"),
		strings.Contains(lower, "forbidden"),
		strings.Contains(lower, "securitytoken"):
		return ErrCodeInvalidCredentials
	case strings.Contains(lower, "throttl"),
		strings.Contains(lower, "ratelimit"),
		strings.Contains(lower, "quotaexceed"),
		strings.Contains(lower, "toomany"):
		return ErrCodeRateLimited
	case strings.Contains(lower, "notfound"),
		strings.Contains(lower, "nosuch"):
		return ErrCodeNotFound
	case strings.Contains(lower, "invalid"),
		strings.Contains(lower, "missing"),
		strings.Contains(lower, "parameter"):
		return ErrCodeInvalidRequest
	case strings.Contains(lower, "timeout"):
		return ErrCodeTimeout
	case strings.Contains(lower, "internal"),
		strings.Contains(lower, "unavailable"):
		return ErrCodeConnectionFailed
	default:
		return ErrCodeOperationFailed
	}
}

func isAliyunCDNRetryableCode(code string) bool {
	lower := strings.ToLower(strings.TrimSpace(code))
	return strings.Contains(lower, "throttl") ||
		strings.Contains(lower, "ratelimit") ||
		strings.Contains(lower, "quotaexceed") ||
		strings.Contains(lower, "toomany") ||
		strings.Contains(lower, "timeout") ||
		strings.Contains(lower, "internal") ||
		strings.Contains(lower, "unavailable")
}

func isURLNormalizationError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(msg, "no refresh targets") ||
		strings.Contains(msg, "invalid endpoint") ||
		strings.Contains(msg, "must be an absolute http") ||
		strings.Contains(msg, "directory path")
}
