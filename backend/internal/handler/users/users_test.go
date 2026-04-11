package users

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	httpresp "github.com/baihua19941101/cdnManage/internal/http"
	"github.com/baihua19941101/cdnManage/internal/middleware"
	"github.com/baihua19941101/cdnManage/internal/model"
	"github.com/baihua19941101/cdnManage/internal/repository"
	serviceusers "github.com/baihua19941101/cdnManage/internal/service/users"
)

func TestResetPasswordWritesSuccessAuditLog(t *testing.T) {
	const (
		actorUserID  uint64 = 9001
		targetUserID uint64 = 42
		requestID           = "req-reset-password-success"
	)

	userRepo := &memoryUserRepository{
		users: map[uint64]*model.User{
			targetUserID: {
				BaseModel:    model.BaseModel{ID: targetUserID},
				Username:     "target-user",
				Email:        "target-user@example.com",
				PasswordHash: "old-hash",
				Status:       model.UserStatusActive,
				PlatformRole: model.PlatformRoleStandard,
			},
		},
	}
	auditRepo := &memoryAuditLogRepository{}
	handler := NewHandler(serviceusers.NewService(userRepo, nil, nil, nil), auditRepo)

	router := newUsersTestRouter()
	router.PUT("/api/v1/users/:id/password", injectCurrentUser(actorUserID), handler.ResetPassword)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/v1/users/42/password", strings.NewReader(`{"newPassword":"new-password-123"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(httpresp.RequestIDHeader, requestID)

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)

	logs, err := auditRepo.List(context.Background(), repository.AuditLogFilter{
		ActorUserID: uint64Pointer(actorUserID),
		Action:      "user.reset_password",
		Result:      model.AuditResultSuccess,
	})
	require.NoError(t, err)
	require.Len(t, logs, 1)
	require.Equal(t, "user", logs[0].TargetType)
	require.Equal(t, "42", logs[0].TargetIdentifier)
	require.Equal(t, requestID, logs[0].RequestID)
}

func TestResetPasswordWritesFailureAuditLogWhenUserNotFound(t *testing.T) {
	const (
		actorUserID  uint64 = 9002
		targetUserID uint64 = 404
		requestID           = "req-reset-password-failure"
	)

	userRepo := &memoryUserRepository{users: map[uint64]*model.User{}}
	auditRepo := &memoryAuditLogRepository{}
	handler := NewHandler(serviceusers.NewService(userRepo, nil, nil, nil), auditRepo)

	router := newUsersTestRouter()
	router.PUT("/api/v1/users/:id/password", injectCurrentUser(actorUserID), handler.ResetPassword)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/v1/users/404/password", strings.NewReader(`{"newPassword":"new-password-123"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(httpresp.RequestIDHeader, requestID)

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNotFound, recorder.Code)

	logs, err := auditRepo.List(context.Background(), repository.AuditLogFilter{
		ActorUserID: uint64Pointer(actorUserID),
		Action:      "user.reset_password",
		Result:      model.AuditResultFailure,
	})
	require.NoError(t, err)
	require.Len(t, logs, 1)
	require.Equal(t, "user", logs[0].TargetType)
	require.Equal(t, "404", logs[0].TargetIdentifier)
	require.Equal(t, requestID, logs[0].RequestID)

	var metadata map[string]any
	require.NoError(t, json.Unmarshal(logs[0].Metadata, &metadata))
	require.Equal(t, "user not found", metadata["error"])
}

