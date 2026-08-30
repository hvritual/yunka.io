package check

import "testing"

func TestCommandIsTopLevelCheck(t *testing.T) {
	command := Command()
	if command.Name != "check" {
		t.Fatalf("Name=%q", command.Name)
	}
	if command.Action == nil {
		t.Fatal("check action is nil")
	}
}
