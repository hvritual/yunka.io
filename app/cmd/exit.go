package main

import "github.com/urfave/cli"

func classifyCLIExit(err error) (message string, code int, ok bool) {
	if err == nil {
		return "", 0, false
	}
	exitCoder, ok := err.(cli.ExitCoder)
	if !ok {
		return "", 0, false
	}
	return exitCoder.Error(), exitCoder.ExitCode(), true
}
