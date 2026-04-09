package projects

import (
	"testing"

	"github.com/stretchr/testify/require"

	httpresp "github.com/baihua19941101/cdnManage/internal/http"
)

func TestBuildBindingCredentialAuditEntries(t *testing.T) {
	req := updateProjectRequest{
		Buckets: []projectBucketRequest{
			{
				ID:                  101,
				BucketName:          "bucket-main",
				ProviderType:        "tencent",
				CredentialOperation: "",
				AccessKeyID:         "AK",
				AccessKeySecret:     "SK",
			},
		},
		CDNs: []projectCDNRequest{
			{
				ID:                  0,
				CDNEndpoint:         "cdn.example.com",
				ProviderType:        "aliyun",
				CredentialOperation: "replace",
				Credential:          `{"accessKeyId":"AK2","accessKeySecret":"SK2"}`,
			},
		},
	}

	entries := buildBindingCredentialAuditEntries(req)
	require.Len(t, entries, 2)

	bucketEntry := entries[0]
	require.Equal(t, "project.binding.credential.keep", bucketEntry.action)
	require.Equal(t, "bucket", bucketEntry.targetType)
	require.Equal(t, "id:101", bucketEntry.targetIdentifier)
	require.Equal(t, "KEEP", bucketEntry.metadata["credentialOperation"])
	require.Equal(t, "default_existing_binding", bucketEntry.metadata["credentialModeSource"])
	require.NotContains(t, bucketEntry.metadata, "credential")
	require.NotContains(t, bucketEntry.metadata, "credentialCiphertext")
	require.NotContains(t, bucketEntry.metadata, "accessKeyId")
	require.NotContains(t, bucketEntry.metadata, "accessKeySecret")

	cdnEntry := entries[1]
	require.Equal(t, "project.binding.credential.replace", cdnEntry.action)
	require.Equal(t, "cdn", cdnEntry.targetType)
	require.Equal(t, "cdn.example.com", cdnEntry.targetIdentifier)
	require.Equal(t, "REPLACE", cdnEntry.metadata["credentialOperation"])
	require.Equal(t, "request", cdnEntry.metadata["credentialModeSource"])
	require.NotContains(t, cdnEntry.metadata, "credential")
	require.NotContains(t, cdnEntry.metadata, "credentialCiphertext")
}

func TestFailureAuditSummaryFromAppError(t *testing.T) {
	appErr := httpresp.NewAppError(400, "provider_change_requires_credential_replace", "provider type change requires credential replacement", nil)
	summary := failureAuditSummary(appErr)

	require.Equal(t, "provider_change_requires_credential_replace", summary["errorCode"])
	require.Equal(t, "provider type change requires credential replacement", summary["errorMessage"])
	require.Equal(t, appErr.Error(), summary["error"])
}
