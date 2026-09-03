package projectflow

import (
	"path/filepath"
	"sort"

	domaincmd "yunka.io/app/cmd/domain"
)

// OwnershipInputs is a transient projection used by AX control-plane tooling.
// Every path is derived from an existing canonical owner (project profile,
// contract source inventory, protobuf output manifest, or Domain generator).
// It is never persisted as a second ownership source of truth.
type OwnershipInputs struct {
	Project                  ProjectDescriptor `json:"project"`
	ContractSourceFiles      []string          `json:"contractSourceFiles"`
	ProtobufGoGeneratedFiles []string          `json:"protobufGoGeneratedFiles"`
	DomainGeneratedFiles     []string          `json:"domainGeneratedFiles"`
}

func DescribeOwnershipInputs(options Options) (OwnershipInputs, error) {
	project, err := resolveProject(options)
	if err != nil {
		return OwnershipInputs{}, err
	}

	sourceSets, err := protobufSourceSets(project)
	if err != nil {
		return OwnershipInputs{}, err
	}
	var sourceFiles []string
	for _, set := range sourceSets {
		for _, file := range set.Files {
			absolute := filepath.Join(set.Root, filepath.FromSlash(file))
			sourceFiles = append(sourceFiles, relative(project.Root, absolute))
		}
	}
	sort.Strings(sourceFiles)

	var protobufFiles []string
	manifest, exists, err := loadProtobufGoManifest(project)
	if err != nil {
		return OwnershipInputs{}, err
	}
	if exists {
		protobufFiles = append(protobufFiles, manifest.Files...)
		sort.Strings(protobufFiles)
	}

	domainFiles, err := domaincmd.GeneratedPaths(project.CodeOut)
	if err != nil {
		return OwnershipInputs{}, err
	}
	codeRoot := relative(project.Root, project.CodeOut)
	for index := range domainFiles {
		domainFiles[index] = filepath.ToSlash(filepath.Join(codeRoot, filepath.FromSlash(domainFiles[index])))
	}
	sort.Strings(domainFiles)

	return OwnershipInputs{
		Project:                  describeResolvedProject(project),
		ContractSourceFiles:      sourceFiles,
		ProtobufGoGeneratedFiles: protobufFiles,
		DomainGeneratedFiles:     domainFiles,
	}, nil
}
