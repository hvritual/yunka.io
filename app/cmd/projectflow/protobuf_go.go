package projectflow

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	projectcmd "yunka.io/app/cmd/project"
	contractcore "yunka.io/pkg/contract"
)

const protobufGoManifestVersion = 1

var goPackageOptionPattern = regexp.MustCompile(`(?s)(^|[[:space:]])option[[:space:]]+go_package[[:space:]]*=[[:space:]]*"[^"]+"[[:space:]]*;`)

type protobufGoManifest struct {
	SchemaVersion int      `json:"schemaVersion"`
	Files         []string `json:"files"`
}

type protobufSourceSet struct {
	Root       string
	Files      []string
	ProtoPaths []string
}

func generateProtobufGo(ctx context.Context, project resolvedProject) (int, bool, error) {
	expected, enabled, err := renderProtobufGo(ctx, project)
	if err != nil {
		return 0, enabled, err
	}
	if err := installProtobufGo(project, expected); err != nil {
		return 0, enabled, err
	}
	return len(expected), enabled, nil
}

func checkProtobufGo(ctx context.Context, project resolvedProject) (int, bool, error) {
	expected, enabled, err := renderProtobufGo(ctx, project)
	if err != nil {
		return 0, enabled, err
	}
	expectedFiles := sortedGeneratedPaths(expected)
	manifest, exists, err := loadProtobufGoManifest(project)
	if err != nil {
		return 0, enabled, err
	}
	if exists && !equalStringSlices(manifest.Files, expectedFiles) {
		return 0, enabled, fmt.Errorf("protobuf-go: generated output set drift; have=%v want=%v; run `yunka generate`", manifest.Files, expectedFiles)
	}
	for _, relative := range expectedFiles {
		actual, err := os.ReadFile(filepath.Join(project.Root, filepath.FromSlash(relative)))
		if err != nil {
			if os.IsNotExist(err) {
				return 0, enabled, fmt.Errorf("protobuf-go: missing generated file %s; run `yunka generate`", relative)
			}
			return 0, enabled, err
		}
		if !bytes.Equal(actual, expected[relative]) {
			return 0, enabled, fmt.Errorf("protobuf-go: generated content drift in %s; run `yunka generate`", relative)
		}
	}
	return len(expectedFiles), enabled, nil
}

func protobufGoRequired(project resolvedProject) (bool, error) {
	if strings.TrimSpace(project.GoModule) == "" {
		return false, nil
	}
	sets, err := protobufSourceSets(project)
	if err != nil {
		return false, err
	}
	for _, set := range sets {
		files, err := goPackageProtoFiles(set.Root, set.Files)
		if err != nil {
			return false, err
		}
		if len(files) > 0 {
			return true, nil
		}
	}
	return false, nil
}

// protobufGoFastFeedbackSafe prevents legacy projects without the explicit
// ownership marker from reusing incomplete output fingerprints. They continue
// to receive canonical generate/check behavior until `yunka init` adopts the
// strict manifest-backed ownership model.
func protobufGoFastFeedbackSafe(project resolvedProject) bool {
	required, err := protobufGoRequired(project)
	if err != nil {
		return false
	}
	if !required {
		return true
	}
	_, exists, err := loadProtobufGoManifest(project)
	return err == nil && exists
}

