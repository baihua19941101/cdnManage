package projects

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type SyncTaskStatus struct {
	TaskID            string            `json:"taskId"`
	ProjectID         uint64            `json:"projectId"`
	BucketName        string            `json:"bucketName"`
	CDNEndpoint       string            `json:"cdnEndpoint"`
	Paths             []string          `json:"paths"`
	Status            string            `json:"status"`
	ProviderRequestID string            `json:"providerRequestId,omitempty"`
	SubmittedAt       time.Time         `json:"submittedAt"`
	CompletedAt       *time.Time        `json:"completedAt,omitempty"`
	Metadata          map[string]string `json:"metadata,omitempty"`
}

type SyncTaskStatusCache interface {
	Set(ctx context.Context, taskID string, status SyncTaskStatus, ttl time.Duration) error
}

type syncTaskKeyValueCache interface {
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
}

type RedisSyncTaskStatusCache struct {
	client syncTaskKeyValueCache
	prefix string
}

func NewRedisSyncTaskStatusCache(client syncTaskKeyValueCache) *RedisSyncTaskStatusCache {
	return &RedisSyncTaskStatusCache{
		client: client,
		prefix: "cdn:sync-task",
	}
}

func (c *RedisSyncTaskStatusCache) Set(ctx context.Context, taskID string, status SyncTaskStatus, ttl time.Duration) error {
	payload, err := json.Marshal(status)
	if err != nil {
		return err
	}

	return c.client.Set(ctx, c.cacheKey(taskID), payload, ttl)
}

func (c *RedisSyncTaskStatusCache) cacheKey(taskID string) string {
	return fmt.Sprintf("%s:%s", c.prefix, taskID)
}
