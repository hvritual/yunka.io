from pathlib import Path
import sys

root = Path(sys.argv[1]).resolve()


def write(path: str, content: str) -> None:
    target = root / path
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(content, encoding="utf-8")


def remove(path: str) -> None:
    target = root / path
    if target.exists():
        target.unlink()


# C8.5.1: Access/IAM is a business domain, not a framework-owned data model.
remove("internal/access/store/store.go")

write("internal/access/domain/model.go", r'''package domain

import "time"

type DataScope string

const (
	DataScopeNone  DataScope = "none"
	DataScopeSelf  DataScope = "self"
	DataScopeSites DataScope = "sites"
	DataScopeAll   DataScope = "all"
)

type Tenant struct {
	ID        string
	Name      string
	Status    string
	CreatedAt time.Time
}

type User struct {
	ID        string
	Email     string
	Status    string
	CreatedAt time.Time
}

type Membership struct {
	TenantID  string
	UserID    string
	Status    string
	CreatedAt time.Time
}

type Role struct {
	ID       string
	TenantID string
	Name     string
	Status   string
}

type MemberRole struct {
	TenantID string
	UserID   string
	RoleID   string
}

type PermissionGrant struct {
	TenantID   string
	RoleID     string
	Permission string
	Scope      DataScope
}

type MemberSite struct {
	TenantID string
	UserID   string
	SiteID   string
}

type Credential struct {
	TokenHash string
	TenantID  string
	UserID    string
	ExpiresAt *time.Time
	Disabled  bool
	CreatedAt time.Time
}
''')

write("internal/access/ports/security.go", r'''package ports

import (
	"context"

	"yunka.io/framework/core/identity"
	"yunka.io/gateway/authz"
)

type Authenticator interface {
	Authenticate(context.Context, string) (identity.Principal, error)
}

type GrantResolver interface {
	ResolveGrants(context.Context, string, []string, []authz.PermissionKey) ([]authz.Grant, error)
}

type MemberSiteResolver interface {
	ResolveMemberSites(context.Context, string, string) ([]string, error)
}
''')