func renderProtobufGo(ctx context.Context, project resolvedProject) (map[string][]byte, bool, error) {
	if strings.TrimSpace(project.GoModule) == "" {
		return map[string][]byte{}, false, nil
	}
	sets, err := protobufSourceSets(project)
	if err != nil {
		return nil, false, err
	}
	type generationSet struct {
		protobufSourceSet
		Eligible []string
	}
	generations := make([]generationSet, 0, len(sets))
	for _, set := range sets {
		eligible, err := goPackageProtoFiles(set.Root, set.Files)
		if err != nil {
			return nil, false, err
		}
		if len(eligible) > 0 {
			generations = append(generations, generationSet{protobufSourceSet: set, Eligible: eligible})
		}
	}
	if len(generations) == 0 {
		return map[string][]byte{}, false, nil
	}

	protoc, err := resolveProjectExecutable(project.Root, project.Protoc, "protoc")
	if err != nil {
		return nil, true, err
	}
	goPlugin, err := resolveProtobufPlugin(project.Root, "PROTOC_GEN_GO", "protoc-gen-go")
	if err != nil {
		return nil, true, err
	}
	grpcPlugin, err := resolveProtobufPlugin(project.Root, "PROTOC_GEN_GO_GRPC", "protoc-gen-go-grpc")
	if err != nil {
		return nil, true, err
	}
	standardInclude := protobufStandardInclude(protoc)
	stageRoot, err := os.MkdirTemp("", "yunka-protobuf-go-*")
	if err != nil {
		return nil, true, err
	}
	defer os.RemoveAll(stageRoot)

	for _, generation := range generations {
		args := []string{"-I", generation.Root}
		if standardInclude != "" && filepath.Clean(standardInclude) != filepath.Clean(generation.Root) {
			args = append(args, "-I", standardInclude)
		}
		for _, protoPath := range generation.ProtoPaths {
			if strings.TrimSpace(protoPath) != "" {
				args = append(args, "-I", filepath.Clean(protoPath))
			}
		}
		args = append(args,
			"--plugin=protoc-gen-go="+goPlugin,
			"--plugin=protoc-gen-go-grpc="+grpcPlugin,
			"--go_out="+stageRoot,
			"--go_opt=module="+project.GoModule,
			"--go-grpc_out="+stageRoot,
			"--go-grpc_opt=module="+project.GoModule+",require_unimplemented_servers=false",
		)
		args = append(args, generation.Eligible...)
		command := exec.CommandContext(ctx, protoc, args...)
		command.Dir = generation.Root
		output, err := command.CombinedOutput()
		if err != nil {
			return nil, true, fmt.Errorf("protobuf-go: protoc failed: %w: %s", err, strings.TrimSpace(string(output)))
		}
	}
	generated, err := collectGeneratedProtobufGo(stageRoot)
	if err != nil {
		return nil, true, err
	}
	return generated, true, nil
}

func protobufSourceSets(project resolvedProject) ([]protobufSourceSet, error) {
	if project.InventoryPath != "" {
		inventory, err := contractcore.LoadSourceInventory(project.InventoryPath)
		if err != nil {
			return nil, err
		}
		sources := append([]contractcore.SourceSet(nil), inventory.SourceSets...)
		sort.Slice(sources, func(i, j int) bool { return sources[i].Name < sources[j].Name })
		sets := make([]protobufSourceSet, 0, len(sources))
		for _, source := range sources {
			root, err := containedProjectPath(project.Root, source.Root)
			if err != nil {
				return nil, fmt.Errorf("protobuf-go: source set %s root: %w", source.Name, err)
			}
			protoPaths := make([]string, 0, len(source.ProtoPaths))
			for _, value := range source.ProtoPaths {
				path, err := containedProjectPath(project.Root, value)
				if err != nil {
					return nil, fmt.Errorf("protobuf-go: source set %s protoPath: %w", source.Name, err)
				}
				protoPaths = append(protoPaths, path)
			}
			files := append([]string(nil), source.Files...)
			for index := range files {
				files[index] = filepath.ToSlash(filepath.Clean(files[index]))
			}
			sort.Strings(files)
			sets = append(sets, protobufSourceSet{Root: root, Files: files, ProtoPaths: protoPaths})
		}
		return sets, nil
	}
	files, err := discoverProtoFilesForGeneration(project.ProtoDir)
	if err != nil {
		return nil, err
	}
	return []protobufSourceSet{{Root: project.ProtoDir, Files: files, ProtoPaths: append([]string(nil), project.AdditionalProtoPaths...)}}, nil
}

