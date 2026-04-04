package model

type UserProjectRole struct {
	BaseModel
	UserID      uint64  `gorm:"not null;uniqueIndex:idx_user_project_role"`
	ProjectID   uint64  `gorm:"not null;uniqueIndex:idx_user_project_role;index"`
	ProjectRole string  `gorm:"type:varchar(32);not null;index;check:project_role IN ('project_admin','project_read_only')"`
	User        User    `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Project     Project `gorm:"foreignKey:ProjectID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}
