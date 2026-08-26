package operations

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	devicedomain "github.com/hvritual/biz/internal/device/domain"
	devicepersistence "github.com/hvritual/biz/internal/device/infrastructure/persistence"
	"github.com/hvritual/biz/internal/iam/access"
	sitepersistence "github.com/hvritual/biz/internal/site/infrastructure/persistence"
	"gorm.io/gorm"
	"yunka.io/framework/requestscope"
)

var (
	ErrNotFound = errors.New("device: not found")
	ErrConflict = errors.New("device: conflict")
	ErrInvalid  = errors.New("device: invalid request")
)

type Repositories struct {
	Devices *Repository
}

type Repository struct {
	database *gorm.DB
}

func newRepositories(database *gorm.DB) (Repositories, error) {
	if database == nil {
		return Repositories{}, errors.New("device: repository database is required")
	}
	return Repositories{Devices: &Repository{database: database}}, nil
}

func (repository *Repository) applyScope(plan access.Plan, permission string) *gorm.DB {
	query := repository.database.Model(&devicepersistence.DevicePORecord{}).
		Where("tenant_id = ?", plan.Principal.TenantID)
	scope, ok := plan.Permissions[permission]
	if !ok || !scope.Allowed {
		return query.Where("1 = 0")
	}
	if scope.All {
		return query
	}
	if scope.Sites && scope.Self {
		if len(plan.SiteIDs) == 0 {
			return query.Where("created_by = ?", plan.Principal.UserID)
		}
		return query.Where("(site_id IN ? OR created_by = ?)", plan.SiteIDs, plan.Principal.UserID)
	}
	if scope.Sites {
		if len(plan.SiteIDs) == 0 {
			return query.Where("1 = 0")
		}
		return query.Where("site_id IN ?", plan.SiteIDs)
	}
	if scope.Self {
		return query.Where("created_by = ?", plan.Principal.UserID)
	}
	return query.Where("1 = 0")
}

func (repository *Repository) List(plan access.Plan) ([]devicedomain.Device, error) {
	var rows []devicepersistence.DevicePORecord
	if err := repository.applyScope(plan, access.PermissionDeviceRead).
		Order("created_at DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]devicedomain.Device, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.Domain())
	}
	return items, nil
}

func (repository *Repository) Find(plan access.Plan, permission, id string) (devicedomain.Device, error) {
	var row devicepersistence.DevicePORecord
	err := repository.applyScope(plan, permission).Where("id = ?", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return devicedomain.Device{}, ErrNotFound
	}
	if err != nil {
		return devicedomain.Device{}, err
	}
	return row.Domain(), nil
}

