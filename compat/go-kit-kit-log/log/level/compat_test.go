package level

import (
	"bytes"
	"strings"
	"testing"

	compatlog "github.com/go-kit/kit/log"
)

func TestFilterCompatibilitySurface(t *testing.T) {
	var output bytes.Buffer
	logger := NewFilter(compatlog.NewLogfmtLogger(&output), AllowInfo())
	if err := Debug(logger).Log("message", "hidden"); err != nil {
		t.Fatal(err)
	}
	if err := Info(logger).Log("message", "visible"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "hidden") || !strings.Contains(output.String(), "visible") {
		t.Fatalf("output=%q", output.String())
	}
}
