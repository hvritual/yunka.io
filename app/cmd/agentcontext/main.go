package agentcontext

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	contractcore "github.com/hvritual/yunka.io/pkg/contract"
	"github.com/urfave/cli"
	"yunka.io/app/cmd/projectflow"
)

const (
	AppName       = "context"
	SchemaVersion = 2
)

type Snapshot struct {
	SchemaVersion int                            `json:"schemaVersion"`
	Project       projectflow.ProjectDescriptor `json:"project"`
	Locations     []Location                    `json:"locations"`
	Commands      Commands                      `json:"commands"`
	AgentProtocol AgentProtocol                 `json:"agentProtocol"`
}

type Location struct {
	Name        string `json:"name"`
	Role        string `json:"role"`
	Path        string `json:"path"`
	Kind        string `json:"kind"`
	State       string `json:"state"`
	SHA256      string `json:"sha256,omitempty"`
	Remediation string `json:"remediation,omitempty"`
}

type Commands struct {
	Generate    string `json:"generate"`
	Check       string `json:"check"`
	Dev         string `json:"dev"`
	GraphImpact string `json:"graphImpact"`
}

type AgentProtocol struct {
	NewStructure string `json:"newStructure"`
	ExistingPlan string `json:"existingPlan"`
	ChangeBegin  string `json:"changeBegin"`
	ChangeCheck  string `json:"changeCheck"`
	ChangeVerify string `json:"changeVerify"`
	RuntimeEvent string `json:"runtimeEvent"`
}

func Command() cli.Command {
	return cli.Command{
		Name:  AppName,
		Usage: "print a read-only AI/automation context snapshot for the current Yunka project",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "root", Usage: "project root", Value: "."},
			cli.BoolFlag{Name: "json", Usage: "emit the stable machine-readable context contract"},
		},
		Action: func(context *cli.Context) error {
			snapshot, err := Build(context.String("root"))
			if err != nil {
				return err
			}
			if context.Bool("json") {
				contents, err := MarshalJSON(snapshot)
				if err != nil {
					return err
				}
				fmt.Print(string(contents))
				return nil
			}
			fmt.Print(FormatText(snapshot))
			return nil
		},
	}
}

func Build(root string) (Snapshot, error) {
	descriptor, err := projectflow.DescribeProject(projectflow.Options{Root: root})
	if err != nil {
		return Snapshot{}, err
	}

	locations := []Location{
		location(descriptor, "contract-source", "source", descriptor.ContractSource, remediationForSource(descriptor)),
		location(descriptor, "modules", "source", descriptor.ModulesRoot, ""),
		location(descriptor, "provider-manifest", "managed", descriptor.ProviderManifest, "run `yunka init` to adopt the managed provider manifest"),
		location(descriptor, "protobuf-go-manifest", "managed", descriptor.ProtobufGoManifest, "run `yunka init` to adopt strict protobuf Go output ownership"),
		location(descriptor, "dev-manifest", "runtime-config", descriptor.DevManifest, "configure the project dev manifest before running `yunka dev`"),
		location(descriptor, "contract-manifest", "generated", filepath.ToSlash(filepath.Join(descriptor.ContractGenerated, contractcore.ManifestFilename)), "run `yunka generate`"),
		location(descriptor, "operation-plans", "generated", filepath.ToSlash(filepath.Join(descriptor.ContractGenerated, contractcore.OperationPlansFilename)), "run `yunka generate`"),
		location(descriptor, "assembly-plan", "generated", filepath.ToSlash(filepath.Join(descriptor.ContractGenerated, contractcore.AssemblyPlanFilename)), "run `yunka generate`"),
		location(descriptor, "application-graph", "generated", filepath.ToSlash(filepath.Join(descriptor.ContractGenerated, "application-graph.json")), "run `yunka generate`"),
	}
	if descriptor.Profile != "" {
		locations = append(locations, location(descriptor, "project-profile", "managed", descriptor.Profile, "run `yunka init`"))
	}
	sort.Slice(locations, func(i, j int) bool {
		if locations[i].Role != locations[j].Role {
			return locations[i].Role < locations[j].Role
		}
		return locations[i].Name < locations[j].Name
	})

	return Snapshot{
		SchemaVersion: SchemaVersion,
		Project:       descriptor,
		Locations:     locations,
		Commands: Commands{
			Generate:    "yunka generate",
			Check:       "yunka check --format agent-json",
			Dev:         "yunka dev",
			GraphImpact: "yunka graph impact --format json --operation <operation>",
		},
		AgentProtocol: AgentProtocol{
			NewStructure: "yunka add <application|operation|event|module> ...",
			ExistingPlan: "yunka change plan --operation <operation> --format agent-json",
			ChangeBegin:  "yunka change begin --operation <operation> --format agent-json",
			ChangeCheck:  "yunka change check --format agent-json",
			ChangeVerify: "yunka change verify --format agent-json",
			RuntimeEvent: "yunka dev --event-format jsonl",
		},
	}, nil
}

func MarshalJSON(snapshot Snapshot) ([]byte, error) {
	contents, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(contents, '\n'), nil
}

func FormatText(snapshot Snapshot) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "PROJECT module=%s profiled=%t\n", valueOr(snapshot.Project.GoModule, "<unknown>"), snapshot.Project.Profiled)
	fmt.Fprintf(&builder, "SOURCE  %s %s\n", snapshot.Project.ContractSourceKind, snapshot.Project.ContractSource)
	for _, item := range snapshot.Locations {
		fmt.Fprintf(&builder, "%-7s %-20s %-8s %s", strings.ToUpper(item.State), item.Name, item.Role, item.Path)
		if item.SHA256 != "" {
			fmt.Fprintf(&builder, " sha256=%s", item.SHA256)
		}
		if item.State == "missing" && item.Remediation != "" {
			fmt.Fprintf(&builder, " action=%s", item.Remediation)
		}
		builder.WriteByte('\n')
	}
	return builder.String()
}

func location(descriptor projectflow.ProjectDescriptor, name, role, path, remediation string) Location {
	item := Location{Name: name, Role: role, Path: filepath.ToSlash(path), State: "missing", Remediation: remediation}
	absolute := projectflow.ResolveDescriptorPath(descriptor, path)
	info, err := os.Stat(absolute)
	if err != nil {
		if !os.IsNotExist(err) {
			item.State = "unreadable"
		}
		return item
	}
	item.State = "present"
	if info.IsDir() {
		item.Kind = "directory"
		item.Remediation = ""
		return item
	}
	item.Kind = "file"
	contents, err := os.ReadFile(absolute)
	if err != nil {
		item.State = "unreadable"
		return item
	}
	digest := sha256.Sum256(contents)
	item.SHA256 = hex.EncodeToString(digest[:])
	item.Remediation = ""
	return item
}

func remediationForSource(descriptor projectflow.ProjectDescriptor) string {
	if descriptor.Profiled {
		return "repair the project profile or canonical contract source"
	}
	return "run `yunka init` or add the canonical contract source"
}

func valueOr(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
