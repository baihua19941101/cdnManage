package model

type ProjectBucket struct {
	BaseModel
	ProjectID            uint64  `gorm:"not null;index;uniqueIndex:idx_project_provider_bucket"`
	ProviderType         string  `gorm:"type:varchar(32);not null;default:unknown;index;uniqueIndex:idx_project_provider_bucket"`
	BucketName           string  `gorm:"type:varchar(255);not null;uniqueIndex:idx_project_provider_bucket"`
	Region               string  `gorm:"type:varchar(128)"`
	CredentialCiphertext string  `gorm:"type:longtext;not null"`
	IsPrimary            bool    `gorm:"not null;default:false;index"`
	Project              Project `gorm:"foreignKey:ProjectID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}
