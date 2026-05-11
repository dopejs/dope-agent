package tenancy

type ThreadAccessScope struct {
	TenantID string
}

func (scope ThreadAccessScope) Allows(threadTenantID string) bool {
	return scope.TenantID != "" && scope.TenantID == threadTenantID
}
