package contract

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/urfave/cli"
	contractcore "yunka.io/pkg/contract"
)

const AppName = "contract"

func Command() cli.Command {
	return cli.Command{
		Name:  AppName,
		Usage: "compile, generate, inspect, and guard protobuf API contracts",
		Subcommands: []cli.Command{
			lintCommand(),
			generateCommand(),
			checkCommand(),
			diffCommand(),
			inspectCommand(),
		},
	}
}

func sourceFlags() []cli.Flag {
	return []cli.Flag{
		cli.StringFlag{Name: "sources", Usage: "canonical contract source inventory; when set, source sets are compiled independently"},
		cli.StringFlag{Name: "repo-root", Value: ".", Usage: "repository root used to resolve --sources entries"},
		cli.StringFlag{Name: "proto-dir", Value: "./app/cmd/rpc/pb", Usage: "legacy single root containing contract .proto files"},
		cli.StringSliceFlag{Name: "proto-path", Usage: "additional protoc import path for legacy single-root mode; may be repeated"},
		cli.StringSliceFlag{Name: "file", Usage: "specific proto file relative to --proto-dir in legacy single-root mode; may be repeated"},
		cli.StringFlag{Name: "protoc", EnvVar: "PROTOC", Usage: "protoc binary; defaults to PATH"},
	}
}

func artifactFlags() []cli.Flag {
	return []cli.Flag{
		cli.StringFlag{Name: "out", Value: "./contracts/generated", Usage: "generated contract artifact directory"},
		cli.StringFlag{Name: "title", Value: "yunka API", Usage: "OpenAPI document title"},
		cli.StringFlag{Name: "version", Value: "1.0.0", Usage: "OpenAPI document version"},
	}
}

func lintCommand() cli.Command {
	return cli.Command{
		Name:  "lint",
		Usage: "compile protobuf contracts and validate the normalized contract model",
		Flags: sourceFlags(),
		Action: func(c *cli.Context) error {
			result, err := compile(c)
			if err != nil {
				return err
			}
			diagnostics := contractcore.Lint(result.Manifest)
			printDiagnostics(diagnostics)
			if contractcore.HasErrors(diagnostics) {
				return errors.New("contract lint failed")
			}
			fmt.Printf("contract lint ok: files=%d messages=%d enums=%d services=%d descriptor=%s\n",
				len(result.Manifest.Files), len(result.Manifest.Messages), len(result.Manifest.Enums), len(result.Manifest.Services), result.DescriptorSHA)
			return nil
		},
	}
}

func generateCommand() cli.Command {
	flags := append(sourceFlags(), artifactFlags()...)
	return cli.Command{
		Name:  "generate",
		Usage: "generate manifest.json, openapi.json, and client.ts from protobuf contracts",
		Flags: flags,
		Action: func(c *cli.Context) error {
			result, artifacts, err := compileArtifacts(c)
			if err != nil {
				return err
			}
			if err := contractcore.WriteArtifacts(c.String("out"), artifacts); err != nil {
				return err
			}
			fmt.Printf("contract generated: out=%s services=%d messages=%d descriptor=%s\n", c.String("out"), len(result.Manifest.Services), len(result.Manifest.Messages), result.DescriptorSHA)
			return nil
		},
	}
}

func checkCommand() cli.Command {
	flags := append(sourceFlags(), artifactFlags()...)
	flags = append(flags, cli.StringFlag{Name: "baseline", Usage: "optional baseline manifest used for breaking-change guard"})
	return cli.Command{
		Name:  "check",
		Usage: "fail when generated artifacts drift or a baseline comparison contains breaking changes",
		Flags: flags,
		Action: func(c *cli.Context) error {
			result, artifacts, err := compileArtifacts(c)
			if err != nil {
				return err
			}
			drift, err := contractcore.CheckArtifacts(c.String("out"), artifacts)
			if err != nil {
				return err
			}
			for _, item := range drift {
				fmt.Printf("DRIFT %s: %s\n", item.File, item.Reason)
			}
			if len(drift) > 0 {
				return errors.New("contract generated artifacts are stale; run `yunka contract generate`")
			}
			baselinePath := strings.TrimSpace(c.String("baseline"))
			if baselinePath != "" {
				baseline, err := contractcore.LoadManifest(baselinePath)
				if err != nil {
					return err
				}
				diff := contractcore.Compare(baseline, result.Manifest)
				printDiff(diff)
				if diff.HasBreaking() {
					return errors.New("contract guard rejected breaking changes")
				}
			}
			fmt.Printf("contract check ok: out=%s descriptor=%s\n", c.String("out"), result.DescriptorSHA)
			return nil
		},
	}
}

