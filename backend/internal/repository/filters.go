package repository

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
	Result           string
	Limit            int
	Offset           int
}
