package db

import "testing"

func TestVerifyRoleAPIRightAndDeleteAPI(t *testing.T) {
	store, err := NewStore(t.TempDir(), "gateway.db")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.BatchCreate([]ApiModuleButton{{ApiUUID: "api-1", ModuleButtonUUID: "button-1"}}); err != nil {
		t.Fatal(err)
	}
	if err := store.OperateRole("org-1", "role-1", []string{"button-1"}, nil); err != nil {
		t.Fatal(err)
	}
	if ok, err := store.VerifyRoleAPIRight("api-1", "org-1", []string{"role-1"}); err != nil || !ok {
		t.Fatalf("authorized role rejected: ok=%v err=%v", ok, err)
	}
	if ok, err := store.VerifyRoleAPIRight("api-1", "org-2", []string{"role-1"}); err != nil || ok {
		t.Fatalf("cross-organization role accepted: ok=%v err=%v", ok, err)
	}
	if err := store.DeleteApi("api-1"); err != nil {
		t.Fatal(err)
	}
	if ok, err := store.VerifyRoleAPIRight("api-1", "org-1", []string{"role-1"}); err != nil || ok {
		t.Fatalf("deleted API right remained: ok=%v err=%v", ok, err)
	}
}
