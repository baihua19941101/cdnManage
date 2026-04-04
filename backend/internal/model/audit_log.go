package model

import "gorm.io/datatypes"

type AuditLog struct {
	BaseModel
	ActorUserID      uint64         `gorm:"not null;index"`
	ProjectID        uint64         `gorm:"not null;index"`
	Action           string         `gorm:"type:varchar(64);not null;index"`
	TargetType       string         `gorm:"type:varchar(64);not null;index"`
	TargetIdentifier string         `gorm:"type:varchar(255);not null"`
	Result           string         `gorm:"type:varchar(32);not null;index;check:result IN ('success','failure','denied')"`
	RequestID        string         `gorm:"type:varchar(64);not null;index"`
	Metadata         datatypes.JSON `gorm:"type:json"`
	ActorUser        User           `gorm:"foreignKey:ActorUserID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
	Project          Project        `gorm:"foreignKey:ProjectID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
}
