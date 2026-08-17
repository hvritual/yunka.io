package db

import (
	"fmt"
	"github.com/didi/gendry/builder"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"os"
	"path/filepath"
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
	s.Table(ApiModuleButtonTableName).Find(&modBtnUUID, "api_uuid=?", uuid)

	return s.Transaction(func(tx *gorm.DB) error {
		result := tx.Exec(deleteApiSQL, uuid)
		if result.Error != nil {
			return result.Error
		}

		if len(modBtnUUID) == 0 {
			return nil
		}

		result = tx.Where(`module_uuid in ?`, modBtnUUID).Delete(&RoleModuleButton{})
		return result.Error
	})
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

func NewStore(dirName, dbName string) (*Store, error) {
	os.MkdirAll(dirName, 0777)

	db, err := gorm.Open(sqlite.Open(filepath.Join(dirName, dbName)), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	s := &Store{
		DB:      db,
		dirName: dirName,
	}

	return s, s.AutoMigrate(&ApiModuleButton{}, &RoleModuleButton{})
}
