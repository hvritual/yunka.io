package log

import (
	"bytes"
	"strings"
	"testing"
)

func TestAliyunLoggerCompatibilitySurface(t *testing.T) {
	var output bytes.Buffer
	var logger Logger = NewLogfmtLogger(NewSyncWriter(&output))
	logger = With(logger, "time", DefaultTimestampUTC, "caller", DefaultCaller)
	if err := logger.Log("message", "ready"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "message=ready") {
		t.Fatalf("output=%q", output.String())
	}
}
