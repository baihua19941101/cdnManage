package repository

import (
	"context"

	"gorm.io/gorm"

	"github.com/baihua19941101/cdnManage/internal/model"
)

type gormAuditLogRepository struct {
	db *gorm.DB
}

func (r *gormAuditLogRepository) Create(ctx context.Context, log *model.AuditLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

func (r *gormAuditLogRepository) List(ctx context.Context, filter AuditLogFilter) ([]model.AuditLog, error) {
	query := r.db.WithContext(ctx).
		Model(&model.AuditLog{}).
		Preload("ActorUser").
		Preload("Project")

	if filter.ProjectID != nil {
		query = query.Where("project_id = ?", *filter.ProjectID)
	}
	if filter.ActorUserID != nil {
		query = query.Where("actor_user_id = ?", *filter.ActorUserID)
	}
	if filter.Action != "" {
		query = query.Where("action = ?", filter.Action)
	}
	if filter.TargetType != "" {
		query = query.Where("target_type = ?", filter.TargetType)
	}
	if filter.TargetIdentifier != "" {
		query = query.Where("target_identifier LIKE ?", "%"+filter.TargetIdentifier+"%")
	}
	if filter.SessionID != "" {
		query = query.Where("JSON_UNQUOTE(JSON_EXTRACT(metadata, '$.sessionId')) = ?", filter.SessionID)
	}
	if filter.Result != "" {
		query = query.Where("result = ?", filter.Result)
	}
	if filter.CreatedAfter != nil {
		query = query.Where("created_at >= ?", *filter.CreatedAfter)
	}
	if filter.CreatedBefore != nil {
		query = query.Where("created_at <= ?", *filter.CreatedBefore)
	}
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}

	var logs []model.AuditLog
	if err := query.Order("id desc").Find(&logs).Error; err != nil {
		return nil, err
	}
	return logs, nil
}

func (r *gormAuditLogRepository) ListDistinctActions(ctx context.Context, projectID *uint64) ([]string, error) {
	return r.listDistinctValues(ctx, "action", projectID)
}

func (r *gormAuditLogRepository) ListDistinctTargetTypes(ctx context.Context, projectID *uint64) ([]string, error) {
	return r.listDistinctValues(ctx, "target_type", projectID)
}

func (r *gormAuditLogRepository) listDistinctValues(ctx context.Context, column string, projectID *uint64) ([]string, error) {
	query := r.db.WithContext(ctx).
		Model(&model.AuditLog{}).
		Where(column+" <> ''")
	if projectID != nil {
		query = query.Where("project_id = ?", *projectID)
	}

	var values []string
	if err := query.
		Distinct(column).
		Order(column + " asc").
		Pluck(column, &values).Error; err != nil {
		return nil, err
	}

	return values, nil
}