write("internal/access/infrastructure/persistence/store.go", r'''package persistence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"time"

	accessdomain "github.com/hvritual/biz/internal/access/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"yunka.io/framework/core/identity"
	"yunka.io/gateway/authz"
)

var ErrUnauthorized = errors.New("access: unauthorized")

type tenantRecord struct {
	ID        string    `gorm:"column:id;primaryKey;size:64"`
	Name      string    `gorm:"column:name;size:200;not null"`
	Status    string    `gorm:"column:status;size:32;not null;index"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
}
func (tenantRecord) TableName() string { return "biz_tenants" }

type userRecord struct {
	ID        string    `gorm:"column:id;primaryKey;size:64"`
	Email     string    `gorm:"column:email;size:320;not null;uniqueIndex"`
	Status    string    `gorm:"column:status;size:32;not null;index"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
}
func (userRecord) TableName() string { return "biz_users" }

type membershipRecord struct {
	TenantID  string    `gorm:"column:tenant_id;primaryKey;size:64"`
	UserID    string    `gorm:"column:user_id;primaryKey;size:64"`
	Status    string    `gorm:"column:status;size:32;not null;index"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
}
func (membershipRecord) TableName() string { return "biz_memberships" }

type roleRecord struct {
	ID       string `gorm:"column:id;primaryKey;size:160"`
	TenantID string `gorm:"column:tenant_id;size:64;not null;index:idx_role_tenant;uniqueIndex:uniq_role_name,priority:1"`
	Name     string `gorm:"column:name;size:100;not null;uniqueIndex:uniq_role_name,priority:2"`
	Status   string `gorm:"column:status;size:32;not null"`
}
func (roleRecord) TableName() string { return "biz_roles" }

type memberRoleRecord struct {
	TenantID string `gorm:"column:tenant_id;primaryKey;size:64"`
	UserID   string `gorm:"column:user_id;primaryKey;size:64"`
	RoleID   string `gorm:"column:role_id;primaryKey;size:160"`
}
func (memberRoleRecord) TableName() string { return "biz_member_roles" }

// permissionGrantRecord is the authoritative role permission + scope fact.
// Scope cannot exist independently from the permission grant.
type permissionGrantRecord struct {
	TenantID   string                 `gorm:"column:tenant_id;primaryKey;size:64"`
	RoleID     string                 `gorm:"column:role_id;primaryKey;size:160"`
	Permission string                 `gorm:"column:permission;primaryKey;size:120"`
	Scope      accessdomain.DataScope `gorm:"column:scope;size:16;not null"`
}
func (permissionGrantRecord) TableName() string { return "biz_permission_grants" }

type memberSiteRecord struct {
	TenantID string `gorm:"column:tenant_id;primaryKey;size:64"`
	UserID   string `gorm:"column:user_id;primaryKey;size:64"`
	SiteID   string `gorm:"column:site_id;primaryKey;size:64"`
}
func (memberSiteRecord) TableName() string { return "biz_member_sites" }

type apiTokenRecord struct {
	TokenHash string     `gorm:"column:token_hash;primaryKey;size:64"`
	TenantID  string     `gorm:"column:tenant_id;size:64;not null;index"`
	UserID    string     `gorm:"column:user_id;size:64;not null;index"`
	ExpiresAt *time.Time `gorm:"column:expires_at"`
	Disabled  bool       `gorm:"column:disabled;not null;default:false"`
	CreatedAt time.Time  `gorm:"column:created_at;not null"`
}
func (apiTokenRecord) TableName() string { return "biz_api_tokens" }

type Store struct{ database *gorm.DB }

func New(database *gorm.DB) (*Store, error) {
	if database == nil {
		return nil, errors.New("access: database is required")
	}
	return &Store{database: database}, nil
}

func (store *Store) AutoMigrate(ctx context.Context) error {
	return store.database.WithContext(ctx).AutoMigrate(
		&tenantRecord{}, &userRecord{}, &membershipRecord{}, &roleRecord{},
		&memberRoleRecord{}, &permissionGrantRecord{}, &memberSiteRecord{}, &apiTokenRecord{},
	)
}

func TokenHash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func (store *Store) Authenticate(ctx context.Context, rawToken string) (identity.Principal, error) {
	if store == nil || store.database == nil || strings.TrimSpace(rawToken) == "" {
		return identity.Principal{}, ErrUnauthorized
	}
	var token apiTokenRecord
	if err := store.database.WithContext(ctx).Where("token_hash = ? AND disabled = ?", TokenHash(rawToken), false).First(&token).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return identity.Principal{}, ErrUnauthorized
		}
		return identity.Principal{}, err
	}
	if token.ExpiresAt != nil && !token.ExpiresAt.After(time.Now()) {
		return identity.Principal{}, ErrUnauthorized
	}
	var membership membershipRecord
	if err := store.database.WithContext(ctx).Where("tenant_id = ? AND user_id = ? AND status = ?", token.TenantID, token.UserID, "active").First(&membership).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return identity.Principal{}, ErrUnauthorized
		}
		return identity.Principal{}, err
	}
	var roles []string
	if err := store.database.WithContext(ctx).Table("biz_member_roles mr").
		Select("r.name").
		Joins("JOIN biz_roles r ON r.id = mr.role_id AND r.tenant_id = mr.tenant_id AND r.status = ?", "active").
		Where("mr.tenant_id = ? AND mr.user_id = ?", token.TenantID, token.UserID).
		Scan(&roles).Error; err != nil {
		return identity.Principal{}, err
	}
	sort.Strings(roles)
	return identity.Principal{
		Subject: "user:" + token.UserID, TenantID: token.TenantID, UserID: token.UserID,
		Roles: roles, AuthMethod: identity.AuthMethodAPIKey, Authenticated: true,
	}, nil
}

// ResolveGrants is the C8.5 unified authorization decision input. The JOIN binds
// permission and scope to the exact same active role grant and ignores legacy
// role_data_scopes rows entirely.
func (store *Store) ResolveGrants(ctx context.Context, tenantID string, roles []string, permissions []authz.PermissionKey) ([]authz.Grant, error) {
	if strings.TrimSpace(tenantID) == "" || len(roles) == 0 || len(permissions) == 0 {
		return nil, nil
	}
	keys := make([]string, 0, len(permissions))
	for _, permission := range permissions {
		if value := strings.TrimSpace(string(permission)); value != "" {
			keys = append(keys, value)
		}
	}
	if len(keys) == 0 {
		return nil, nil
	}
	type grantRow struct {
		RoleID     string
		Permission string
		Scope      accessdomain.DataScope
	}
	var rows []grantRow
	if err := store.database.WithContext(ctx).Table("biz_roles r").
		Select("r.id AS role_id, pg.permission, pg.scope").
		Joins("JOIN biz_permission_grants pg ON pg.role_id = r.id AND pg.tenant_id = r.tenant_id").
		Where("r.tenant_id = ? AND r.status = ? AND r.name IN ? AND pg.permission IN ?", tenantID, "active", roles, keys).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]authz.Grant, 0, len(rows))
	for _, row := range rows {
		result = append(result, authz.Grant{Permission: authz.PermissionKey(row.Permission), RoleID: row.RoleID, Scope: string(row.Scope)})
	}
	return result, nil
}

func (store *Store) ResolveMemberSites(ctx context.Context, tenantID, userID string) ([]string, error) {
	if strings.TrimSpace(tenantID) == "" || strings.TrimSpace(userID) == "" {
		return nil, nil
	}
	var rows []memberSiteRecord
	if err := store.database.WithContext(ctx).Where("tenant_id = ? AND user_id = ?", tenantID, userID).Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]string, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.SiteID)
	}
	sort.Strings(result)
	return result, nil
}

type Bootstrap struct{ TenantID, TenantName, UserID, Email, Token string }

func (store *Store) Bootstrap(ctx context.Context, config Bootstrap, permissions []authz.PermissionKey) error {
	if strings.TrimSpace(config.Token) == "" {
		return nil
	}
	roleID := config.TenantID + ":owner"
	values := []any{
		&tenantRecord{ID: config.TenantID, Name: config.TenantName, Status: "active"},
		&userRecord{ID: config.UserID, Email: config.Email, Status: "active"},
		&membershipRecord{TenantID: config.TenantID, UserID: config.UserID, Status: "active"},
		&roleRecord{ID: roleID, TenantID: config.TenantID, Name: "owner", Status: "active"},
		&memberRoleRecord{TenantID: config.TenantID, UserID: config.UserID, RoleID: roleID},
		&apiTokenRecord{TokenHash: TokenHash(config.Token), TenantID: config.TenantID, UserID: config.UserID},
	}
	db := store.database.WithContext(ctx)
	for _, value := range values {
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(value).Error; err != nil {
			return err
		}
	}
	for _, permission := range permissions {
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&permissionGrantRecord{
			TenantID: config.TenantID, RoleID: roleID, Permission: string(permission), Scope: accessdomain.DataScopeAll,
		}).Error; err != nil {
			return err
		}
	}
	return nil
}
''')

