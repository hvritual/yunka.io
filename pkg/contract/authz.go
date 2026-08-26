package contract

import (
	"sort"
	"strconv"
	"strings"
)

func authorizationFromDirectives(directives map[string]string) *AuthorizationPolicy {
	if len(directives) == 0 {
		return nil
	}
	operation := strings.TrimSpace(directives["operation"])
	permissions := splitDirectiveList(directives["permission"])
	authentication := splitDirectiveList(directives["authentication"])
	mode := strings.ToLower(strings.TrimSpace(directives["permission_mode"]))
	if mode == "" {
		mode = "all"
	}
	tenantRequired, _ := strconv.ParseBool(strings.TrimSpace(directives["tenant_required"]))
	if operation == "" && len(permissions) == 0 && len(authentication) == 0 && directives["tenant_required"] == "" && directives["permission_mode"] == "" {
		return nil
	}
	return &AuthorizationPolicy{OperationID: operation, Permissions: permissions, PermissionMode: mode, TenantRequired: tenantRequired, Authentication: authentication}
}

func splitDirectiveList(value string) []string {
	seen := map[string]struct{}{}
	var result []string
	for _, item := range strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == '|' || r == ' ' || r == '\t' }) {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	sort.Strings(result)
	return result
}
