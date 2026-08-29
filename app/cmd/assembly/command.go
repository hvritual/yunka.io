package assembly

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/urfave/cli"
	"yunka.io/pkg/assemblyplan"
	contractcore "yunka.io/pkg/contract"
)

const AppName = "assembly"

func Command() cli.Command {
	return cli.Command{
		Name:  AppName,
		Usage: "compile deterministic Runtime Assembly artifacts from canonical contract and generated module facts",
		Subcommands: []cli.Command{
			generateCommand(),
			checkCommand(),
			inspectCommand(),
		},
	}
}

func sourceFlags() []cli.Flag {
	return []cli.Flag{
		cli.StringFlag{Name: "sources", Usage: "canonical contract source inventory; when set, source sets are compiled independently"},
		cli.StringFlag{Name: "repo-root", Value: ".", Usage: "repository root used to resolve --sources entries"},
		cli.StringFlag{Name: "proto-dir", Value: "./contracts/proto", Usage: "canonical contract proto root"},
		cli.StringSliceFlag{Name: "proto-path", Usage: "additional protoc import path; may be repeated"},
		cli.StringSliceFlag{Name: "file", Usage: "specific proto file relative to --proto-dir; may be repeated"},
		cli.StringFlag{Name: "protoc", EnvVar: "PROTOC", Usage: "protoc binary; defaults to PATH"},
	}
}

func assemblyFlags() []cli.Flag {
	flags := append([]cli.Flag{}, sourceFlags()...)
	return append(flags,
		cli.StringFlag{Name: "module-root", Value: "./modules", Usage: "explicit root containing generated Yunka modules"},
		cli.StringFlag{Name: "out", Value: "./contracts/generated", Usage: "generated contract artifact directory for assembly-plan.json"},
		cli.StringFlag{Name: "code-out", Usage: "root directory for generated typed assembly Go"},
		cli.StringFlag{Name: "code-import", Usage: "Go import path corresponding to generated application/code root"},
	)
}

func generateCommand() cli.Command {
	return cli.Command{
		Name:  "generate",
		Usage: "generate assembly-plan.json and typed Application/module composition",
		Flags: assemblyFlags(),
		Action: func(c *cli.Context) error {
			compilation, bindings, err := compileAssembly(c)
			if err != nil {
				return err
			}
			if err := contractcore.WriteAssemblyCompilation(c.String("out"), c.String("code-out"), compilation); err != nil {
				return err
			}
			summary, _ := assemblyplan.Inspect(compilation.Plan)
			fmt.Printf("assembly generated: applications=%d modules=%d externalOperations=%d internalOperations=%d bindings=%d out=%s codeOut=%s\n",
				len(summary.Applications), len(summary.Modules), len(summary.ExternalOperations), len(summary.InternalOperations), len(bindings), c.String("out"), c.String("code-out"))
			return nil
		},
	}
}

func checkCommand() cli.Command {
	return cli.Command{
		Name:  "check",
		Usage: "fail when AssemblyPlan or typed assembly artifacts drift from canonical facts",
		Flags: assemblyFlags(),
		Action: func(c *cli.Context) error {
			compilation, _, err := compileAssembly(c)
			if err != nil {
				return err
			}
			drift, err := contractcore.CheckAssemblyCompilation(c.String("out"), c.String("code-out"), compilation)
			if err != nil {
				return err
			}
			for _, item := range drift {
				fmt.Printf("DRIFT %s: %s\n", item.File, item.Reason)
			}
			if len(drift) > 0 {
				return errors.New("assembly generated artifacts are stale; run `yunka assembly generate`")
			}
			fmt.Println("assembly check ok")
			return nil
		},
	}
}

func inspectCommand() cli.Command {
	flags := append([]cli.Flag{}, sourceFlags()...)
	flags = append(flags,
		cli.StringFlag{Name: "module-root", Value: "./modules", Usage: "explicit root containing generated Yunka modules"},
		cli.StringFlag{Name: "code-import", Usage: "Go import path corresponding to generated application/code root"},
	)
	return cli.Command{
		Name:  "inspect",
		Usage: "print the deterministic Assembly compiler summary without writing files",
		Flags: flags,
		Action: func(c *cli.Context) error {
			compilation, bindings, err := compileAssemblyWithOutputs(c, "./contracts/generated", ".")
			if err != nil {
				return err
			}
			summary, err := assemblyplan.Inspect(compilation.Plan)
			if err != nil {
				return err
			}
			digest, err := assemblyplan.Digest(compilation.Plan)
			if err != nil {
				return err
			}
			fmt.Printf("identity: %s\ndigest: %s\napplications: %d\nmodules: %d\nexternalOperations: %d\ninternalOperations: %d\nmoduleBindings: %d\n",
				summary.Identity, digest, len(summary.Applications), len(summary.Modules), len(summary.ExternalOperations), len(summary.InternalOperations), len(bindings))
			for _, binding := range bindings {
				fmt.Printf("- module %s: %s.%s [%s]\n", binding.Name, binding.ImportPath, binding.DescriptorSymbol, binding.Evidence)
			}
			return nil
		},
	}
}

func compileAssembly(c *cli.Context) (contractcore.AssemblyCompilation, []contractcore.ModuleBinding, error) {
	return compileAssemblyWithOutputs(c, c.String("out"), c.String("code-out"))
}

func compileAssemblyWithOutputs(c *cli.Context, _ string, codeOut string) (contractcore.AssemblyCompilation, []contractcore.ModuleBinding, error) {
	codeImport := strings.TrimRight(strings.TrimSpace(c.String("code-import")), "/")
	if codeImport == "" {
		return contractcore.AssemblyCompilation{}, nil, errors.New("assembly: --code-import is required")
	}
	if strings.TrimSpace(codeOut) == "" {
		return contractcore.AssemblyCompilation{}, nil, errors.New("assembly: --code-out is required")
	}
	result, err := compileContract(c)
	if err != nil {
		return contractcore.AssemblyCompilation{}, nil, err
	}
	diagnostics := contractcore.Lint(result.Manifest)
	if contractcore.HasErrors(diagnostics) {
		for _, diagnostic := range diagnostics {
			fmt.Printf("%s %s: %s\n", strings.ToUpper(string(diagnostic.Severity)), diagnostic.Path, diagnostic.Message)
		}
		return contractcore.AssemblyCompilation{}, nil, errors.New("assembly: contract lint failed")
	}
	modules, bindings, err := contractcore.DiscoverModuleSnapshot(c.String("module-root"))
	if err != nil {
		return contractcore.AssemblyCompilation{}, nil, err
	}
	compilation, err := contractcore.CompileBoundAssembly(result.Manifest, modules, bindings, contractcore.AssemblyCodeOptions{RootImport: codeImport})
	if err != nil {
		return contractcore.AssemblyCompilation{}, nil, err
	}
	return compilation, bindings, nil
}

func compileContract(c *cli.Context) (contractcore.CompileResult, error) {
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
