package authz

import (
	"strings"

	"github.com/hvritual/yunka.io/gateway/rpc/meta"
)

func PolicyFromRuntimeAPI(api *meta.RuntimeApi) Policy {
	if api == nil || api.Authorization == nil {
		return Policy{}
	}
	mode := PermissionAll
	if api.Authorization.Mode == meta.PermissionMode_PermissionAny {
		mode = PermissionAny
	}
	permissions := make([]PermissionKey, 0, len(api.Authorization.Permissions))
	for _, value := range api.Authorization.Permissions {
		if value = strings.TrimSpace(value); value != "" {
			permissions = append(permissions, PermissionKey(value))
		}
	}
	return Normalize(Policy{
		Operation:      OperationID(api.Authorization.OperationId),
		Permissions:    permissions,
		Mode:           mode,
		TenantRequired: api.Authorization.TenantRequired,
		Authentication: append([]string(nil), api.Authorization.Authentication...),
	})
}