func discoverProtoFilesForGeneration(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if !strings.EqualFold(filepath.Ext(entry.Name()), ".proto") {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(relative))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func goPackageProtoFiles(root string, files []string) ([]string, error) {
	eligible := make([]string, 0, len(files))
	for _, relative := range files {
		clean, err := safeGeneratedRelative(relative)
		if err != nil {
			return nil, fmt.Errorf("protobuf-go: invalid proto file %q: %w", relative, err)
		}
		contents, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(clean)))
		if err != nil {
			return nil, err
		}
		if goPackageOptionPattern.Match(contents) {
			eligible = append(eligible, clean)
		}
	}
	sort.Strings(eligible)
	return eligible, nil
}

func collectGeneratedProtobufGo(root string) (map[string][]byte, error) {
	generated := make(map[string][]byte)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("protobuf-go: generated output may not be a symlink: %s", path)
		}
		if !strings.HasSuffix(entry.Name(), ".pb.go") {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative, err = safeGeneratedRelative(relative)
		if err != nil {
			return err
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if _, duplicate := generated[relative]; duplicate {
			return fmt.Errorf("protobuf-go: duplicate generated output %s", relative)
		}
		generated[relative] = contents
		return nil
	})
	return generated, err
}

func installProtobufGo(project resolvedProject, expected map[string][]byte) error {
	manifest, exists, err := loadProtobufGoManifest(project)
	if err != nil {
		return err
	}
	expectedFiles := sortedGeneratedPaths(expected)
	if exists {
		expectedSet := make(map[string]struct{}, len(expectedFiles))
		for _, path := range expectedFiles {
			expectedSet[path] = struct{}{}
		}
		for _, stale := range manifest.Files {
			if _, keep := expectedSet[stale]; keep {
				continue
			}
			target := filepath.Join(project.Root, filepath.FromSlash(stale))
			contents, err := os.ReadFile(target)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return err
			}
			if !isGeneratedProtobufGo(contents) {
				return fmt.Errorf("protobuf-go: refusing to remove stale non-generated file %s", stale)
			}
			if err := os.Remove(target); err != nil {
				return err
			}
		}
	}
	for _, relative := range expectedFiles {
		target := filepath.Join(project.Root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return err
		}
		if err := os.WriteFile(target, expected[relative], 0o640); err != nil {
			return err
		}
	}
	if !exists {
		return nil
	}
	manifestPath := filepath.Join(project.Root, filepath.FromSlash(projectcmd.ProtobufGoManifestRelativePath))
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o750); err != nil {
		return err
	}
	contents, err := json.MarshalIndent(protobufGoManifest{SchemaVersion: protobufGoManifestVersion, Files: expectedFiles}, "", "  ")
	if err != nil {
		return err
	}
	contents = append(contents, '\n')
	return os.WriteFile(manifestPath, contents, 0o640)
}

func loadProtobufGoManifest(project resolvedProject) (protobufGoManifest, bool, error) {
	path := filepath.Join(project.Root, filepath.FromSlash(projectcmd.ProtobufGoManifestRelativePath))
	contents, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return protobufGoManifest{}, false, nil
	}
	if err != nil {
		return protobufGoManifest{}, false, err
	}
	var manifest protobufGoManifest
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return protobufGoManifest{}, false, fmt.Errorf("protobuf-go: decode %s: %w", projectcmd.ProtobufGoManifestRelativePath, err)
	}
	if manifest.SchemaVersion != protobufGoManifestVersion {
		return protobufGoManifest{}, false, fmt.Errorf("protobuf-go: unsupported manifest schemaVersion %d", manifest.SchemaVersion)
	}
	seen := make(map[string]struct{}, len(manifest.Files))
	for index, file := range manifest.Files {
		clean, err := safeGeneratedRelative(file)
		if err != nil {
			return protobufGoManifest{}, false, fmt.Errorf("protobuf-go: invalid managed file %q: %w", file, err)
		}
		if clean != file {
			return protobufGoManifest{}, false, fmt.Errorf("protobuf-go: managed file %q is not canonical", file)
		}
		if _, duplicate := seen[file]; duplicate {
			return protobufGoManifest{}, false, fmt.Errorf("protobuf-go: duplicate managed file %q", file)
		}
		seen[file] = struct{}{}
		if index > 0 && manifest.Files[index-1] > file {
			return protobufGoManifest{}, false, errors.New("protobuf-go: manifest files must be sorted")
		}
	}
	return manifest, true, nil
}

