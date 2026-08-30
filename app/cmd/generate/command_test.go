package generate

import "testing"

func TestCommandIsTopLevelGenerate(t *testing.T) {
	command := Command()
	if command.Name != "generate" {
		t.Fatalf("Name=%q", command.Name)
	}
	if command.Action == nil {
		t.Fatal("generate action is nil")
	}
}