# C8.5.3-C8.5.4: domain-owned typed scope is prepared before Application.
write("internal/deviceops/security/scope.go", r'''package security

import (
	"context"
	"errors"
	"sort"
	"strings"

	deviceopsv1 "github.com/hvritual/biz/contracts/gen/deviceops/v1"
	accessdomain "github.com/hvritual/biz/internal/access/domain"
	accessports "github.com/hvritual/biz/internal/access/ports"
	"yunka.io/gateway/authz"
)

var ErrAuthorizedScopeMissing = errors.New("deviceops security: authorized scope missing")

type Scope struct {
	All     bool
	Self    bool
	Sites   bool
	UserID  string
	SiteIDs []string
}

func (scope Scope) AllowsSite(siteID string) bool {
	if scope.All || scope.Self {
		return true
	}
	if !scope.Sites {
		return false
	}
	for _, allowed := range scope.SiteIDs {
		if allowed == siteID {
			return true
		}
	}
	return false
}

type scopeKey struct{}

func WithScope(ctx context.Context, scope Scope) context.Context {
	return context.WithValue(ctx, scopeKey{}, scope)
}

func FromContext(ctx context.Context) (Scope, bool) {
	if ctx == nil {
		return Scope{}, false
	}
	scope, ok := ctx.Value(scopeKey{}).(Scope)
	return scope, ok
}

func RequireScope(ctx context.Context) (Scope, error) {
	scope, ok := FromContext(ctx)
	if !ok {
		return Scope{}, ErrAuthorizedScopeMissing
	}
	return scope, nil
}

type Guard struct{ sites accessports.MemberSiteResolver }

func NewGuard(sites accessports.MemberSiteResolver) (*Guard, error) {
	if sites == nil {
		return nil, errors.New("deviceops security: member site resolver is required")
	}
	return &Guard{sites: sites}, nil
}

func (guard *Guard) Prepare(ctx context.Context, authorized authz.AuthorizedOperation, input any) (context.Context, error) {
	scope := Scope{UserID: authorized.Principal.UserID}
	for _, grant := range authorized.Decision.Grants {
		switch accessdomain.DataScope(strings.TrimSpace(grant.Scope)) {
		case accessdomain.DataScopeAll:
			scope.All = true
		case accessdomain.DataScopeSites:
			scope.Sites = true
		case accessdomain.DataScopeSelf:
			scope.Self = true
		}
	}
	if !scope.All && !scope.Sites && !scope.Self {
		return nil, denied(authorized)
	}
	if (scope.Sites || scope.Self) && strings.TrimSpace(scope.UserID) == "" {
		return nil, denied(authorized)
	}
	if scope.Sites {
		sites, err := guard.sites.ResolveMemberSites(ctx, authorized.Principal.TenantID, authorized.Principal.UserID)
		if err != nil {
			return nil, err
		}
		scope.SiteIDs = append([]string(nil), sites...)
		sort.Strings(scope.SiteIDs)
	}
	// Resource write scope is resolved before the Application boundary.
	switch request := input.(type) {
	case *deviceopsv1.CreateDeviceRequest:
		if siteID := strings.TrimSpace(request.GetSiteId()); siteID != "" && !scope.AllowsSite(siteID) {
			return nil, denied(authorized)
		}
	case *deviceopsv1.UpdateDeviceRequest:
		if siteID := strings.TrimSpace(request.GetSiteId()); siteID != "" && !scope.AllowsSite(siteID) {
			return nil, denied(authorized)
		}
	}
	return WithScope(ctx, scope), nil
}

func denied(authorized authz.AuthorizedOperation) error {
	decision := authorized.Decision
	decision.Allowed = false
	decision.Reason = authz.ReasonPermissionDenied
	return authz.Denied(decision)
}
''')

write("internal/deviceops/ports/scoped.go", r'''package ports

import (
	"context"

	"github.com/hvritual/biz/internal/deviceops/domain"
)

type ScopedDeviceRepository interface {
	DeviceRepository
	ListVisible(context.Context) ([]domain.Device, error)
	GetVisible(context.Context, string) (domain.Device, error)
}

type ScopedRepositories struct {
	Device ScopedDeviceRepository
	Site   SiteRepository
}
''')

