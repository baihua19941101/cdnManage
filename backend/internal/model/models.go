package model

func AllModels() []any {
	return []any{
		&User{},
		&Project{},
		&UserProjectRole{},
		&ProjectBucket{},
		&ProjectCDN{},
		&AuditLog{},
	}
}
