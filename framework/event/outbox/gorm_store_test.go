package outbox

import (
	"context"
	"errors"
	"testing"

	"gorm.io/gorm"
	"yunka.io/framework/event"
)

func TestGORMStoreRejectsUnsafeTableName(t *testing.T) {
	if _, err := NewGORMStore(&gorm.DB{}, WithTable("outbox;drop table users")); err == nil {
		t.Fatal("unsafe table name accepted")
	}
	if _, err := NewGORMStore(&gorm.DB{}, WithTable("yunka_outbox_v2")); err != nil {
		t.Fatal(err)
	}
}

func TestGORMStoreEnqueueTxRejectsWrongHandle(t *testing.T) {
	store, err := NewGORMStore(&gorm.DB{})
	if err != nil {
		t.Fatal(err)
	}
	envelope := event.Envelope{Topic: "t", Type: "t.v1"}
	if err := store.EnqueueTx(context.Background(), "not-a-gorm-tx", envelope); !errors.Is(err, ErrInvalidTx) {
		t.Fatalf("err=%v", err)
	}
}
