package model

type User struct {
	BaseModel
	Username     string            `gorm:"type:varchar(64);not null;uniqueIndex"`
	Email        string            `gorm:"type:varchar(255);not null;uniqueIndex"`
	PasswordHash string            `gorm:"type:varchar(255);not null"`
	Status       string            `gorm:"type:varchar(32);not null;default:active;index;check:status IN ('active','disabled')"`
	PlatformRole string            `gorm:"type:varchar(32);not null;default:standard_user;index;check:platform_role IN ('super_admin','platform_admin','standard_user')"`
	ProjectRoles []UserProjectRole `gorm:"foreignKey:UserID"`
	AuditLogs    []AuditLog        `gorm:"foreignKey:ActorUserID"`
}
