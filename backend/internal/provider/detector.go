package provider

import (
	"fmt"
	"strings"
)

// DetectObjectStorageProvider uses lightweight heuristics to identify the
// provider type from access key and bucket metadata before real SDK adapters are wired.
func DetectObjectStorageProvider(credential CredentialPayload, bucket string) (Type, error) {
	accessKeyID := strings.TrimSpace(credential.AccessKeyID)
	bucket = strings.ToLower(strings.TrimSpace(bucket))

	// Explicit hint has the highest priority.
	if hint := strings.TrimSpace(credential.CustomFields["providerType"]); hint != "" {
		t := Type(hint)
		if IsKnownType(t) && t != TypeUnknown {
			return t, nil
		}
		return TypeUnknown, NewError(TypeUnknown, ServiceObjectStorage, "detect_provider", ErrCodeUnsupportedProvider, "provider hint is not supported", false, nil)
	}

	// AccessKey-based quick patterns.
	switch {
	case strings.HasPrefix(accessKeyID, "LTAI"):
		return TypeAliyun, nil
	case strings.HasPrefix(accessKeyID, "AKID"):
		return TypeTencentCloud, nil
	case strings.HasPrefix(accessKeyID, "HUAWEI_"), strings.HasPrefix(accessKeyID, "HW_"):
		return TypeHuaweiCloud, nil
	case strings.HasPrefix(accessKeyID, "QINIU_"), strings.HasPrefix(accessKeyID, "QAK_"):
		return TypeQiniu, nil
	}

	// Bucket naming patterns as fallback.
	switch {
	case strings.Contains(bucket, "aliyuncs"), strings.HasSuffix(bucket, "-oss"):
		return TypeAliyun, nil
	case strings.Contains(bucket, "myqcloud"), strings.HasSuffix(bucket, "-cos"):
		return TypeTencentCloud, nil
	case strings.Contains(bucket, ".obs."), strings.HasSuffix(bucket, "-obs"):
		return TypeHuaweiCloud, nil
	case strings.Contains(bucket, "qiniucs"), strings.HasSuffix(bucket, "-kodo"):
		return TypeQiniu, nil
	}

	if accessKeyID == "" {
		return TypeUnknown, NewError(TypeUnknown, ServiceObjectStorage, "detect_provider", ErrCodeInvalidCredentials, "access key is required for provider detection", false, nil)
	}

	return TypeUnknown, NewError(TypeUnknown, ServiceObjectStorage, "detect_provider", ErrCodeConnectionFailed, fmt.Sprintf("unable to detect provider for bucket %q", bucket), false, nil)
}
