package projectflow

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var goPackageValuePattern = regexp.MustCompile(`(?m)^\s*option\s+go_package\s*=\s*"([^"]+)"\s*;`)

// ProtobufGoOutputCandidatesForSource derives the exact local Go output paths
// that protoc-gen-go/protoc-gen-go-grpc may update for one canonical protobuf
// source under Yunka's existing --go_opt=module=<project.GoModule> contract.
//
// The returned paths are candidates only. Callers must still prove generator
// ownership (for example through AX2 ownership classification) before granting
// mutation authority. This preserves legacy marker-based ownership without
// turning path shape alone into a generated-code source of truth.
func ProtobufGoOutputCandidatesForSource(options Options, source string) ([]string, error) {
	project, err := resolveProject(options)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(project.GoModule) == "" {
		return []string{}, nil
	}

	absolute, err := canonicalProtoSource(project, source)
	if err != nil {
		return nil, err
	}
	contents, err := os.ReadFile(absolute)
	if err != nil {
		return nil, err
	}
	match := goPackageValuePattern.FindSubmatch(contents)
	if len(match) != 2 {
		return []string{}, nil
	}
	goPackage := strings.TrimSpace(string(match[1]))
	if index := strings.IndexByte(goPackage, ';'); index >= 0 {
		goPackage = strings.TrimSpace(goPackage[:index])
	}
	if goPackage == "" {
		return nil, fmt.Errorf("protobuf-go output candidates: %s has an empty go_package import path", relative(project.Root, absolute))
	}

	module := strings.TrimSuffix(strings.TrimSpace(project.GoModule), "/")
	var outputDir string
	switch {
	case goPackage == module:
		outputDir = ""
	case strings.HasPrefix(goPackage, module+"/"):
		outputDir = strings.TrimPrefix(goPackage, module+"/")
	default:
		return nil, fmt.Errorf("protobuf-go output candidates: go_package %q for %s is outside project module %q", goPackage, relative(project.Root, absolute), module)
	}

	base := filepath.Base(absolute)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	candidates := []string{
		filepath.ToSlash(filepath.Join(filepath.FromSlash(outputDir), stem+".pb.go")),
		filepath.ToSlash(filepath.Join(filepath.FromSlash(outputDir), stem+"_grpc.pb.go")),
	}
	sort.Strings(candidates)
	return candidates, nil
}

func canonicalProtoSource(project resolvedProject, source string) (string, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return "", fmt.Errorf("protobuf-go output candidates: source is required")
	}
	candidate := filepath.FromSlash(source)
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(project.Root, candidate)
	}
	candidate, err := filepath.Abs(candidate)
	if err != nil {
		return "", err
	}
	candidate = filepath.Clean(candidate)
	relativePath, err := filepath.Rel(project.Root, candidate)
	if err != nil || relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("protobuf-go output candidates: source %q is outside project root", source)
	}

	sets, err := protobufSourceSets(project)
	if err != nil {
		return "", err
	}
	matches := 0
	for _, set := range sets {
		for _, file := range set.Files {
			absolute := filepath.Clean(filepath.Join(set.Root, filepath.FromSlash(file)))
			if absolute == candidate {
				matches++
			}
		}
	}
	if matches != 1 {
		return "", fmt.Errorf("protobuf-go output candidates: source %s must resolve to exactly one canonical protobuf source, found %d", filepath.ToSlash(relativePath), matches)
	}
	return candidate, nil
}
