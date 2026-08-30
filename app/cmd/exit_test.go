package main

import (
	"errors"
	"testing"

	"github.com/urfave/cli"
)

func TestClassifyCLIExitPreservesEmptyStructuredMessage(t *testing.T) {
	message, code, ok := classifyCLIExit(cli.NewExitError("", 1))
	if !ok {
		t.Fatal("expected cli exit coder")
	}
	if message != "" || code != 1 {
		t.Fatalf("message=%q code=%d", message, code)
	}
}

func TestClassifyCLIExitLeavesOrdinaryErrorsOnLegacyPath(t *testing.T) {
	if _, _, ok := classifyCLIExit(errors.New("legacy failure")); ok {
		t.Fatal("ordinary error must not be classified as structured cli exit")
	}
}
