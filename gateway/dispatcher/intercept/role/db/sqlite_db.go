package db

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/didi/gendry/builder"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Store struct {
	*gorm.DB
	dirName string
}

var (
	deleteApiSQL = fmt.Sprintf("DELETE FROM %s WHERE `api_uuid`=?", ApiModuleButtonTableName)
)

func (s *Store) DeleteApi(uuid string) error {

	modBtnUUID := []string(nil)
	if err := s.Table(ApiModuleButtonTableName).
		Where("api_uuid = ?", uuid).
		Pluck("module_button_uuid", &modBtnUUID).Error; err != nil {
		return err
	}

	return s.Transaction(func(tx *gorm.DB) error {
		result := tx.Exec(deleteApiSQL, uuid)
		if result.Error != nil {
			return result.Error
		}

		if len(modBtnUUID) == 0 {
			return nil
		}

		result = tx.Where(`module_button_uuid in ?`, modBtnUUID).Delete(&RoleModuleButton{})
		return result.Error
	})
}

func (s *Store) VerifyRoleAPIRight(apiUUID, orgUUID string, roleUUID []string) (bool, error) {
	if apiUUID == "" || orgUUID == "" || len(roleUUID) == 0 {
		return false, nil
	}

	var count int64
	err := s.Table(RoleModuleButtonTableName+" AS role_button").
		Joins("JOIN "+ApiModuleButtonTableName+" AS api_button ON api_button.module_button_uuid = role_button.module_button_uuid").
		Where("api_button.api_uuid = ? AND role_button.org_uuid = ? AND role_button.role_uuid IN ?", apiUUID, orgUUID, roleUUID).
		Count(&count).Error
	return count > 0, err
}

func (s *Store) BatchCreate(btns []ApiModuleButton) error {
	if len(btns) == 0 {
		return nil
	}
	result := s.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(btns)
	return result.Error
}

func (s *Store) OperateRole(orgUUID string, roleUUID string, modBtnUUID []string, delModBtnUUID []string) error {
	roleBtns := make([]RoleModuleButton, len(modBtnUUID))
	for idx, modBtn := range modBtnUUID {
		roleBtns[idx] = RoleModuleButton{
			OrgUUID:          orgUUID,
			RoleUUID:         roleUUID,
			ModuleButtonUUID: modBtn,
		}
	}
	var (
		sql  string
		cond []interface{}
		err  error
	)
	if len(delModBtnUUID) != 0 {
		sql, cond, err = builder.BuildDelete(RoleModuleButtonTableName, map[string]interface{}{
			`org_uuid`:              orgUUID,
			`role_uuid`:             roleUUID,
			`module_button_uuid in`: delModBtnUUID,
		})
		if err != nil {
			return err
		}
	}

	return s.Transaction(func(tx *gorm.DB) error {
		if len(modBtnUUID) != 0 {
			result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(roleBtns)
			if result.Error != nil {
				return result.Error
			}
		}

		if sql != `` {
			result := tx.Exec(sql, cond...)
			if result.Error != nil {
				return result.Error
			}
		}
		return nil
	})
}

// NewStoreFromDB binds the role repository to an existing GORM handle. In the
// typed path this handle is the request-owned transaction supplied by
// requestscope; the Store never owns or closes the App-level connection pool.
func NewStoreFromDB(database *gorm.DB) (*Store, error) {
	if database == nil {
		return nil, errors.New("role db: GORM database is required")
	}
	return &Store{DB: database}, nil
}

func Migrate(database *gorm.DB) error {
	if database == nil {
		return errors.New("role db: GORM database is required")
	}
	return database.AutoMigrate(&ApiModuleButton{}, &RoleModuleButton{})
}

func NewStore(dirName, dbName string) (*Store, error) {
	if err := os.MkdirAll(dirName, 0750); err != nil {
		return nil, err
	}

	database, err := gorm.Open(sqlite.Open(filepath.Join(dirName, dbName)), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	store, err := NewStoreFromDB(database)
	if err != nil {
		return nil, err
	}
	store.dirName = dirName
	return store, Migrate(database)
}
