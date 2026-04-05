package model

const (
	PlatformRoleSuperAdmin = "super_admin"
	PlatformRoleAdmin      = "platform_admin"
	PlatformRoleStandard   = "standard_user"
	UserStatusActive       = "active"
	UserStatusDisabled     = "disabled"
	ProjectRoleAdmin       = "project_admin"
	ProjectRoleReadOnly    = "project_read_only"
	ProviderTypeUnknown    = "unknown"
	AuditResultSuccess     = "success"
	AuditResultFailure     = "failure"
	AuditResultDenied      = "denied"
)

func IsPlatformAdminRole(role string) bool {
	return role == PlatformRoleSuperAdmin || role == PlatformRoleAdmin
}

func IsKnownPlatformRole(role string) bool {
	switch role {
	case PlatformRoleSuperAdmin, PlatformRoleAdmin, PlatformRoleStandard:
		return true
	default:
		return false
	}
}

func IsKnownProjectRole(role string) bool {
	switch role {
	case ProjectRoleAdmin, ProjectRoleReadOnly:
		return true
	default:
		return false
	}
}

func CanReadProject(platformRole, projectRole string) bool {
	if IsPlatformAdminRole(platformRole) {
		return true
	}

	return projectRole == ProjectRoleAdmin || projectRole == ProjectRoleReadOnly
}

func CanWriteProject(platformRole, projectRole string) bool {
	if IsPlatformAdminRole(platformRole) {
		return true
	}

	return projectRole == ProjectRoleAdmin
}
