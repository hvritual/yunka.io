package add

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/hvritual/yunka.io/pkg/contract"
	"github.com/hvritual/yunka.io/pkg/modulespec"
	"yunka.io/app/cmd/module"
	"yunka.io/app/cmd/projectflow"
)

func AddEvent(options EventOptions) (Report, error) {
	domain := strings.TrimSpace(options.Domain)
	name := strings.TrimSpace(options.Name)
	if !validPolicyKey(domain) {
		return Report{}, requestFailure(fmt.Errorf("add event: domain %q must be a stable lowercase key", domain))
	}
	if !validPolicyKey(name) {
		return Report{}, requestFailure(fmt.Errorf("add event: event name %q must be a stable lowercase key", name))
	}
	message := strings.TrimSpace(options.Message)
	if message == "" {
		message = exportedIdentifier(name)
	}
	if !goIdentifierPattern.MatchString(message) || !unicode.IsUpper([]rune(message)[0]) {
		return Report{}, requestFailure(fmt.Errorf("add event: message %q must be an exported protobuf identifier", message))
	}
	inputs, err := projectflow.DescribeOwnershipInputs(projectflow.Options{Root: options.Root})
	if err != nil {
		return Report{}, sourceFailure("", fmt.Errorf("add event: resolve project facts: %w", err))
	}
	sources, err := loadSources(inputs)
	if err != nil {
		return Report{}, err
	}
	source, err := selectDomainSource(sources, domain, options.Source)
	if err != nil {
		return Report{}, err
	}
	contents, err := os.ReadFile(source.Absolute)
	if err != nil {
		return Report{}, sourceFailure(source.Relative, err)
	}
	packageName := protoPackage(string(contents))
	if packageName == "" {
		return Report{}, sourceFailure(source.Relative, errors.New("add event: canonical proto source has no package declaration"))
	}
	if exists, _ := dtoMessageKind(string(contents), message); exists {
		return Report{}, conflictFailure(source.Relative, fmt.Errorf("add event: message %s already exists", message))
	}
	if paths, err := messageSourcesInPackage(sources, packageName, message, source.Relative); err != nil {
		return Report{}, err
	} else if len(paths) > 0 {
		return Report{}, conflictFailure("", fmt.Errorf("add event: message %s already exists in protobuf package %s (%s)", message, packageName, strings.Join(paths, ", ")))
	}
	owner, err := requireEditable(inputs.Project.Root, source.Relative)
	if err != nil {
		return Report{}, err
	}
	updated := appendProtoBlock(string(contents), renderDTOMessage(message, "DTO_EVENT", "event payload fields"))
	if err := writeAtomic(source.Absolute, []byte(updated)); err != nil {
		return Report{}, sourceFailure(source.Relative, err)
	}
	return Report{
		SchemaVersion: SchemaVersion,
		Kind:          "event",
		Identity:      map[string]string{"domain": domain, "event": name, "message": message},
		Mutations:     []Mutation{{Path: source.Relative, Action: "modified", Owner: owner}},
		Effects:       contractEffects(inputs.Project, false, false),
		NextActions:   contractNextActions(""),
		Notes: []string{
			"The scaffold declares only a DTO_EVENT schema type.",
			"It does not infer publisher, topic, broker, delivery guarantee, Outbox, transaction, or external-effect semantics.",
		},
	}, nil
}

func AddModule(options ModuleOptions) (Report, error) {
	name := strings.TrimSpace(options.Name)
	if name == "" {
		return Report{}, requestFailure(errors.New("add module: module name is required"))
	}
	inputs, err := projectflow.DescribeOwnershipInputs(projectflow.Options{Root: options.Root})
	if err != nil {
		return Report{}, sourceFailure("", fmt.Errorf("add module: resolve project facts: %w", err))
	}
	relative := filepath.ToSlash(filepath.Join(inputs.Project.ModulesRoot, name, modulespec.Filename))
	owner, err := requireEditable(inputs.Project.Root, relative)
	if err != nil {
		return Report{}, err
	}
	moduleRoot := projectflow.ResolveDescriptorPath(inputs.Project, inputs.Project.ModulesRoot)
	if err := module.AddSpec(module.SpecOptions{
		Name: name, Root: moduleRoot, Version: options.Version, ConfigKey: options.ConfigKey,
		Logger: options.Logger, Databases: options.Databases, EventBus: options.EventBus, RPC: options.RPC, DependsOn: options.DependsOn,
	}); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "already exists") {
			return Report{}, conflictFailure(relative, err)
		}
		return Report{}, requestFailure(err)
	}
	createdSpec, err := modulespec.Load(projectflow.ResolveDescriptorPath(inputs.Project, relative))
	if err != nil {
		return Report{}, sourceFailure(relative, fmt.Errorf("add module: read created declarative spec: %w", err))
	}
	return Report{
		SchemaVersion: SchemaVersion,
		Kind:          "module",
		Identity:      map[string]string{"module": name, "version": createdSpec.Version},
		Mutations:     []Mutation{{Path: relative, Action: "created", Owner: owner}},
		Effects: []Effect{
			{Stage: "module", Scope: inputs.Project.ModulesRoot, Conditional: true, Reason: "module catalog is derived from declarative module specs"},
			{Stage: "assembly", Path: filepath.ToSlash(filepath.Join(inputs.Project.ContractGenerated, contract.AssemblyPlanFilename)), Conditional: true, Reason: "assembly may change when a module is enabled by project configuration"},
		},
		NextActions: []NextAction{
			{Command: "yunka module check", Purpose: "validate the declarative module contract"},
			{Command: "yunka generate", Purpose: "derive module and assembly artifacts"},
			{Command: "yunka check --format agent-json", Purpose: "verify project closure"},
		},
		Notes: []string{"Capabilities and dependencies are present only when explicitly supplied on the command line; no runtime Build or provider behavior was generated."},
	}, nil
}
