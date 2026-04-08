package provider

import (
	"context"
	"testing"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/stretchr/testify/require"
)

func TestAliyunOSSProviderType(t *testing.T) {
	p := NewAliyunOSSProvider()
	require.Equal(t, TypeAliyun, p.Type())
}

func TestResolveAliyunOSSEndpoint(t *testing.T) {
	t.Run("uses_custom_endpoint", func(t *testing.T) {
		endpoint := resolveAliyunOSSEndpoint(map[string]string{
			"oss_endpoint": "oss-cn-shanghai.aliyuncs.com",
		}, "")
		require.Equal(t, "https://oss-cn-shanghai.aliyuncs.com", endpoint)
	})

	t.Run("uses_region_fallback", func(t *testing.T) {
		endpoint := resolveAliyunOSSEndpoint(nil, "cn-hangzhou")
		require.Equal(t, "https://oss-cn-hangzhou.aliyuncs.com", endpoint)
	})
}

func TestAliyunOSSProviderBuildClientValidation(t *testing.T) {
	p := NewAliyunOSSProvider()

	_, err := p.newClient(CredentialPayload{}, "cn-hangzhou")
	require.Error(t, err)
	providerErr, ok := err.(*Error)
	require.True(t, ok)
	require.Equal(t, ErrCodeInvalidCredentials, providerErr.Code)

	_, err = p.newClient(CredentialPayload{
		AccessKeyID:     "LTAI_TEST",
		AccessKeySecret: "secret",
	}, "")
	require.Error(t, err)
	providerErr, ok = err.(*Error)
	require.True(t, ok)
	require.Equal(t, ErrCodeInvalidRequest, providerErr.Code)
}

func TestAliyunOSSProviderValidationWithoutNetwork(t *testing.T) {
	p := NewAliyunOSSProvider()
	cred := CredentialPayload{
		AccessKeyID:     "LTAI_TEST",
		AccessKeySecret: "secret",
		CustomFields: map[string]string{
			"oss_endpoint": "oss-cn-hangzhou.aliyuncs.com",
		},
	}

	err := p.DeleteObject(context.Background(), DeleteObjectRequest{
		Bucket:     "demo-bucket",
		Key:        "",
		Credential: cred,
	})
	require.Error(t, err)
	providerErr, ok := err.(*Error)
	require.True(t, ok)
	require.Equal(t, ErrCodeInvalidRequest, providerErr.Code)

	err = p.RenameObject(context.Background(), RenameObjectRequest{
		Bucket:     "demo-bucket",
		SourceKey:  "",
		TargetKey:  "",
		Credential: cred,
	})
	require.Error(t, err)
	providerErr, ok = err.(*Error)
	require.True(t, ok)
	require.Equal(t, ErrCodeInvalidRequest, providerErr.Code)
}

func TestAliyunOSSProviderMapServiceError(t *testing.T) {
	p := NewAliyunOSSProvider()

	err := p.mapError(aliyunOSSOpList, oss.ServiceError{
		StatusCode: 403,
		Code:       "InvalidAccessKeyId",
	})
	providerErr, ok := err.(*Error)
	require.True(t, ok)
	require.Equal(t, ErrCodeInvalidCredentials, providerErr.Code)

	err = p.mapError(aliyunOSSOpList, oss.ServiceError{
		StatusCode: 404,
		Code:       "NoSuchBucket",
	})
	providerErr, ok = err.(*Error)
	require.True(t, ok)
	require.Equal(t, ErrCodeNotFound, providerErr.Code)
}
