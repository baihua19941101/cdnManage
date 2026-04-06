package model

type ProjectCDN struct {
	BaseModel
	ProjectID            uint64  `gorm:"not null;index;uniqueIndex:idx_project_cdn_endpoint"`
	ProviderType         string  `gorm:"type:varchar(32);not null;default:unknown;index"`
	CDNEndpoint          string  `gorm:"column:cdn_endpoint;type:varchar(255);not null;uniqueIndex:idx_project_cdn_endpoint"`
	Region               string  `gorm:"type:varchar(128)"`
	CredentialCiphertext string  `gorm:"type:longtext;not null"`
	PurgeScope           string  `gorm:"type:varchar(32);not null;default:url"`
	IsPrimary            bool    `gorm:"not null;default:false;index"`
	Project              Project `gorm:"foreignKey:ProjectID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}
