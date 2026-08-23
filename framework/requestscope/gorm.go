package requestscope

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"

	"gorm.io/gorm"
)

// GORMUnitOfWork is the typed repository seam for GORM-backed request scopes.
type GORMUnitOfWork interface {
	UnitOfWork
	GORM() *gorm.DB
}

// GORMFactory begins one transaction from an App-owned GORM connection pool.
type GORMFactory struct {
	database *gorm.DB
	options  *sql.TxOptions
}

func NewGORMFactory(database *gorm.DB, options *sql.TxOptions) (*GORMFactory, error) {
	if database == nil {
		return nil, errors.New("requestscope: GORM database is required")
	}
	return &GORMFactory{database: database, options: options}, nil
}

func (factory *GORMFactory) Begin(ctx context.Context) (UnitOfWork, error) {
	if factory == nil || factory.database == nil {
		return nil, ErrFactoryUnavailable
	}
	ctx = normalizeContext(ctx)
	database := factory.database.WithContext(ctx)
	var transaction *gorm.DB
	if factory.options == nil {
		transaction = database.Begin()
	} else {
		transaction = database.Begin(factory.options)
	}
	if transaction.Error != nil {
		return nil, transaction.Error
	}
	return &gormUnitOfWork{transaction: transaction}, nil
}

type gormUnitOfWork struct {
	mu          sync.Mutex
	transaction *gorm.DB
	finished    bool
	finishErr   error
}

func (unit *gormUnitOfWork) GORM() *gorm.DB {
	if unit == nil {
		return nil
	}
	unit.mu.Lock()
	defer unit.mu.Unlock()
	return unit.transaction
}

func (unit *gormUnitOfWork) Commit(ctx context.Context) error {
	if unit == nil || unit.transaction == nil {
		return errors.New("requestscope: GORM unit of work is unavailable")
	}
	unit.mu.Lock()
	defer unit.mu.Unlock()
	if unit.finished {
		return unit.finishErr
	}
	result := unit.transaction.WithContext(normalizeContext(ctx)).Commit()
	unit.finished = true
	unit.finishErr = result.Error
	return unit.finishErr
}

func (unit *gormUnitOfWork) Rollback(ctx context.Context) error {
	if unit == nil || unit.transaction == nil {
		return errors.New("requestscope: GORM unit of work is unavailable")
	}
	unit.mu.Lock()
	defer unit.mu.Unlock()
	if unit.finished {
		return unit.finishErr
	}
	result := unit.transaction.WithContext(normalizeContext(ctx)).Rollback()
	unit.finished = true
	unit.finishErr = result.Error
	return unit.finishErr
}

func (unit *gormUnitOfWork) Close() error {
	if unit == nil || unit.transaction == nil {
		return nil
	}
	unit.mu.Lock()
	defer unit.mu.Unlock()
	if unit.finished {
		return nil
	}
	result := unit.transaction.Rollback()
	unit.finished = true
	unit.finishErr = result.Error
	return unit.finishErr
}

// GORMFrom returns the request transaction from a generic UnitOfWork.
func GORMFrom(unit UnitOfWork) (*gorm.DB, error) {
	gormUnit, ok := unit.(GORMUnitOfWork)
	if !ok || gormUnit == nil || gormUnit.GORM() == nil {
		return nil, fmt.Errorf("requestscope: unit of work %T is not GORM-backed", unit)
	}
	return gormUnit.GORM(), nil
}

// GORMRepositories adapts a typed GORM repository constructor to a generic
// RepositoryFactory.
func GORMRepositories[R any](build func(context.Context, *gorm.DB) (R, error)) RepositoryFactory[R] {
	return func(ctx context.Context, unit UnitOfWork) (R, error) {
		var zero R
		if build == nil {
			return zero, errors.New("requestscope: GORM repository builder is required")
		}
		database, err := GORMFrom(unit)
		if err != nil {
			return zero, err
		}
		return build(normalizeContext(ctx), database)
	}
}
