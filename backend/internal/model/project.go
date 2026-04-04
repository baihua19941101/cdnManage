package model

type Project struct {
	BaseModel
	Name         string            `gorm:"type:varchar(128);not null;uniqueIndex"`
	Description  string            `gorm:"type:text"`
	ProjectRoles []UserProjectRole `gorm:"foreignKey:ProjectID"`
	Buckets      []ProjectBucket   `gorm:"foreignKey:ProjectID"`
	CDNs         []ProjectCDN      `gorm:"foreignKey:ProjectID"`
	AuditLogs    []AuditLog        `gorm:"foreignKey:ProjectID"`
}
