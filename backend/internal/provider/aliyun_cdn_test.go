package provider

import (
	"context"
	"errors"
	"testing"

	sdkerrors "github.com/aliyun/alibaba-cloud-sdk-go/sdk/errors"
	"github.com/stretchr/testify/require"
)

func TestAliyunCDNProviderType(t *testing.T) {
	p := NewAliyunCDNProvider()
	require.Equal(t, TypeAliyun, p.Type())
}

func TestAliyunCDNBuildClientValidation(t *testing.T) {
	p := NewAliyunCDNProvider()

	_, err := p.newCDNClient(CredentialPayload{})
	require.Error(t, err)
	providerErr, ok := err.(*Error)
	require.True(t, ok)
	require.Equal(t, ErrCodeInvalidCredentials, providerErr.Code)
}

func TestAliyunCDNRefreshValidation(t *testing.T) {
	p := NewAliyunCDNProvider()
	_, err := p.RefreshURLs(context.Background(), RefreshURLsRequest{
		Endpoint: "https://cdn.example.com",
		URLs:     []string{},
	})
	require.Error(t, err)
	providerErr, ok := err.(*Error)
	require.True(t, ok)
	require.Equal(t, ErrCodeInvalidRequest, providerErr.Code)
}

func TestAliyunCDNSyncSkip(t *testing.T) {
	p := NewAliyunCDNProvider()
	result, err := p.SyncLatestResources(context.Background(), SyncResourcesRequest{
		InvalidateCDN: false,
	})
	require.NoError(t, err)
	require.Equal(t, statusSkipped, result.Status)
}

func TestAliyunCDNWrapErrorMappings(t *testing.T) {
	p := NewAliyunCDNProvider()

	t.Run("server error invalid credentials", func(t *testing.T) {
		err := p.wrapError("refresh_urls", sdkerrors.NewServerError(403, `{"Code":"InvalidAccessKeyId"}`, ""))
		providerErr, ok := err.(*Error)
		require.True(t, ok)
		require.Equal(t, ErrCodeInvalidCredentials, providerErr.Code)
	})

	t.Run("server error timeout", func(t *testing.T) {
		err := p.wrapError("refresh_urls", sdkerrors.NewServerError(504, `{"Code":"OperationTimeout"}`, ""))
		providerErr, ok := err.(*Error)
		require.True(t, ok)
		require.Equal(t, ErrCodeTimeout, providerErr.Code)
	})

	t.Run("context deadline", func(t *testing.T) {
		err := p.wrapError("refresh_urls", context.DeadlineExceeded)
		providerErr, ok := err.(*Error)
		require.True(t, ok)
		require.Equal(t, ErrCodeTimeout, providerErr.Code)
	})

	t.Run("fallback", func(t *testing.T) {
		err := p.wrapError("refresh_urls", errors.New("boom"))
		providerErr, ok := err.(*Error)
		require.True(t, ok)
		require.Equal(t, ErrCodeOperationFailed, providerErr.Code)
	})
}
