package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	httpresp "github.com/baihua19941101/cdnManage/internal/http"
	"github.com/baihua19941101/cdnManage/internal/model"
)

const (
	CurrentProjectIDKey = "current_project_id"
	projectIDParamKey   = "id"
)

const requestContextCurrentProjectIDKey requestContextKey = "current_project_id"

type UserProjectRoleRepository interface {
	ListByUserID(ctx context.Context, userID uint64) ([]model.UserProjectRole, error)
}

type UserProjectRoleCache interface {
	Get(ctx context.Context, userID uint64) (map[uint64]string, bool, error)
	Set(ctx context.Context, userID uint64, roles map[uint64]string, ttl time.Duration) error
}

type ProjectScopeResolver struct {
	roles    UserProjectRoleRepository
	cache    UserProjectRoleCache
	cacheTTL time.Duration
}

func NewProjectScopeResolver(roles UserProjectRoleRepository, cache UserProjectRoleCache, cacheTTL time.Duration) *ProjectScopeResolver {
	return &ProjectScopeResolver{
		roles:    roles,
		cache:    cache,
		cacheTTL: cacheTTL,
	}
}

func (r *ProjectScopeResolver) Middleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		projectID, err := currentProjectIDFromRoute(ctx)
		if err != nil {
			ctx.Error(err)
			ctx.Abort()
			return
		}

		SetCurrentProjectID(ctx, projectID)

		platformRole, ok := CurrentPlatformRole(ctx)
		if !ok {
			ctx.Error(httpresp.NewAppError(http.StatusUnauthorized, "authentication_failed", "authenticated user is required", nil))
			ctx.Abort()
			return
		}

		if model.IsPlatformAdminRole(platformRole) {
			SetCurrentProjectRole(ctx, model.ProjectRoleAdmin)
			ctx.Next()
			return
		}

		userID, ok := CurrentUserID(ctx)
		if !ok {
			ctx.Error(httpresp.NewAppError(http.StatusUnauthorized, "authentication_failed", "authenticated user is required", nil))
			ctx.Abort()
			return
		}

		projectRoles, err := r.projectRoles(ctx.Request.Context(), userID)
		if err != nil {
			ctx.Error(err)
			ctx.Abort()
			return
		}

		projectRole, exists := projectRoles[projectID]
		if !exists {
			ctx.Error(httpresp.NewAppError(http.StatusForbidden, "project_scope_denied", "project is outside the authorized scope", gin.H{
				"projectID": projectID,
			}))
			ctx.Abort()
			return
		}

		SetCurrentProjectRole(ctx, projectRole)
		ctx.Next()
	}
}

func SetCurrentProjectID(ctx *gin.Context, projectID uint64) {
	ctx.Set(CurrentProjectIDKey, projectID)
	requestContext := context.WithValue(ctx.Request.Context(), requestContextCurrentProjectIDKey, projectID)
	ctx.Request = ctx.Request.WithContext(requestContext)
}

func CurrentProjectID(ctx *gin.Context) (uint64, bool) {
	value, exists := ctx.Get(CurrentProjectIDKey)
	if !exists {
		return 0, false
	}

	projectID, ok := value.(uint64)
	return projectID, ok
}

func currentProjectIDFromRoute(ctx *gin.Context) (uint64, error) {
	rawID := ctx.Param(projectIDParamKey)
	if rawID == "" {
		return 0, httpresp.NewAppError(http.StatusBadRequest, "validation_error", "project id is required", nil)
	}

	projectID, err := strconv.ParseUint(rawID, 10, 64)
	if err != nil {
		return 0, httpresp.NewAppError(http.StatusBadRequest, "validation_error", "project id must be a positive integer", gin.H{
			"projectID": rawID,
		})
	}

	return projectID, nil
}

func (r *ProjectScopeResolver) projectRoles(ctx context.Context, userID uint64) (map[uint64]string, error) {
	if r.cache != nil {
		cachedRoles, hit, err := r.cache.Get(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("load project role cache: %w", err)
		}
		if hit {
			return cachedRoles, nil
		}
	}

	bindings, err := r.roles.ListByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list user project roles: %w", err)
	}

	projectRoles := make(map[uint64]string, len(bindings))
	for _, binding := range bindings {
		projectRoles[binding.ProjectID] = binding.ProjectRole
	}

	if r.cache != nil {
		if err := r.cache.Set(ctx, userID, projectRoles, r.cacheTTL); err != nil {
			return nil, fmt.Errorf("store project role cache: %w", err)
		}
	}

	return projectRoles, nil
}

type RedisUserProjectRoleCache struct {
	client keyValueCache
	prefix string
}

type keyValueCache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
}

func NewRedisUserProjectRoleCache(client keyValueCache) *RedisUserProjectRoleCache {
	return &RedisUserProjectRoleCache{
		client: client,
		prefix: "rbac:user-project-roles",
	}
}

func (c *RedisUserProjectRoleCache) Get(ctx context.Context, userID uint64) (map[uint64]string, bool, error) {
	raw, err := c.client.Get(ctx, c.cacheKey(userID))
	if err != nil {
		if raw == "" {
			return nil, false, nil
		}
		return nil, false, err
	}
	if raw == "" {
		return nil, false, nil
	}

	var payload map[uint64]string
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, false, err
	}

	return payload, true, nil
}

func (c *RedisUserProjectRoleCache) Set(ctx context.Context, userID uint64, roles map[uint64]string, ttl time.Duration) error {
	payload, err := json.Marshal(roles)
	if err != nil {
		return err
	}

	return c.client.Set(ctx, c.cacheKey(userID), payload, ttl)
}

func (c *RedisUserProjectRoleCache) cacheKey(userID uint64) string {
	return fmt.Sprintf("%s:%d", c.prefix, userID)
}
