package idempotencygorm

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/hvritual/yunka.io/framework/execution"
)

func TestRecordKeyHashesOpaqueIdempotencyKey(t *testing.T) {
	identity := execution.IdempotencyIdentity{TenantID: "tenant-a", OperationID: "device.create", Key: "secret-request-key", Attempt: "attempt-a"}
	record := recordKey(identity)
	want := sha256.Sum256([]byte(identity.Key))
	if record.TenantID != "tenant-a" || record.OperationID != "device.create" || record.KeyHash != hex.EncodeToString(want[:]) {
		t.Fatalf("record key=%+v", record)
	}
	if record.KeyHash == identity.Key {
		t.Fatal("raw idempotency key leaked into persistent record")
	}
}

func TestNewStoreRejectsNilDatabase(t *testing.T) {
	if _, err := NewStore(nil, Options{}); err == nil {
		t.Fatal("expected nil database rejection")
	}
}
