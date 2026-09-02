package requestscope

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"

	"gorm.io/gorm"
	"github.com/hvritual/yunka.io/framework/execution"
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
	unit.finishErr = result.Error
	if result.Error == nil {
		unit.finished = true
	}
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

// GORMExecutionFactory is the C9.7 root transaction factory used by the
// Operation Executor. Legacy GORMFactory remains available for pre-C9.7 callers.
type GORMExecutionFactory struct{ database *gorm.DB }

func NewGORMExecutionFactory(database *gorm.DB) (*GORMExecutionFactory, error) {
	if database == nil {
		return nil, errors.New("requestscope: GORM database is required")
	}
	return &GORMExecutionFactory{database: database}, nil
}

func (factory *GORMExecutionFactory) Begin(ctx context.Context, mode execution.TransactionMode) (execution.UnitOfWork, error) {
	if factory == nil || factory.database == nil {
		return nil, ErrFactoryUnavailable
	}
	if mode != execution.TransactionReadOnly && mode != execution.TransactionLocal {
		return nil, fmt.Errorf("requestscope: unsupported execution transaction mode %q", mode)
	}
	options := &sql.TxOptions{ReadOnly: mode == execution.TransactionReadOnly}
	transaction := factory.database.WithContext(normalizeContext(ctx)).Begin(options)
	if transaction.Error != nil {
		return nil, transaction.Error
	}
	return &gormUnitOfWork{transaction: transaction}, nil
}

// TransactionHandle lets framework mechanisms such as Saga/Outbox join the
// exact root transaction without exposing *gorm.DB through Application APIs.
func (unit *gormUnitOfWork) TransactionHandle() any {
	return unit.GORM()
}
