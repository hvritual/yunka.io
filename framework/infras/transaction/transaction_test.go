package transaction

import (
	"errors"
	"testing"
)

type fakeTransaction struct {
	beginErr, rollbackErr, commitErr error
	begins, rollbacks, commits       int
}

func (t *fakeTransaction) Begin(interface{}) error { t.begins++; return t.beginErr }
func (t *fakeTransaction) Rollback() error         { t.rollbacks++; return t.rollbackErr }
func (t *fakeTransaction) Commit() error           { t.commits++; return t.commitErr }

func TestOpenTransactionRollsBackStartedTransactions(t *testing.T) {
	first := &fakeTransaction{}
	second := &fakeTransaction{beginErr: errors.New("begin failed")}
	if err := OpenTransaction(nil, func() error { return nil }, first, second); err == nil {
		t.Fatal("begin error was lost")
	}
	if first.rollbacks != 1 || first.commits != 0 {
		t.Fatalf("first transaction: rollbacks=%d commits=%d", first.rollbacks, first.commits)
	}
	if second.rollbacks != 0 {
		t.Fatalf("transaction that failed to begin was rolled back %d times", second.rollbacks)
	}
}

func TestOpenTransactionReturnsCommitError(t *testing.T) {
	tx := &fakeTransaction{commitErr: errors.New("commit failed")}
	if err := OpenTransaction(nil, func() error { return nil }, tx); err == nil {
		t.Fatal("commit error was lost")
	}
}

func TestOpenTransactionRollsBackPanic(t *testing.T) {
	tx := &fakeTransaction{}
	err := OpenTransaction(nil, func() error { panic("boom") }, tx)
	if err == nil || tx.rollbacks != 1 {
		t.Fatalf("panic result err=%v rollbacks=%d", err, tx.rollbacks)
	}
}
