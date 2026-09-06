package change

import (
	"strings"

	"github.com/urfave/cli"
	"yunka.io/app/cmd/projectflow"
)

// Preserve blank occurrences for validation and literal commas in paths.
type sourceIncludeValues []string

func (v *sourceIncludeValues) Set(value string) error { *v = append(*v, value); return nil }
func (v *sourceIncludeValues) String() string         { return strings.Join(*v, ", ") }
func sourceProtocFlag() cli.Flag {
	return cli.StringFlag{Name: "protoc", EnvVar: "PROTOC", Usage: "protoc binary for canonical source scope; defaults to PATH"}
}
func sourceIncludesFlag() cli.Flag {
	return cli.GenericFlag{Name: "proto-path", Value: &sourceIncludeValues{}, Usage: "additional project-relative protobuf include path; repeatable for proto-root projects"}
}
func sourceCompilerOptions(c *cli.Context) projectflow.Options {
	options := projectflow.Options{Root: c.String("root"), Protoc: c.String("protoc")}
	if paths, ok := c.Generic("proto-path").(*sourceIncludeValues); ok && paths != nil {
		options.ProtoPaths = append([]string(nil), (*paths)...)
	}
	return options
}