func TestGetProjectBindingsReturnsBindings(t *testing.T) {
	userRepo := &memoryUserRepository{
		users: map[uint64]*model.User{
			42: {
				BaseModel:    model.BaseModel{ID: 42},
				Username:     "bound-user",
				Email:        "bound-user@example.com",
				PasswordHash: "hash",
				Status:       model.UserStatusActive,
				PlatformRole: model.PlatformRoleStandard,
			},
		},
	}
	roleRepo := &memoryUserProjectRoleRepository{
		rolesByUserID: map[uint64][]model.UserProjectRole{
			42: {
				{UserID: 42, ProjectID: 101, ProjectRole: model.ProjectRoleAdmin},
				{UserID: 42, ProjectID: 102, ProjectRole: model.ProjectRoleReadOnly},
			},
		},
	}
	handler := NewHandler(serviceusers.NewService(userRepo, roleRepo, nil, nil), nil)

	router := newUsersTestRouter()
	router.GET("/api/v1/users/:id/project-bindings", handler.GetProjectBindings)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/users/42/project-bindings", nil)
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload struct {
		Code string `json:"code"`
		Data struct {
			Bindings []projectRoleResponse `json:"bindings"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Equal(t, "success", payload.Code)
	require.Len(t, payload.Data.Bindings, 2)
	require.Equal(t, projectRoleResponse{ProjectID: 101, ProjectRole: model.ProjectRoleAdmin}, payload.Data.Bindings[0])
	require.Equal(t, projectRoleResponse{ProjectID: 102, ProjectRole: model.ProjectRoleReadOnly}, payload.Data.Bindings[1])
}

func TestGetProjectBindingsReturnsUserNotFound(t *testing.T) {
	userRepo := &memoryUserRepository{users: map[uint64]*model.User{}}
	roleRepo := &memoryUserProjectRoleRepository{rolesByUserID: map[uint64][]model.UserProjectRole{}}
	handler := NewHandler(serviceusers.NewService(userRepo, roleRepo, nil, nil), nil)

	router := newUsersTestRouter()
	router.GET("/api/v1/users/:id/project-bindings", handler.GetProjectBindings)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/users/404/project-bindings", nil)
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNotFound, recorder.Code)
	var payload struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Equal(t, "user_not_found", payload.Code)
	require.Equal(t, "user not found", payload.Message)
}

func newUsersTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.RequestID(), middleware.ErrorHandler())
	return router
}

func injectCurrentUser(userID uint64) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Set(middleware.CurrentUserIDKey, userID)
		ctx.Set(middleware.CurrentPlatformRoleKey, model.PlatformRoleAdmin)
		ctx.Next()
	}
}

type memoryAuditLogRepository struct {
	logs []model.AuditLog
}

func (r *memoryAuditLogRepository) Create(_ context.Context, log *model.AuditLog) error {
	copied := *log
	r.logs = append(r.logs, copied)
	return nil
}

func (r *memoryAuditLogRepository) List(_ context.Context, filter repository.AuditLogFilter) ([]model.AuditLog, error) {
	result := make([]model.AuditLog, 0, len(r.logs))
	for _, log := range r.logs {
		if filter.ActorUserID != nil && log.ActorUserID != *filter.ActorUserID {
			continue
		}
		if filter.Action != "" && log.Action != filter.Action {
			continue
		}
		if filter.Result != "" && log.Result != filter.Result {
			continue
		}
		result = append(result, log)
	}
	return result, nil
}

func (r *memoryAuditLogRepository) ListDistinctActions(_ context.Context, _ *uint64) ([]string, error) {
	return nil, nil
}

func (r *memoryAuditLogRepository) ListDistinctTargetTypes(_ context.Context, _ *uint64) ([]string, error) {
	return nil, nil
}

type memoryUserRepository struct {
	users map[uint64]*model.User
}

type memoryUserProjectRoleRepository struct {
	rolesByUserID map[uint64][]model.UserProjectRole
}

func (r *memoryUserProjectRoleRepository) Create(_ context.Context, _ *model.UserProjectRole) error {
	return nil
}

func (r *memoryUserProjectRoleRepository) DeleteByUserID(_ context.Context, userID uint64) error {
	if r.rolesByUserID == nil {
		return nil
	}
	delete(r.rolesByUserID, userID)
	return nil
}

func (r *memoryUserProjectRoleRepository) DeleteByProjectID(_ context.Context, projectID uint64) error {
	for userID, roles := range r.rolesByUserID {
		filtered := make([]model.UserProjectRole, 0, len(roles))
		for _, role := range roles {
			if role.ProjectID != projectID {
				filtered = append(filtered, role)
			}
		}
		r.rolesByUserID[userID] = filtered
	}
	return nil
}

func (r *memoryUserProjectRoleRepository) ListByUserID(_ context.Context, userID uint64) ([]model.UserProjectRole, error) {
	roles := r.rolesByUserID[userID]
	result := make([]model.UserProjectRole, len(roles))
	copy(result, roles)
	return result, nil
}

func (r *memoryUserProjectRoleRepository) ListByProjectID(_ context.Context, projectID uint64) ([]model.UserProjectRole, error) {
	result := make([]model.UserProjectRole, 0)
	for _, roles := range r.rolesByUserID {
		for _, role := range roles {
			if role.ProjectID == projectID {
				result = append(result, role)
			}
		}
	}
	return result, nil
}

func (r *memoryUserRepository) Create(_ context.Context, user *model.User) error {
	if r.users == nil {
		r.users = make(map[uint64]*model.User)
	}
	copied := *user
	r.users[user.ID] = &copied
	return nil
}

func (r *memoryUserRepository) Update(_ context.Context, user *model.User) error {
	if r.users == nil {
		return errors.New("user not found")
	}
	if _, exists := r.users[user.ID]; !exists {
		return errors.New("user not found")
	}
	copied := *user
	r.users[user.ID] = &copied
	return nil
}

func (r *memoryUserRepository) Delete(_ context.Context, id uint64) error {
	if r.users == nil {
		return nil
	}
	delete(r.users, id)
	return nil
}

func (r *memoryUserRepository) Count(_ context.Context, _ repository.UserFilter) (int64, error) {
	return int64(len(r.users)), nil
}

func (r *memoryUserRepository) GetByID(_ context.Context, id uint64) (*model.User, error) {
	if r.users == nil {
		return nil, errors.New("user not found")
	}
	user, exists := r.users[id]
	if !exists {
		return nil, errors.New("user not found")
	}
	copied := *user
	return &copied, nil
}

func (r *memoryUserRepository) GetByEmail(_ context.Context, email string) (*model.User, error) {
	for _, user := range r.users {
		if user.Email == email {
			copied := *user
			return &copied, nil
		}
	}
	return nil, errors.New("user not found")
}

func (r *memoryUserRepository) GetByUsername(_ context.Context, username string) (*model.User, error) {
	for _, user := range r.users {
		if user.Username == username {
			copied := *user
			return &copied, nil
		}
	}
	return nil, errors.New("user not found")
}

func (r *memoryUserRepository) List(_ context.Context, _ repository.UserFilter) ([]model.User, error) {
	result := make([]model.User, 0, len(r.users))
	for _, user := range r.users {
		result = append(result, *user)
	}
	return result, nil
}

func uint64Pointer(value uint64) *uint64 {
	return &value
}
