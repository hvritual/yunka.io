package saga

import (
	"context"
	"errors"

	"yunka.io/framework/event/outbox"
	"yunka.io/framework/execution"
)

type Stager interface {
	Stage(context.Context, Plan) error
	StageCompensations(context.Context, Plan, int) error
}

type stager struct{ store outbox.TransactionalStore }

func NewStager(store outbox.TransactionalStore) (Stager, error) {
	if store == nil {
		return nil, errors.New("saga: transactional outbox store is required")
	}
	return &stager{store: store}, nil
}

func (stager *stager) Stage(ctx context.Context, plan Plan) error {
	if stager == nil || stager.store == nil {
		return errors.New("saga: stager unavailable")
	}
	transaction, err := execution.TransactionHandleFrom(ctx)
	if err != nil {
		return err
	}
	return EnqueueTx(ctx, stager.store, transaction, plan)
}

func (stager *stager) StageCompensations(ctx context.Context, plan Plan, completed int) error {
	if stager == nil || stager.store == nil {
		return errors.New("saga: stager unavailable")
	}
	transaction, err := execution.TransactionHandleFrom(ctx)
	if err != nil {
		return err
	}
	return EnqueueCompensationsTx(ctx, stager.store, transaction, plan, completed)
}