write("internal/deviceops/infrastructure/persistence/scoped.go", r'''package persistence

import (
	"context"
	"errors"

	"github.com/hvritual/biz/internal/deviceops/domain"
	"github.com/hvritual/biz/internal/deviceops/ports"
	devicesecurity "github.com/hvritual/biz/internal/deviceops/security"
	"gorm.io/gorm"
	"yunka.io/framework/requestscope"
)

func applyDeviceDataScope(query *gorm.DB, scope devicesecurity.Scope) *gorm.DB {
	if scope.All {
		return query
	}
	if scope.Sites && scope.Self {
		if len(scope.SiteIDs) == 0 {
			return query.Where("created_by = ?", scope.UserID)
		}
		return query.Where("(site_id IN ? OR created_by = ?)", scope.SiteIDs, scope.UserID)
	}
	if scope.Sites {
		if len(scope.SiteIDs) == 0 {
			return query.Where("1 = 0")
		}
		return query.Where("site_id IN ?", scope.SiteIDs)
	}
	if scope.Self {
		return query.Where("created_by = ?", scope.UserID)
	}
	return query.Where("1 = 0")
}

func (repository *DeviceRepository) ListVisible(ctx context.Context) ([]domain.Device, error) {
	scope, err := devicesecurity.RequireScope(ctx)
	if err != nil {
		return nil, err
	}
	query, err := repository.scoped(ctx)
	if err != nil {
		return nil, err
	}
	var rows []DevicePORecord
	if err := applyDeviceDataScope(query, scope).Order("created_at DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]domain.Device, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.Domain())
	}
	return result, nil
}

func (repository *DeviceRepository) GetVisible(ctx context.Context, id string) (domain.Device, error) {
	scope, err := devicesecurity.RequireScope(ctx)
	if err != nil {
		return domain.Device{}, err
	}
	query, err := repository.scoped(ctx)
	if err != nil {
		return domain.Device{}, err
	}
	var row DevicePORecord
	if err := applyDeviceDataScope(query, scope).Where("id = ?", id).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Device{}, ports.ErrNotFound
		}
		return domain.Device{}, err
	}
	return row.Domain(), nil
}

func NewScopedScopeFactory(database *gorm.DB) (requestscope.ScopeFactory[ports.ScopedRepositories], error) {
	unit, err := requestscope.NewGORMFactory(database, nil)
	if err != nil {
		return nil, err
	}
	return requestscope.NewFactory(requestscope.FactoryOptions[ports.ScopedRepositories]{
		UnitOfWork: unit,
		Repositories: requestscope.GORMRepositories(func(ctx context.Context, transaction *gorm.DB) (ports.ScopedRepositories, error) {
			devices, err := NewDeviceRepository(transaction)
			if err != nil {
				return ports.ScopedRepositories{}, err
			}
			sites, err := NewSiteRepository(transaction)
			if err != nil {
				return ports.ScopedRepositories{}, err
			}
			return ports.ScopedRepositories{Device: devices, Site: sites}, nil
		}),
	})
}

func EnsureIndexes(database *gorm.DB) error {
	if database == nil {
		return errors.New("deviceops persistence: database is required")
	}
	if !database.Migrator().HasIndex(&DevicePORecord{}, "uniq_deviceops_tenant_serial") {
		if err := database.Exec("CREATE UNIQUE INDEX uniq_deviceops_tenant_serial ON biz_deviceops_device (tenant_id, serial)").Error; err != nil {
			return err
		}
	}
	return nil
}
''')

# C8.5.5: Application has no permission literals, Authorizer or IAM dependency.
write("internal/deviceops/application/service.go", r'''package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"

	deviceopsv1 "github.com/hvritual/biz/contracts/gen/deviceops/v1"
	"github.com/hvritual/biz/internal/deviceops/domain"
	"github.com/hvritual/biz/internal/deviceops/ports"
	devicesecurity "github.com/hvritual/biz/internal/deviceops/security"
	"yunka.io/framework/requestscope"
)

var ErrInvalid = errors.New("deviceops: invalid request")

type Service struct {
	scopes requestscope.ScopeFactory[ports.ScopedRepositories]
}

func NewService(scopes requestscope.ScopeFactory[ports.ScopedRepositories]) (*Service, error) {
	if scopes == nil {
		return nil, errors.New("deviceops: request scope factory is required")
	}
	return &Service{scopes: scopes}, nil
}

func (service *Service) ListDevices(ctx context.Context, _ *deviceopsv1.ListDevicesRequest) (*deviceopsv1.ListDevicesResponse, error) {
	devices, err := requestscope.ExecuteValue(ctx, service.scopes, func(scope *requestscope.Scope[ports.ScopedRepositories]) ([]domain.Device, error) {
		return scope.Repositories().Device.ListVisible(scope.Context())
	})
	if err != nil {
		return nil, err
	}
	result := &deviceopsv1.ListDevicesResponse{Devices: make([]*deviceopsv1.DeviceDTO, 0, len(devices))}
	for _, device := range devices {
		result.Devices = append(result.Devices, toDTO(device))
	}
	return result, nil
}

func (service *Service) GetDevice(ctx context.Context, request *deviceopsv1.GetDeviceRequest) (*deviceopsv1.DeviceDTO, error) {
	if request == nil || strings.TrimSpace(request.GetId()) == "" {
		return nil, ErrInvalid
	}
	device, err := requestscope.ExecuteValue(ctx, service.scopes, func(scope *requestscope.Scope[ports.ScopedRepositories]) (domain.Device, error) {
		return scope.Repositories().Device.GetVisible(scope.Context(), strings.TrimSpace(request.GetId()))
	})
	if err != nil {
		return nil, err
	}
	return toDTO(device), nil
}

func (service *Service) CreateDevice(ctx context.Context, request *deviceopsv1.CreateDeviceRequest) (*deviceopsv1.DeviceDTO, error) {
	if request == nil {
		return nil, ErrInvalid
	}
	siteID, name, serial := strings.TrimSpace(request.GetSiteId()), strings.TrimSpace(request.GetName()), strings.TrimSpace(request.GetSerial())
	if siteID == "" || name == "" || serial == "" {
		return nil, ErrInvalid
	}
	access, err := devicesecurity.RequireScope(ctx)
	if err != nil || strings.TrimSpace(access.UserID) == "" {
		return nil, devicesecurity.ErrAuthorizedScopeMissing
	}
	device, err := requestscope.ExecuteValue(ctx, service.scopes, func(scope *requestscope.Scope[ports.ScopedRepositories]) (domain.Device, error) {
		if _, err := scope.Repositories().Site.Get(scope.Context(), siteID); err != nil {
			return domain.Device{}, err
		}
		value := domain.Device{ID: newID(), SiteID: siteID, Name: name, Serial: serial, CreatedBy: access.UserID}
		if err := scope.Repositories().Device.Create(scope.Context(), &value); err != nil {
			return domain.Device{}, err
		}
		return value, nil
	})
	if err != nil {
		return nil, err
	}
	return toDTO(device), nil
}

func (service *Service) UpdateDevice(ctx context.Context, request *deviceopsv1.UpdateDeviceRequest) (*deviceopsv1.DeviceDTO, error) {
	if request == nil || strings.TrimSpace(request.GetId()) == "" || request.GetVersion() == 0 {
		return nil, ErrInvalid
	}
	device, err := requestscope.ExecuteValue(ctx, service.scopes, func(scope *requestscope.Scope[ports.ScopedRepositories]) (domain.Device, error) {
		current, err := scope.Repositories().Device.GetVisible(scope.Context(), strings.TrimSpace(request.GetId()))
		if err != nil {
			return domain.Device{}, err
		}
		name := strings.TrimSpace(request.GetName())
		if name == "" {
			name = current.Name
		}
		siteID := strings.TrimSpace(request.GetSiteId())
		if siteID == "" {
			siteID = current.SiteID
		}
		if siteID != current.SiteID {
			if _, err := scope.Repositories().Site.Get(scope.Context(), siteID); err != nil {
				return domain.Device{}, err
			}
		}
		current.Name, current.SiteID = name, siteID
		if err := scope.Repositories().Device.Update(scope.Context(), &current, request.GetVersion()); err != nil {
			return domain.Device{}, err
		}
		return scope.Repositories().Device.GetVisible(scope.Context(), current.ID)
	})
	if err != nil {
		return nil, err
	}
	return toDTO(device), nil
}

func (service *Service) DeleteDevice(ctx context.Context, request *deviceopsv1.DeleteDeviceRequest) (*deviceopsv1.DeleteDeviceResponse, error) {
	if request == nil || strings.TrimSpace(request.GetId()) == "" || request.GetVersion() == 0 {
		return nil, ErrInvalid
	}
	err := requestscope.Execute(ctx, service.scopes, func(scope *requestscope.Scope[ports.ScopedRepositories]) error {
		current, err := scope.Repositories().Device.GetVisible(scope.Context(), strings.TrimSpace(request.GetId()))
		if err != nil {
			return err
		}
		return scope.Repositories().Device.Delete(scope.Context(), current.ID, request.GetVersion())
	})
	if err != nil {
		return nil, err
	}
	return &deviceopsv1.DeleteDeviceResponse{}, nil
}

func toDTO(device domain.Device) *deviceopsv1.DeviceDTO {
	return &deviceopsv1.DeviceDTO{Id: device.ID, SiteId: device.SiteID, Name: device.Name, Serial: device.Serial, CreatedBy: device.CreatedBy, Version: device.Version}
}

func newID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(value[:])
}
''')

