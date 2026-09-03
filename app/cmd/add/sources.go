package add

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"yunka.io/app/cmd/projectflow"
)

type sourceFile struct {
	Relative string
	Absolute string
}

type applicationService struct {
	Name        string
	Application string
	Start       int
	Open        int
	Close       int
	Body        string
}

func loadSources(inputs projectflow.OwnershipInputs) ([]sourceFile, error) {
	result := make([]sourceFile, 0, len(inputs.ContractSourceFiles))
	for _, relative := range inputs.ContractSourceFiles {
		relative = cleanRelative(relative)
		if relative == "" {
			continue
		}
		result = append(result, sourceFile{Relative: relative, Absolute: projectflow.ResolveDescriptorPath(inputs.Project, relative)})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Relative < result[j].Relative })
	if len(result) == 0 {
		return nil, sourceFailure(inputs.Project.ContractSource, errors.New("structural scaffold: no canonical protobuf source files were resolved"))
	}
	return result, nil
}

func messageSourcesInPackage(sources []sourceFile, packageName, message, exclude string) ([]string, error) {
	var result []string
	for _, source := range sources {
		if source.Relative == exclude {
			continue
		}
		contents, err := os.ReadFile(source.Absolute)
		if err != nil {
			return nil, sourceFailure(source.Relative, err)
		}
		text := string(contents)
		if protoPackage(text) != packageName {
			continue
		}
		if exists, _ := dtoMessageKind(text, message); exists {
			result = append(result, source.Relative)
		}
	}
	sort.Strings(result)
	return result, nil
}

func selectDomainSource(sources []sourceFile, domain, explicit string) (sourceFile, error) {
	if explicit = cleanRelative(explicit); explicit != "" {
		for _, source := range sources {
			if source.Relative != explicit {
				continue
			}
			contents, err := os.ReadFile(source.Absolute)
			if err != nil {
				return sourceFile{}, sourceFailure(source.Relative, err)
			}
			if got := domainName(string(contents)); got != domain {
				return sourceFile{}, sourceFailure(source.Relative, fmt.Errorf("structural scaffold: source declares domain %q, want %q", got, domain))
			}
			return source, nil
		}
		return sourceFile{}, sourceFailure(explicit, errors.New("structural scaffold: --source is not a canonical protobuf source file"))
	}
	var matches []sourceFile
	for _, source := range sources {
		contents, err := os.ReadFile(source.Absolute)
		if err != nil {
			return sourceFile{}, sourceFailure(source.Relative, err)
		}
		if domainName(string(contents)) == domain {
			matches = append(matches, source)
		}
	}
	return exactlyOneSource("domain "+domain, matches)
}

func selectApplicationSource(sources []sourceFile, domain, application, explicit string) (sourceFile, applicationService, error) {
	var candidates []struct {
		source  sourceFile
		service applicationService
	}
	for _, source := range sources {
		if explicit != "" && cleanRelative(explicit) != source.Relative {
			continue
		}
		contents, err := os.ReadFile(source.Absolute)
		if err != nil {
			return sourceFile{}, applicationService{}, sourceFailure(source.Relative, err)
		}
		if domainName(string(contents)) != domain {
			continue
		}
		for _, service := range applicationServices(string(contents)) {
			if service.Application == application {
				candidates = append(candidates, struct {
					source  sourceFile
					service applicationService
				}{source: source, service: service})
			}
		}
	}
	if explicit != "" {
		found := false
		for _, source := range sources {
			if source.Relative == cleanRelative(explicit) {
				found = true
				break
			}
		}
		if !found {
			return sourceFile{}, applicationService{}, sourceFailure(cleanRelative(explicit), errors.New("add operation: --source is not a canonical protobuf source file"))
		}
	}
	switch len(candidates) {
	case 1:
		return candidates[0].source, candidates[0].service, nil
	case 0:
		return sourceFile{}, applicationService{}, sourceFailure(cleanRelative(explicit), fmt.Errorf("add operation: application %s/%s was not found in canonical protobuf sources", domain, application))
	default:
		paths := make([]string, 0, len(candidates))
		for _, candidate := range candidates {
			paths = append(paths, candidate.source.Relative)
		}
		sort.Strings(paths)
		return sourceFile{}, applicationService{}, sourceFailure("", fmt.Errorf("add operation: application %s/%s is ambiguous across sources %s; pass --source", domain, application, strings.Join(paths, ", ")))
	}
}

func exactlyOneSource(identity string, matches []sourceFile) (sourceFile, error) {
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return sourceFile{}, sourceFailure("", fmt.Errorf("structural scaffold: %s has no canonical protobuf source", identity))
	default:
		paths := make([]string, 0, len(matches))
		for _, match := range matches {
			paths = append(paths, match.Relative)
		}
		sort.Strings(paths)
		return sourceFile{}, sourceFailure("", fmt.Errorf("structural scaffold: %s maps to multiple protobuf sources %s; pass --source", identity, strings.Join(paths, ", ")))
	}
}
