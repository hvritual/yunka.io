package orm

/**
 * @BelongProject yunka
 * @BelongPackage infra
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/12/9 9:21 下午
 * @Version V1.0
 */

import (
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"log"
	"os"
	"time"
	"yunka.io/framework/core"
	"yunka.io/framework/core/request"
	"yunka.io/framework/infras/transaction"
)

func NewOrm(dsn string) (*ORM, error) {
	if dsn == `` {
		panic(dsn)
	}
	db, err := gorm.Open(mysql.New(mysql.Config{
		DSN:                       dsn,   // DSN data source name
		DefaultStringSize:         256,   // string 类型字段的默认长度
		DisableDatetimePrecision:  true,  // 禁用 datetime 精度，MySQL 5.6 之前的数据库不支持
		DontSupportRenameIndex:    true,  // 重命名索引时采用删除并新建的方式，MySQL 5.7 之前的数据库和 MariaDB 不支持重命名索引
		DontSupportRenameColumn:   true,  // 用 `change` 重命名列，MySQL 8 之前的数据库和 MariaDB 不支持重命名列
		SkipInitializeWithVersion: false, // 根据当前 MySQL 版本自动配置
	}), &gorm.Config{
		PrepareStmt: true,
		Logger: logger.New(log.New(os.Stdout, "[yunka] ", log.LstdFlags), logger.Config{
			SlowThreshold: 200 * time.Millisecond,
			LogLevel:      logger.Error,
			//LogLevel: logger.Info,
			Colorful: false,
		}),
	})
	return &ORM{
		DB: db,
	}, err
}

var (
	_ core.RuntimeInit        = (*ORM)(nil)
	_ transaction.Transaction = (*ORM)(nil)
)

type ORM struct {
	begin bool
	*gorm.DB
	beforeTxDb *gorm.DB
	request.Runtime
}

func (orm *ORM) Begin(i interface{}) error {

	if orm.begin {
		return nil
	}

	db := orm.DB
	result := orm.DB.Begin()
	if result.Error == nil {
		orm.beforeTxDb = db
		orm.DB = result
		orm.begin = true
	} else {
		if orm.Runtime != nil {
			orm.Runtime.Logger().Error("Begin, err:", result.Error)
		} else {
			core.Log().Debug("Begin, err:", result.Error)
		}
	}

	return result.Error
}

func (orm *ORM) Rollback() error {
	if orm.begin {
		result := orm.DB.Rollback()
		orm.Reset()
		return result.Error
	}
	return nil

}

func (orm *ORM) Commit() error {
	if orm.begin {
		result := orm.DB.Commit()
		orm.Reset()
		return result.Error
	}
	return nil
}

func (orm *ORM) Reset() {
	orm.begin = false
	if orm.beforeTxDb != nil {
		orm.DB = orm.beforeTxDb
		orm.beforeTxDb = nil
	}
}

func (orm *ORM) Init(rt request.Runtime) error {
	orm.Runtime = rt
	orm.Reset()

	if rt != nil {
		rt.TransactionPrepare(orm)
		rt.BindFinishHook(orm.requestFinish)
	}

	return nil
}

func (orm *ORM) requestFinish(err error) error {
	if orm.begin {
		if err != nil {
			return orm.Rollback()
		} else {
			return orm.Commit()
		}
	}
	return nil
}