write("internal/deviceops/application/security_boundary_test.go", r'''package application

import (
	"os"
	"strings"
	"testing"
)

func TestApplicationContainsNoAuthorizationPolicy(t *testing.T) {
	source, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, forbidden := range []string{
		"device.read", "device.create", "device.update", "device.delete",
		"ResolveGrants", "ResolveDeviceScope", "HasPermissions", "Authorize(",
		"internal/access", "gateway/authz",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("Application security boundary leak %q in service.go", forbidden)
		}
	}
}
''')

# C8.5.6: module composes one OperationRuntime for REST and gRPC.
write("modules/deviceops/module.go", r'''package deviceops

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	deviceapp "github.com/hvritual/biz/internal/deviceops/application"
	"github.com/hvritual/biz/internal/deviceops/domain"
	devicepersistence "github.com/hvritual/biz/internal/deviceops/infrastructure/persistence"
	devicepolicy "github.com/hvritual/biz/internal/deviceops/policy"
	devicesecurity "github.com/hvritual/biz/internal/deviceops/security"
	devicerest "github.com/hvritual/biz/internal/deviceops/transport/rest"
	devicerpc "github.com/hvritual/biz/internal/deviceops/transport/rpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"yunka.io/framework/core/identity"
	"yunka.io/gateway/authz"
	authzgrpc "yunka.io/gateway/rpc/transport/grpc"
)

const ModuleName = "deviceops"

type Module struct {
	dependencies Dependencies
	mu           sync.RWMutex
	httpServer   *http.Server
	grpcServer   *grpc.Server
	httpListener net.Listener
	grpcListener net.Listener
	serveErr     error
}

func NewModule(dependencies Dependencies) (*Module, error) {
	if dependencies.Logger == nil {
		return nil, errors.New("deviceops: logger is required")
	}
	if dependencies.PrimaryDatabase == nil {
		return nil, errors.New("deviceops: primary database is required")
	}
	if err := dependencies.Config.Validate(); err != nil {
		return nil, err
	}
	return &Module{dependencies: dependencies}, nil
}
func (*Module) Name() string { return ModuleName }

func (module *Module) Start(ctx context.Context) error {
	accessStore, err := accesspersistence.New(module.dependencies.PrimaryDatabase)
	if err != nil {
		return err
	}
	if module.dependencies.Config.AutoMigrate {
		if err := accessStore.AutoMigrate(ctx); err != nil {
			return fmt.Errorf("deviceops: access migrate: %w", err)
		}
		if err := devicepersistence.AutoMigrate(ctx, module.dependencies.PrimaryDatabase); err != nil {
			return fmt.Errorf("deviceops: domain migrate: %w", err)
		}
		if err := devicepersistence.EnsureIndexes(module.dependencies.PrimaryDatabase); err != nil {
			return fmt.Errorf("deviceops: indexes: %w", err)
		}
	}
	if config := module.dependencies.Config.Bootstrap; config.Token != "" {
		if err := accessStore.Bootstrap(ctx, accesspersistence.Bootstrap{TenantID: config.TenantID, TenantName: config.TenantName, UserID: config.UserID, Email: config.Email, Token: config.Token}, devicepolicy.Permissions()); err != nil {
			return fmt.Errorf("deviceops: bootstrap identity: %w", err)
		}
		principal, err := accessStore.Authenticate(ctx, config.Token)
		if err != nil {
			return fmt.Errorf("deviceops: bootstrap authenticate: %w", err)
		}
		siteRepository, err := devicepersistence.NewSiteRepository(module.dependencies.PrimaryDatabase)
		if err != nil {
			return err
		}
		trusted := identity.WithPrincipal(ctx, principal)
		if _, err := siteRepository.Get(trusted, config.SiteID); err != nil {
			site := domain.Site{ID: config.SiteID, Name: config.SiteName}
			if err := siteRepository.Create(trusted, &site); err != nil {
				return fmt.Errorf("deviceops: bootstrap site: %w", err)
			}
		}
	}
	grantAuthorizer, err := authz.NewGrantAuthorizer(accessStore)
	if err != nil {
		return err
	}
	guard, err := devicesecurity.NewGuard(accessStore)
	if err != nil {
		return err
	}
	guards := authz.NewStaticGuardResolver(map[authz.OperationID]authz.OperationGuard{
		devicepolicy.OperationListDevices: guard,
		devicepolicy.OperationGetDevice: guard,
		devicepolicy.OperationCreateDevice: guard,
		devicepolicy.OperationUpdateDevice: guard,
		devicepolicy.OperationDeleteDevice: guard,
	})
	operationRuntime, err := authz.NewOperationRuntime(devicepolicy.Resolver(), grantAuthorizer, guards)
	if err != nil {
		return err
	}
	scopes, err := devicepersistence.NewScopedScopeFactory(module.dependencies.PrimaryDatabase)
	if err != nil {
		return err
	}
	service, err := deviceapp.NewService(scopes)
	if err != nil {
		return err
	}

	apiMux := http.NewServeMux()
	if err := devicerest.Register(apiMux, service, operationRuntime); err != nil {
		return err
	}
	rootMux := http.NewServeMux()
	rootMux.HandleFunc("GET /healthz", module.healthHTTP)
	rootMux.Handle("/v1/", httpAuthentication(accessStore, apiMux))
	httpListener, err := net.Listen("tcp", module.dependencies.Config.HTTPListenAddress)
	if err != nil {
		return fmt.Errorf("deviceops: HTTP listen: %w", err)
	}
	httpServer := &http.Server{Handler: rootMux, ReadHeaderTimeout: 5 * time.Second}

	securityInterceptor, err := authzgrpc.SecuredUnaryServerInterceptor(operationRuntime)
	if err != nil {
		_ = httpListener.Close()
		return err
	}
	grpcListener, err := net.Listen("tcp", module.dependencies.Config.GRPCListenAddress)
	if err != nil {
		_ = httpListener.Close()
		return fmt.Errorf("deviceops: gRPC listen: %w", err)
	}
	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(grpcAuthentication(accessStore), securityInterceptor))
	devicerpc.Register(grpcServer, service)

	module.mu.Lock()
	module.httpServer = httpServer
	module.grpcServer = grpcServer
	module.httpListener = httpListener
	module.grpcListener = grpcListener
	module.serveErr = nil
	module.mu.Unlock()
	go module.serveHTTP()
	go module.serveGRPC()
	return nil
}

func (module *Module) serveHTTP() {
	if err := module.httpServer.Serve(module.httpListener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		module.recordServeError(err)
	}
}
func (module *Module) serveGRPC() {
	if err := module.grpcServer.Serve(module.grpcListener); err != nil {
		module.recordServeError(err)
	}
}
func (module *Module) recordServeError(err error) {
	module.mu.Lock()
	if module.serveErr == nil {
		module.serveErr = err
	}
	module.mu.Unlock()
	module.dependencies.Logger.Errorf("deviceops server: %v", err)
}

func (module *Module) healthHTTP(writer http.ResponseWriter, request *http.Request) {
	if err := module.Health(request.Context()); err != nil {
		http.Error(writer, "unhealthy", http.StatusServiceUnavailable)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	_, _ = writer.Write([]byte(`{"status":"ok"}`))
}

func (module *Module) Health(ctx context.Context) error {
	sqlDB, err := module.dependencies.PrimaryDatabase.DB()
	if err != nil {
		return err
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return err
	}
	module.mu.RLock()
	defer module.mu.RUnlock()
	if module.httpListener == nil || module.grpcListener == nil {
		return errors.New("deviceops: servers not started")
	}
	return module.serveErr
}

func (module *Module) HTTPAddress() string {
	module.mu.RLock()
	defer module.mu.RUnlock()
	if module.httpListener == nil {
		return ""
	}
	return module.httpListener.Addr().String()
}
func (module *Module) GRPCAddress() string {
	module.mu.RLock()
	defer module.mu.RUnlock()
	if module.grpcListener == nil {
		return ""
	}
	return module.grpcListener.Addr().String()
}

func (module *Module) Shutdown(ctx context.Context) error {
	module.mu.RLock()
	httpServer, grpcServer := module.httpServer, module.grpcServer
	module.mu.RUnlock()
	var httpErr error
	if httpServer != nil {
		httpErr = httpServer.Shutdown(ctx)
	}
	if grpcServer != nil {
		done := make(chan struct{})
		go func() { grpcServer.GracefulStop(); close(done) }()
		select {
		case <-done:
		case <-ctx.Done():
			grpcServer.Stop()
		}
	}
	return httpErr
}

func parseBearer(value string) string {
	value = strings.TrimSpace(value)
	if len(value) < 7 || !strings.EqualFold(value[:7], "Bearer ") {
		return ""
	}
	return strings.TrimSpace(value[7:])
}
func httpAuthentication(accessStore *accesspersistence.Store, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, err := accessStore.Authenticate(r.Context(), parseBearer(r.Header.Get("Authorization")))
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(identity.WithPrincipal(r.Context(), principal)))
	})
}
func grpcAuthentication(accessStore *accesspersistence.Store) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		raw := ""
		if values := md.Get("authorization"); len(values) > 0 {
			raw = parseBearer(values[0])
		}
		principal, err := accessStore.Authenticate(ctx, raw)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "unauthenticated")
		}
		return handler(identity.WithPrincipal(ctx, principal), request)
	}
}
''')

