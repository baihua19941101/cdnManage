package repository

import "time"

type UserFilter struct {
	Status       string
	PlatformRole string
}

type ProjectFilter struct {
	Name string
}

type AuditLogFilter struct {
	ProjectID        *uint64
	ActorUserID      *uint64
	Action           string
	TargetType       string
	TargetIdentifier string
	SessionID        string
	Result           string
	CreatedAfter     *time.Time
	CreatedBefore    *time.Time
	Limit            int
	Offset           int
}