func diffCommand() cli.Command {
	return cli.Command{
		Name:  "diff",
		Usage: "compare two normalized manifest files and classify compatibility changes",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "baseline", Usage: "baseline manifest.json"},
			cli.StringFlag{Name: "current", Usage: "current manifest.json"},
			cli.StringFlag{Name: "format", Value: "text", Usage: "text or json"},
			cli.BoolTFlag{Name: "fail-on-breaking", Usage: "return a failing exit code when breaking changes exist"},
		},
		Action: func(c *cli.Context) error {
			if strings.TrimSpace(c.String("baseline")) == "" || strings.TrimSpace(c.String("current")) == "" {
				return errors.New("--baseline and --current are required")
			}
			baseline, err := contractcore.LoadManifest(c.String("baseline"))
			if err != nil {
				return err
			}
			current, err := contractcore.LoadManifest(c.String("current"))
			if err != nil {
				return err
			}
			diff := contractcore.Compare(baseline, current)
			if strings.EqualFold(c.String("format"), "json") {
				data, _ := json.MarshalIndent(diff, "", "  ")
				fmt.Println(string(data))
			} else {
				printDiff(diff)
			}
			if c.Bool("fail-on-breaking") && diff.HasBreaking() {
				return errors.New("contract guard rejected breaking changes")
			}
			return nil
		},
	}
}

func inspectCommand() cli.Command {
	return cli.Command{
		Name:  "inspect",
		Usage: "print a compact summary of the compiled contract",
		Flags: sourceFlags(),
		Action: func(c *cli.Context) error {
			result, err := compile(c)
			if err != nil {
				return err
			}
			fmt.Printf("schemaVersion: %d\n", result.Manifest.SchemaVersion)
			fmt.Printf("descriptorSHA256: %s\n", result.DescriptorSHA)
			fmt.Printf("files: %d\nmessages: %d\nenums: %d\nservices: %d\n", len(result.Manifest.Files), len(result.Manifest.Messages), len(result.Manifest.Enums), len(result.Manifest.Services))
			for _, service := range result.Manifest.Services {
				fmt.Printf("- %s (%d methods)\n", service.FullName, len(service.Methods))
				for _, method := range service.Methods {
					fmt.Printf("  - %s: %s -> %s", method.Name, method.Request, method.Response)
					if len(method.HTTP) > 0 {
						fmt.Printf(" [%s %s]", method.HTTP[0].Method, method.HTTP[0].Path)
					}
					fmt.Println()
				}
			}
			return nil
		},
	}
}

func compile(c *cli.Context) (contractcore.CompileResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if inventory := strings.TrimSpace(c.String("sources")); inventory != "" {
		return contractcore.CompileInventory(ctx, contractcore.InventoryCompileOptions{
			RepositoryRoot: c.String("repo-root"),
			InventoryPath:  inventory,
			Protoc:         c.String("protoc"),
		})
	}
	return contractcore.Compile(ctx, contractcore.CompileOptions{
		Dir:        c.String("proto-dir"),
		ProtoPaths: c.StringSlice("proto-path"),
		Files:      c.StringSlice("file"),
		Protoc:     c.String("protoc"),
	})
}

func compileArtifacts(c *cli.Context) (contractcore.CompileResult, contractcore.Artifacts, error) {
	result, err := compile(c)
	if err != nil {
		return contractcore.CompileResult{}, contractcore.Artifacts{}, err
	}
	diagnostics := contractcore.Lint(result.Manifest)
	printDiagnostics(diagnostics)
	if contractcore.HasErrors(diagnostics) {
		return contractcore.CompileResult{}, contractcore.Artifacts{}, errors.New("contract lint failed")
	}
	artifacts, err := contractcore.RenderArtifacts(result.Manifest, contractcore.ArtifactOptions{
		OpenAPI: contractcore.OpenAPIOptions{Title: c.String("title"), Version: c.String("version")},
	})
	return result, artifacts, err
}

func printDiagnostics(diagnostics []contractcore.Diagnostic) {
	for _, diagnostic := range diagnostics {
		fmt.Printf("%s %s: %s\n", strings.ToUpper(string(diagnostic.Severity)), diagnostic.Path, diagnostic.Message)
	}
}

func printDiff(diff contractcore.Diff) {
	if len(diff.Changes) == 0 {
		fmt.Println("contract diff: no changes")
		return
	}
	for _, change := range diff.Changes {
		fmt.Printf("%s %s %s: %s\n", strings.ToUpper(string(change.Severity)), change.Kind, change.Path, change.Message)
	}
}
