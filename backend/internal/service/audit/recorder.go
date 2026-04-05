package audit

import (
	"context"
	"encoding/json"

	"gorm.io/datatypes"

	"github.com/baihua19941101/cdnManage/internal/model"
	"github.com/baihua19941101/cdnManage/internal/repository"
)

type Recorder struct {
	repo repository.AuditLogRepository
}

type RecordInput struct {
	ActorUserID      uint64
	ProjectID        *uint64
	Action           string
	TargetType       string
	TargetIdentifier string
	Result           string
	RequestID        string
	Metadata         map[string]interface{}
}

func NewRecorder(repo repository.AuditLogRepository) *Recorder {
	return &Recorder{repo: repo}
}

func (r *Recorder) Record(ctx context.Context, input RecordInput) error {
	if r == nil || r.repo == nil || input.ActorUserID == 0 {
		return nil
	}

	var metadata datatypes.JSON
	if len(input.Metadata) > 0 {
		raw, err := json.Marshal(input.Metadata)
		if err != nil {
			return err
		}
		metadata = datatypes.JSON(raw)
	}

	return r.repo.Create(ctx, &model.AuditLog{
		ActorUserID:      input.ActorUserID,
		ProjectID:        input.ProjectID,
		Action:           input.Action,
		TargetType:       input.TargetType,
		TargetIdentifier: input.TargetIdentifier,
		Result:           input.Result,
		RequestID:        input.RequestID,
		Metadata:         metadata,
	})
}
