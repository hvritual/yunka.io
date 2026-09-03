package add

import (
	"errors"
	"fmt"
	"os"

	"yunka.io/app/cmd/projectflow"
)

func AddApplication(options ApplicationOptions) (Report, error) {
	domain, application, err := parseApplicationKey(options.Key)
	if err != nil {
		return Report{}, requestFailure(err)
	}
	inputs, err := projectflow.DescribeOwnershipInputs(projectflow.Options{Root: options.Root})
	if err != nil {
		return Report{}, sourceFailure("", fmt.Errorf("add application: resolve project facts: %w", err))
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
		return Report{}, sourceFailure(source.Relative, errors.New("add application: canonical proto source has no package declaration"))
	}
	if protoGoPackage(string(contents)) == "" {
		return Report{}, sourceFailure(source.Relative, errors.New("add application: typed Application scaffold requires an explicit option go_package so generated application ports have a canonical Go type owner"))
	}
	for _, candidate := range sources {
		data, readErr := os.ReadFile(candidate.Absolute)
		if readErr != nil {
			return Report{}, sourceFailure(candidate.Relative, readErr)
		}
		if domainName(string(data)) == domain {
			for _, block := range applicationServices(string(data)) {
				if block.Application == application {
					return Report{}, conflictFailure(candidate.Relative, fmt.Errorf("add application: %s/%s already exists as service %s", domain, application, block.Name))
				}
			}
		}
	}
	serviceName := applicationServiceName(application)
	for _, candidate := range sources {
		data, readErr := os.ReadFile(candidate.Absolute)
		if readErr != nil {
			return Report{}, sourceFailure(candidate.Relative, readErr)
		}
		if protoPackage(string(data)) == packageName && serviceExists(string(data), serviceName) {
			return Report{}, conflictFailure(candidate.Relative, fmt.Errorf("add application: service name %s already exists in protobuf package %s", serviceName, packageName))
		}
	}
	owner, err := requireEditable(inputs.Project.Root, source.Relative)
	if err != nil {
		return Report{}, err
	}
	addition := fmt.Sprintf("\n// AX5 structural scaffold. Developer-owned; fill business Operations explicitly.\nservice %s {\n  option (yunka.dsl.v1.application) = {\n    name: %q\n  };\n  // TODO(agent): add Operations with `yunka add operation %s/%s <operation-id> ...`.\n}\n", serviceName, application, domain, application)
	updated := appendProtoBlock(string(contents), addition)
	if err := writeAtomic(source.Absolute, []byte(updated)); err != nil {
		return Report{}, sourceFailure(source.Relative, err)
	}
	return Report{
		SchemaVersion: SchemaVersion,
		Kind:          "application",
		Identity:      map[string]string{"domain": domain, "application": application, "capability": domain + "/" + application, "service": serviceName},
		Mutations:     []Mutation{{Path: source.Relative, Action: "modified", Owner: owner}},
		Effects:       contractEffects(inputs.Project, false, true),
		NextActions:   contractNextActions(""),
		Notes: []string{
			"No Operation, permission, transaction, idempotency, composition, persistence, or transport behavior was inferred.",
		},
	}, nil
}