write("integration/deviceops_mysql_test.go", r'''//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	deviceopsv1 "github.com/hvritual/biz/contracts/gen/deviceops/v1"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	"github.com/hvritual/biz/modules/deviceops"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"yunka.io/pkg/logExt"
)

func openDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("YUNKA_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Fatal("YUNKA_TEST_MYSQL_DSN is required")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func startModule(t *testing.T, db *gorm.DB, tenant, ownerToken, site string) *deviceops.Module {
	t.Helper()
	config := deviceops.DefaultConfig()
	config.HTTPListenAddress = "127.0.0.1:0"
	config.GRPCListenAddress = "127.0.0.1:0"
	config.AutoMigrate = true
	config.Bootstrap = deviceops.BootstrapConfig{
		TenantID: tenant, TenantName: tenant, UserID: tenant + "-owner",
		Email: tenant + "-owner@example.invalid", Token: ownerToken, SiteID: site, SiteName: site,
	}
	module, err := deviceops.NewModule(deviceops.Dependencies{Config: config, Logger: logExt.NewBaseLogger(), PrimaryDatabase: db})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	if err := module.Start(ctx); err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		shutdown, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		_ = module.Shutdown(shutdown)
	})
	return module
}

func postDevice(t *testing.T, base, token, site, name, serial string) map[string]any {
	t.Helper()
	payload, _ := json.Marshal(map[string]any{"siteId": site, "name": name, "serial": serial})
	req, _ := http.NewRequest(http.MethodPost, base+"/v1/devices", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST status=%d body=%s", resp.StatusCode, body)
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func seedReader(t *testing.T, db *gorm.DB, tenant, user, token, site, roleID, roleName, permission, scope string) {
	t.Helper()
	exec := func(query string, args ...any) {
		if err := db.Exec(query, args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	exec("INSERT IGNORE INTO biz_users (id,email,status,created_at) VALUES (?,?,?,NOW(3))", user, user+"@example.invalid", "active")
	exec("INSERT IGNORE INTO biz_memberships (tenant_id,user_id,status,created_at) VALUES (?,?,?,NOW(3))", tenant, user, "active")
	exec("INSERT IGNORE INTO biz_roles (id,tenant_id,name,status) VALUES (?,?,?,?)", roleID, tenant, roleName, "active")
	exec("INSERT IGNORE INTO biz_member_roles (tenant_id,user_id,role_id) VALUES (?,?,?)", tenant, user, roleID)
	exec("INSERT IGNORE INTO biz_permission_grants (tenant_id,role_id,permission,scope) VALUES (?,?,?,?)", tenant, roleID, permission, scope)
	if site != "" {
		exec("INSERT IGNORE INTO biz_member_sites (tenant_id,user_id,site_id) VALUES (?,?,?)", tenant, user, site)
	}
	exec("INSERT IGNORE INTO biz_api_tokens (token_hash,tenant_id,user_id,disabled,created_at) VALUES (?,?,?,?,NOW(3))", accesspersistence.TokenHash(token), tenant, user, false)
}

func listHTTP(t *testing.T, base, token string) []map[string]any {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, base+"/v1/devices", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list status=%d body=%s", resp.StatusCode, body)
	}
	var out struct{ Devices []map[string]any `json:"devices"` }
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	return out.Devices
}

func TestRESTAndGRPCShareOperationSecurityRuntime(t *testing.T) {
	db := openDB(t)
	tenant, ownerToken, site := "tenant-parity", "owner-parity-token", "site-parity"
	module := startModule(t, db, tenant, ownerToken, site)
	base := "http://" + module.HTTPAddress()
	postDevice(t, base, ownerToken, site, "A", "SN-PARITY-A")

	readerToken := "reader-parity-token"
	seedReader(t, db, tenant, "reader-parity", readerToken, site, tenant+":reader", "reader", "device.read", "sites")
	if devices := listHTTP(t, base, readerToken); len(devices) != 1 || devices[0]["serial"] != "SN-PARITY-A" {
		t.Fatalf("REST reader devices=%v", devices)
	}

	payload, _ := json.Marshal(map[string]any{"siteId": site, "name": "Denied", "serial": "SN-PARITY-DENIED"})
	denied, _ := http.NewRequest(http.MethodPost, base+"/v1/devices", bytes.NewReader(payload))
	denied.Header.Set("Authorization", "Bearer "+readerToken)
	denied.Header.Set("Content-Type", "application/json")
	deniedResp, err := (&http.Client{Timeout: 5 * time.Second}).Do(denied)
	if err != nil {
		t.Fatal(err)
	}
	_ = deniedResp.Body.Close()
	if deniedResp.StatusCode != http.StatusForbidden {
		t.Fatalf("REST create deny status=%d", deniedResp.StatusCode)
	}

	dialCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(dialCtx, module.GRPCAddress(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	client := deviceopsv1.NewDeviceApplicationClient(conn)
	grpcCtx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+readerToken)
	grpcList, err := client.ListDevices(grpcCtx, &deviceopsv1.ListDevicesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(grpcList.GetDevices()) != 1 || grpcList.GetDevices()[0].GetSerial() != "SN-PARITY-A" {
		t.Fatalf("gRPC reader devices=%v", grpcList.GetDevices())
	}
	_, err = client.CreateDevice(grpcCtx, &deviceopsv1.CreateDeviceRequest{SiteId: site, Name: "Denied", Serial: "SN-GRPC-DENIED"})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("gRPC create deny err=%v code=%s", err, status.Code(err))
	}
}

func TestCrossRoleLegacyScopeCannotEscalateGrant(t *testing.T) {
	db := openDB(t)
	tenant, ownerToken, site := "tenant-scope", "owner-scope-token", "site-scope"
	module := startModule(t, db, tenant, ownerToken, site)
	base := "http://" + module.HTTPAddress()
	postDevice(t, base, ownerToken, site, "Owner", "SN-SCOPE-OWNER")
	postDevice(t, base, ownerToken, site, "Reader", "SN-SCOPE-READER")
	if err := db.Exec("UPDATE biz_deviceops_device SET created_by = ? WHERE tenant_id = ? AND serial = ?", "reader-scope", tenant, "SN-SCOPE-READER").Error; err != nil {
		t.Fatal(err)
	}

	readerToken := "reader-scope-token"
	seedReader(t, db, tenant, "reader-scope", readerToken, "", tenant+":reader", "reader", "device.read", "self")
	seedReader(t, db, tenant, "reader-scope", readerToken, "", tenant+":other", "other", "device.create", "all")

	// Simulate a stale pre-C8.5 scope row on a role that does NOT grant device.read.
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS biz_role_data_scopes (
		tenant_id varchar(64) NOT NULL,
		role_id varchar(160) NOT NULL,
		permission varchar(120) NOT NULL,
		scope varchar(16) NOT NULL,
		PRIMARY KEY (tenant_id, role_id, permission)
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT IGNORE INTO biz_role_data_scopes (tenant_id,role_id,permission,scope) VALUES (?,?,?,?)", tenant, tenant+":other", "device.read", "all").Error; err != nil {
		t.Fatal(err)
	}

	devices := listHTTP(t, base, readerToken)
	if len(devices) != 1 || devices[0]["serial"] != "SN-SCOPE-READER" {
		t.Fatalf("cross-role scope escalation detected: devices=%v", devices)
	}
}
''')

