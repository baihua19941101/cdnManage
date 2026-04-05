package provider

import (
	"context"
	"io"
	"time"
)

type CredentialPayload struct {
	AccessKeyID     string
	AccessKeySecret string
	SecurityToken   string
	CustomFields    map[string]string
}

type ObjectInfo struct {
	Key          string
	ETag         string
	ContentType  string
	Size         int64
	LastModified time.Time
	IsDir        bool
}

type ObjectMeta struct {
	ContentLength int64
	ContentType   string
	ETag          string
	LastModified  time.Time
}

type ListObjectsRequest struct {
	Bucket     string
	Region     string
	Prefix     string
	Marker     string
	MaxKeys    int
	Credential CredentialPayload
}

type UploadObjectRequest struct {
	Bucket      string
	Region      string
	Key         string
	ContentType string
	Content     io.Reader
	Size        int64
	Credential  CredentialPayload
}

type DownloadObjectRequest struct {
	Bucket     string
	Region     string
	Key        string
	Credential CredentialPayload
}

type DeleteObjectRequest struct {
	Bucket     string
	Region     string
	Key        string
	Credential CredentialPayload
}

type RenameObjectRequest struct {
	Bucket     string
	Region     string
	SourceKey  string
	TargetKey  string
	Credential CredentialPayload
}

type RefreshURLsRequest struct {
	Endpoint   string
	URLs       []string
	Credential CredentialPayload
}

type RefreshDirectoriesRequest struct {
	Endpoint    string
	Directories []string
	Credential  CredentialPayload
}

type SyncResourcesRequest struct {
	Endpoint      string
	Bucket        string
	Region        string
	Paths         []string
	Credential    CredentialPayload
	InvalidateCDN bool
}

type TaskResult struct {
	ProviderRequestID string
	TaskID            string
	Status            string
	SubmittedAt       time.Time
	CompletedAt       *time.Time
	Metadata          map[string]string
}

type ObjectStorageProvider interface {
	Type() Type
	Detect(ctx context.Context, credential CredentialPayload, bucket string) (Type, error)
	ListObjects(ctx context.Context, req ListObjectsRequest) ([]ObjectInfo, error)
	UploadObject(ctx context.Context, req UploadObjectRequest) error
	DownloadObject(ctx context.Context, req DownloadObjectRequest) (io.ReadCloser, ObjectMeta, error)
	DeleteObject(ctx context.Context, req DeleteObjectRequest) error
	RenameObject(ctx context.Context, req RenameObjectRequest) error
}

type CDNProvider interface {
	Type() Type
	RefreshURLs(ctx context.Context, req RefreshURLsRequest) (TaskResult, error)
	RefreshDirectories(ctx context.Context, req RefreshDirectoriesRequest) (TaskResult, error)
	SyncLatestResources(ctx context.Context, req SyncResourcesRequest) (TaskResult, error)
}