func (repository *Repository) SiteExists(tenantID, siteID string) (bool, error) {
	var count int64
	if err := repository.database.Model(&sitepersistence.SitePORecord{}).
		Where("tenant_id = ? AND id = ?", tenantID, siteID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count == 1, nil
}

func (repository *Repository) Create(value *devicedomain.Device) error {
	if value == nil {
		return ErrInvalid
	}
	record := devicepersistence.DevicePORecord{
		DevicePO: devicepersistence.DevicePO{
			SiteID:    value.SiteID,
			Name:      value.Name,
			Serial:    value.Serial,
			CreatedBy: value.CreatedBy,
		},
		DevicePOBase: devicepersistence.DevicePOBase{
			ID:        value.ID,
			TenantID:  value.TenantID,
			Version:   value.Version,
			CreatedAt: value.CreatedAt,
			UpdatedAt: value.UpdatedAt,
		},
	}
	if err := repository.database.Create(&record).Error; err != nil {
		return err
	}
	*value = record.Domain()
	return nil
}

func (repository *Repository) Update(plan access.Plan, current devicedomain.Device, name, siteID string, expectedVersion uint64) (devicedomain.Device, error) {
	updates := map[string]any{
		"name":       name,
		"site_id":    siteID,
		"version":    expectedVersion + 1,
		"updated_at": time.Now().UTC(),
	}
	result := repository.applyScope(plan, access.PermissionDeviceUpdate).
		Where("id = ? AND version = ?", current.ID, expectedVersion).
		Updates(updates)
	if result.Error != nil {
		return devicedomain.Device{}, result.Error
	}
	if result.RowsAffected != 1 {
		return devicedomain.Device{}, ErrConflict
	}
	return repository.Find(plan, access.PermissionDeviceUpdate, current.ID)
}

func (repository *Repository) Delete(plan access.Plan, current devicedomain.Device, expectedVersion uint64) error {
	result := repository.applyScope(plan, access.PermissionDeviceDelete).
		Where("id = ? AND version = ?", current.ID, expectedVersion).
		Delete(&devicepersistence.DevicePORecord{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrConflict
	}
	return nil
}

type Service struct {
	scopes *requestscope.Factory[Repositories]
}

func NewService(database *gorm.DB) (*Service, error) {
	unitOfWork, err := requestscope.NewGORMFactory(database, nil)
	if err != nil {
		return nil, err
	}
	factory, err := requestscope.NewFactory(requestscope.FactoryOptions[Repositories]{
		UnitOfWork: unitOfWork,
		Repositories: requestscope.GORMRepositories(func(ctx context.Context, transaction *gorm.DB) (Repositories, error) {
			return newRepositories(transaction.WithContext(ctx))
		}),
	})
	if err != nil {
		return nil, err
	}
	return &Service{scopes: factory}, nil
}

func (service *Service) ListDevices(ctx context.Context, plan access.Plan) ([]devicedomain.Device, error) {
	if !plan.Can(access.PermissionDeviceRead) {
		return nil, access.ErrForbidden
	}
	return requestscope.ExecuteValue(ctx, service.scopes, func(scope *requestscope.Scope[Repositories]) ([]devicedomain.Device, error) {
		return scope.Repositories().Devices.List(plan)
	})
}

type CreateDeviceInput struct {
	SiteID string `json:"siteId"`
	Name   string `json:"name"`
	Serial string `json:"serial"`
}

func (service *Service) CreateDevice(ctx context.Context, plan access.Plan, input CreateDeviceInput) (devicedomain.Device, error) {
	input.SiteID = strings.TrimSpace(input.SiteID)
	input.Name = strings.TrimSpace(input.Name)
	input.Serial = strings.TrimSpace(input.Serial)
	if input.SiteID == "" || input.Name == "" || input.Serial == "" {
		return devicedomain.Device{}, ErrInvalid
	}
	if !plan.Can(access.PermissionDeviceCreate) || !plan.CanTargetSite(access.PermissionDeviceCreate, input.SiteID) {
		return devicedomain.Device{}, access.ErrForbidden
	}
	return requestscope.ExecuteValue(ctx, service.scopes, func(scope *requestscope.Scope[Repositories]) (devicedomain.Device, error) {
		repository := scope.Repositories().Devices
		exists, err := repository.SiteExists(plan.Principal.TenantID, input.SiteID)
		if err != nil {
			return devicedomain.Device{}, err
		}
		if !exists {
			return devicedomain.Device{}, ErrInvalid
		}
		now := time.Now().UTC()
		device := devicedomain.Device{
			ID:        newID(),
			TenantID:  plan.Principal.TenantID,
			SiteID:    input.SiteID,
			Name:      input.Name,
			Serial:    input.Serial,
			CreatedBy: plan.Principal.UserID,
			Version:   1,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := repository.Create(&device); err != nil {
			return devicedomain.Device{}, err
		}
		return device, nil
	})
}

func (service *Service) GetDevice(ctx context.Context, plan access.Plan, id string) (devicedomain.Device, error) {
	if !plan.Can(access.PermissionDeviceRead) {
		return devicedomain.Device{}, access.ErrForbidden
	}
	return requestscope.ExecuteValue(ctx, service.scopes, func(scope *requestscope.Scope[Repositories]) (devicedomain.Device, error) {
		return scope.Repositories().Devices.Find(plan, access.PermissionDeviceRead, strings.TrimSpace(id))
	})
}

type UpdateDeviceInput struct {
	Name    string `json:"name"`
	SiteID  string `json:"siteId"`
	Version uint64 `json:"version"`
}

func (service *Service) UpdateDevice(ctx context.Context, plan access.Plan, id string, input UpdateDeviceInput) (devicedomain.Device, error) {
	if !plan.Can(access.PermissionDeviceUpdate) {
		return devicedomain.Device{}, access.ErrForbidden
	}
	if input.Version == 0 {
		return devicedomain.Device{}, ErrInvalid
	}
	return requestscope.ExecuteValue(ctx, service.scopes, func(scope *requestscope.Scope[Repositories]) (devicedomain.Device, error) {
		repository := scope.Repositories().Devices
		current, err := repository.Find(plan, access.PermissionDeviceUpdate, strings.TrimSpace(id))
		if err != nil {
			return devicedomain.Device{}, err
		}
		if current.Version != input.Version {
			return devicedomain.Device{}, ErrConflict
		}
		name := strings.TrimSpace(input.Name)
		if name == "" {
			name = current.Name
		}
		siteID := strings.TrimSpace(input.SiteID)
		if siteID == "" {
			siteID = current.SiteID
		}
		if siteID != current.SiteID {
			if !plan.CanTargetSite(access.PermissionDeviceUpdate, siteID) {
				return devicedomain.Device{}, access.ErrForbidden
			}
			exists, err := repository.SiteExists(plan.Principal.TenantID, siteID)
			if err != nil {
				return devicedomain.Device{}, err
			}
			if !exists {
				return devicedomain.Device{}, ErrInvalid
			}
		}
		return repository.Update(plan, current, name, siteID, input.Version)
	})
}

func (service *Service) DeleteDevice(ctx context.Context, plan access.Plan, id string, version uint64) error {
	if !plan.Can(access.PermissionDeviceDelete) {
		return access.ErrForbidden
	}
	if version == 0 {
		return ErrInvalid
	}
	return requestscope.Execute(ctx, service.scopes, func(scope *requestscope.Scope[Repositories]) error {
		repository := scope.Repositories().Devices
		current, err := repository.Find(plan, access.PermissionDeviceDelete, strings.TrimSpace(id))
		if err != nil {
			return err
		}
		if current.Version != version {
			return ErrConflict
		}
		return repository.Delete(plan, current, version)
	})
}

func newID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(value[:])
}