write("docs/waves/C8.5-operation-security-boundary.md", r'''# C8.5 Operation Security Boundary

State: **Complete**

## Responsibility

- Access/IAM is a business domain and owns Tenant/User/Membership/Role/Credential/PermissionGrant persistence.
- Yunka owns the deterministic pre-Application security execution pipeline.
- Permission + scope are one grant fact; scope cannot be sourced from an unrelated role.
- DeviceOps owns the meaning of `all/sites/self` and converts opaque IAM grants into a typed request scope.
- Application code contains no Permission literals, Authorizer calls, IAM lookups or repeated authorization decisions.
- REST and gRPC invoke the same `authz.OperationRuntime` before Application.

## Security boundary

```text
credential -> Principal -> PB policy -> GrantAuthorizer -> Device OperationGuard
           -> AuthorizedOperation + typed DeviceScope -> Application -> Domain/Repository
```

## Verification

The C8.5 builder uses Yunka main `97b15d732269c4ec63f212cbf2deaecc2551e02e`, regenerates domain/contract artifacts, and requires test/race/vet/build, MySQL 8.4 integration, cross-role legacy-scope escalation denial, REST/gRPC parity, regeneration determinism and zero drift before producing the final Git patch.
''')

write("README.md", r'''# biz — Yunka C8.5 reference business application

This repository exercises the current Yunka Operation Security Boundary with a real multi-tenant DeviceOps domain and a separate Access/IAM business domain.

## Authority boundaries

```text
PB DSL -> RPC + REST + DTO + Domain/Application + Operation + Permission
PO     -> persistence schema
Yunka  -> Entity + basic Repository CRUD + Application Port + adapters + policy + security runtime
Access/IAM domain -> Tenant/User/Membership/Role/Credential/PermissionGrant
DeviceOps security -> interpret IAM scope grant as typed DeviceScope
Application -> use cases + DTO/domain mapping + business invariants
```

Permission and data scope are now one IAM `PermissionGrant`. Yunka's `OperationRuntime` resolves authentication/tenant/permission and runs the Device `OperationGuard` before the Application boundary. Application methods do not repeat authorization checks or permission strings.

## Workspace

```text
workspace/
├── yunka.io/
└── biz/
```

## Generate and verify

```bash
cd ../yunka.io
make rpc-tools
cd ../biz
make generate
make check
go test ./...
go test -race ./...
go vet ./...
go build ./...
```

MySQL 8.4 integration:

```bash
YUNKA_TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:3306)/biz?parseTime=true&charset=utf8mb4&loc=UTC' \
  go test -tags=integration ./integration
```

The integration gate proves that stale pre-C8.5 role scope cannot escalate a permission from another role and that REST/gRPC share the same allow/deny security pipeline.
''')