func protobufGoOutputPaths(project resolvedProject) []string {
	manifest, exists, err := loadProtobufGoManifest(project)
	if err != nil || !exists {
		return nil
	}
	return append([]string(nil), manifest.Files...)
}

func sortedGeneratedPaths(files map[string][]byte) []string {
	result := make([]string, 0, len(files))
	for path := range files {
		result = append(result, path)
	}
	sort.Strings(result)
	return result
}

func equalStringSlices(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func isGeneratedProtobufGo(contents []byte) bool {
	return bytes.HasPrefix(contents, []byte("// Code generated by protoc-gen-go"))
}

func safeGeneratedRelative(value string) (string, error) {
	value = filepath.ToSlash(filepath.Clean(strings.TrimSpace(value)))
	if value == "" || value == "." || value == ".." || strings.HasPrefix(value, "../") || filepath.IsAbs(filepath.FromSlash(value)) {
		return "", errors.New("path must be repository-relative")
	}
	return value, nil
}

func containedProjectPath(root, value string) (string, error) {
	clean, err := safeGeneratedRelative(value)
	if err != nil {
		return "", err
	}
	path := filepath.Join(root, filepath.FromSlash(clean))
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return "", err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", errors.New("path escapes project root")
	}
	return filepath.Clean(path), nil
}

func resolveProtobufPlugin(projectRoot, envName, binary string) (string, error) {
	if configured := strings.TrimSpace(os.Getenv(envName)); configured != "" {
		return resolveProjectExecutable(projectRoot, configured, binary)
	}
	for _, name := range []string{binary, binary + ".exe"} {
		candidate := filepath.Join(projectRoot, ".yunka", "bin", name)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return filepath.Abs(candidate)
		}
	}
	if path, err := exec.LookPath(binary); err == nil {
		return filepath.Abs(path)
	}
	return "", fmt.Errorf("protobuf-go: %s is required; set %s, install it on PATH, or place it in .yunka/bin", binary, envName)
}

func resolveProjectExecutable(projectRoot, configured, fallback string) (string, error) {
	value := strings.TrimSpace(configured)
	if value == "" {
		value = fallback
	}
	if filepath.IsAbs(value) {
		if info, err := os.Stat(value); err != nil {
			return "", err
		} else if info.IsDir() {
			return "", fmt.Errorf("%s is a directory", value)
		}
		return filepath.Clean(value), nil
	}
	if strings.ContainsAny(value, `/\\`) {
		candidate := filepath.Join(projectRoot, value)
		if info, err := os.Stat(candidate); err != nil {
			return "", err
		} else if info.IsDir() {
			return "", fmt.Errorf("%s is a directory", candidate)
		}
		return filepath.Abs(candidate)
	}
	path, err := exec.LookPath(value)
	if err != nil {
		return "", err
	}
	return filepath.Abs(path)
}

func protobufStandardInclude(protoc string) string {
	for _, candidate := range []string{
		filepath.Join(filepath.Dir(filepath.Dir(protoc)), "include"),
		filepath.Join(filepath.Dir(protoc), "include"),
	} {
		if info, err := os.Stat(filepath.Join(candidate, "google", "protobuf", "descriptor.proto")); err == nil && !info.IsDir() {
			return filepath.Clean(candidate)
		}
	}
	return ""
}
