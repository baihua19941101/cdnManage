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
	handler := NewHandler(serviceusers.NewService(userRepo, nil, nil), auditRepo)

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
	handler := NewHandler(serviceusers.NewService(userRepo, nil, nil), auditRepo)

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

type memoryUserRepository struct {
	users map[uint64]*model.User
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
