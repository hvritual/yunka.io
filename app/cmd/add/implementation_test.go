package add

import (
	"bytes"
	"github.com/urfave/cli"
	"strings"
	"testing"
)

func TestAG061ImplementationCommandIsRegistered(t *testing.T) {
	var out bytes.Buffer
	app := cli.NewApp()
	app.Writer, app.ErrWriter = &out, &out
	app.Commands = []cli.Command{Command()}
	if err := app.Run([]string{"yunka", "add", "implementation", "--help"}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"composition-package", "apply", "sealed leaf Application"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("registered help misses %q: %s", want, out.String())
		}
	}
	if err := app.Run([]string{"yunka", "add", "implementation"}); err == nil || !strings.Contains(err.Error(), "domain/application") {
		t.Fatalf("invalid invocation: %v", err)
	}
}
