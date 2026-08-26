package access

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"time"

	persistence "github.com/hvritual/biz/internal/iam/infrastructure/persistence"
	"gorm.io/gorm"
	"yunka.io/framework/core/identity"
)

var (
	ErrUnauthorized = errors.New("iam: unauthorized")
	ErrForbidden    = errors.New("iam: forbidden")
)

const (
	PermissionDeviceRead   = "device.read"
	PermissionDeviceCreate = "device.create"
	PermissionDeviceUpdate = "device.update"
	PermissionDeviceDelete = "device.delete"
)

type DataScope string

const (
	DataScopeNone  DataScope = "none"
	DataScopeSelf  DataScope = "self"
	DataScopeSites DataScope = "sites"
	DataScopeAll   DataScope = "all"
)

type PermissionScope struct {
	Allowed bool
	All     bool
	Self    bool
	Sites   bool
}

type Plan struct {
	Principal   identity.Principal
	Permissions map[string]PermissionScope
	SiteIDs     []string
}

func (plan Plan) Can(permission string) bool {
	scope, ok := plan.Permissions[permission]
	return ok && scope.Allowed
}

func (plan Plan) CanTargetSite(permission, siteID string) bool {
	scope, ok := plan.Permissions[permission]
	if !ok || !scope.Allowed {
		return false
	}
	if scope.All {
		return true
	}
	if !scope.Sites {
		return false
	}
	for _, allowed := range plan.SiteIDs {
		if allowed == siteID {
			return true
		}
	}
	return false
}

type Authenticator struct {
	database *gorm.DB
}

func NewAuthenticator(database *gorm.DB) (*Authenticator, error) {
	if database == nil {
		return nil, errors.New("iam: authentication database is required")
	}
	return &Authenticator{database: database}, nil
}

func (auth *Authenticator) Authenticate(ctx context.Context, rawToken string) (Plan, error) {
	if auth == nil || auth.database == nil || strings.TrimSpace(rawToken) == "" {
		return Plan{}, ErrUnauthorized
	}
	hash := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(hash[:])
	var token persistence.APITokenPORecord
	if err := auth.database.WithContext(ctx).
		Where("token_hash = ? AND disabled = ?", tokenHash, false).
		First(&token).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Plan{}, ErrUnauthorized
		}
		return Plan{}, err
	}
	if token.ExpiresAt != nil && !token.ExpiresAt.After(time.Now()) {
		return Plan{}, ErrUnauthorized
	}

	tenantID := strings.TrimSpace(token.ScopeTenantID)
	userID := strings.TrimSpace(token.UserID)
	if tenantID == "" || userID == "" {
		return Plan{}, ErrUnauthorized
	}
	var membership persistence.MembershipPORecord
	if err := auth.database.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ? AND status = ?", tenantID, userID, "active").
		First(&membership).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Plan{}, ErrUnauthorized
		}
		return Plan{}, err
	}

	type grantRow struct {
		RoleName   string
		Permission string
		DataScope  DataScope
	}
	var rows []grantRow
	memberRoleTable := (persistence.MemberRolePORecord{}).TableName()
	roleTable := (persistence.RolePORecord{}).TableName()
	permissionTable := (persistence.RolePermissionPORecord{}).TableName()
	if err := auth.database.WithContext(ctx).
		Table(memberRoleTable + " mr").
		Select("r.name AS role_name, rp.permission, rp.data_scope").
		Joins("JOIN "+roleTable+" r ON r.id = mr.role_id AND r.tenant_id = mr.tenant_id AND r.status = ? AND r.deleted_at IS NULL", "active").
		Joins("JOIN "+permissionTable+" rp ON rp.role_id = mr.role_id AND rp.tenant_id = mr.tenant_id AND rp.deleted_at IS NULL").
		Where("mr.tenant_id = ? AND mr.user_id = ? AND mr.deleted_at IS NULL", tenantID, userID).
		Scan(&rows).Error; err != nil {
		return Plan{}, err
	}

	roleSet := map[string]struct{}{}
	permissions := make(map[string]PermissionScope)
	for _, row := range rows {
		roleSet[row.RoleName] = struct{}{}
		scope := permissions[row.Permission]
		scope.Allowed = true
		switch row.DataScope {
		case DataScopeAll:
			scope.All = true
		case DataScopeSites:
			scope.Sites = true
		case DataScopeSelf:
			scope.Self = true
		}
		permissions[row.Permission] = scope
	}
	roles := make([]string, 0, len(roleSet))
	for role := range roleSet {
		roles = append(roles, role)
	}
	sort.Strings(roles)

	var siteRows []persistence.MemberSitePORecord
	if err := auth.database.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Find(&siteRows).Error; err != nil {
		return Plan{}, err
	}
	sites := make([]string, 0, len(siteRows))
	seenSites := map[string]struct{}{}
	for _, row := range siteRows {
		if row.SiteID == "" {
			continue
		}
		if _, exists := seenSites[row.SiteID]; exists {
			continue
		}
		seenSites[row.SiteID] = struct{}{}
		sites = append(sites, row.SiteID)
	}
	sort.Strings(sites)

	principal := identity.Principal{
		Subject:       "user:" + userID,
		TenantID:      tenantID,
		UserID:        userID,
		Roles:         roles,
		AuthMethod:    "bearer-token",
		Authenticated: true,
	}
	return Plan{Principal: principal, Permissions: permissions, SiteIDs: sites}, nil
}
