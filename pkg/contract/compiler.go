package contract

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type CompileOptions struct {
	Dir        string
	ProtoPaths []string
	Files      []string
	Protoc     string
}

type CompileResult struct {
	Manifest      Manifest
	DescriptorSHA string
	Files         []string
}

func Compile(ctx context.Context, options CompileOptions) (CompileResult, error) {
	dir := options.Dir
	if dir == "" {
		dir = "."
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return CompileResult{}, err
	}
	files := append([]string(nil), options.Files...)
	if len(files) == 0 {
		files, err = discoverProtoFiles(absDir)
		if err != nil {
			return CompileResult{}, err
		}
	}
	if len(files) == 0 {
		return CompileResult{}, fmt.Errorf("contract: no .proto files found in %s", absDir)
	}
	for i := range files {
		files[i] = filepath.ToSlash(files[i])
	}
	sort.Strings(files)

	protoc, err := resolveProtoc(options.Protoc)
	if err != nil {
		return CompileResult{}, err
	}
	tmp, err := os.CreateTemp("", "yunka-contract-*.pb")
	if err != nil {
		return CompileResult{}, err
	}
	descriptorPath := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(descriptorPath)

	args := []string{"--include_imports", "--include_source_info", "--descriptor_set_out=" + descriptorPath, "-I", absDir}
	if include := standardProtoInclude(protoc); include != "" && filepath.Clean(include) != filepath.Clean(absDir) {
		args = append(args, "-I", include)
	}
	for _, protoPath := range options.ProtoPaths {
		if protoPath == "" {
			continue
		}
		if !filepath.IsAbs(protoPath) {
			protoPath = filepath.Join(absDir, protoPath)
		}
		args = append(args, "-I", filepath.Clean(protoPath))
	}
	args = append(args, files...)
	cmd := exec.CommandContext(ctx, protoc, args...)
	cmd.Dir = absDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return CompileResult{}, fmt.Errorf("contract: protoc failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	data, err := os.ReadFile(descriptorPath)
	if err != nil {
		return CompileResult{}, err
	}
	manifest, err := ManifestFromDescriptorSet(data, files)
	if err != nil {
		return CompileResult{}, err
	}
	digest := sha256.Sum256(data)
	return CompileResult{
		Manifest:      manifest,
		DescriptorSHA: hex.EncodeToString(digest[:]),
		Files:         files,
	}, nil
}

func ManifestFromDescriptorSet(data []byte, roots []string) (Manifest, error) {
	set, err := parseDescriptorSet(data)
	if err != nil {
		return Manifest{}, err
	}
	rootSet := make(map[string]struct{}, len(roots))
	for _, root := range roots {
		rootSet[filepath.ToSlash(filepath.Clean(root))] = struct{}{}
	}
	manifest := Manifest{SchemaVersion: ManifestVersion}
	messageDescriptors := make(map[string]messageDescriptor)
	mapEntries := make(map[string]messageDescriptor)

	for _, file := range set.Files {
		if !isRootFile(file.Name, rootSet, len(roots) == 0) || isDSLSupportFile(file.Name) {
			continue
		}
		manifest.Files = append(manifest.Files, File{
			Name:      file.Name,
			Package:   file.Package,
			Syntax:    file.Syntax,
			GoPackage: file.GoPackage,
		})
		collectMessageDescriptors(file.Package, "", file.Messages, messageDescriptors, mapEntries)
	}

	for _, file := range set.Files {
		if !isRootFile(file.Name, rootSet, len(roots) == 0) || isDSLSupportFile(file.Name) {
			continue
		}
		appendMessages(&manifest, file.Package, "", file.Messages, messageDescriptors, mapEntries)
		appendEnums(&manifest, file.Package, "", file.Enums)
		appendNestedEnums(&manifest, file.Package, "", file.Messages)
		for serviceIndex, service := range file.Services {
			serviceFullName := fullName(file.Package, "", service.Name)
			contractService := Service{Name: service.Name, FullName: serviceFullName}
			for methodIndex, method := range service.Methods {
				methodFullName := serviceFullName + "." + method.Name
				comment := file.SourceInfo.Comments[pathKey([]int32{6, int32(serviceIndex), 2, int32(methodIndex)})]
				directives := parseDirectives(comment)
				bindings, err := parseHTTPBindings(method.Options)
				if err != nil {
					return Manifest{}, fmt.Errorf("contract: %s: %w", methodFullName, err)
				}
				if len(bindings) == 0 {
					if binding, ok := directiveHTTPBinding(directives); ok {
						bindings = append(bindings, binding)
					}
				}
				contractService.Methods = append(contractService.Methods, Method{
					Name:            method.Name,
					FullName:        methodFullName,
					Request:         method.InputType,
					Response:        method.OutputType,
					ClientStreaming: method.ClientStreaming,
					ServerStreaming: method.ServerStreaming,
					HTTP:            bindings,
					Directives:      directives,
					Authorization:   authorizationFromDirectives(directives),
				})
			}
			manifest.Services = append(manifest.Services, contractService)
		}
	}
	if err := applyDSLDeclarations(&manifest, data); err != nil {
		return Manifest{}, err
	}
	manifest.Normalize()
	return manifest, nil
}

func isRootFile(name string, roots map[string]struct{}, includeAll bool) bool {
	if includeAll {
		return !strings.HasPrefix(name, "google/")
	}
	_, ok := roots[filepath.ToSlash(filepath.Clean(name))]
	return ok
}

func collectMessageDescriptors(pkg, parent string, messages []messageDescriptor, out, mapEntries map[string]messageDescriptor) {
	for _, message := range messages {
		name := fullName(pkg, parent, message.Name)
		out[name] = message
		if message.MapEntry {
			mapEntries[name] = message
		}
		nextParent := message.Name
		if parent != "" {
			nextParent = parent + "." + message.Name
		}
		collectMessageDescriptors(pkg, nextParent, message.Nested, out, mapEntries)
	}
}

func appendMessages(manifest *Manifest, pkg, parent string, messages []messageDescriptor, all, mapEntries map[string]messageDescriptor) {
	for _, message := range messages {
		name := fullName(pkg, parent, message.Name)
		nextParent := message.Name
		if parent != "" {
			nextParent = parent + "." + message.Name
		}
		if !message.MapEntry {
			contractMessage := Message{Name: message.Name, FullName: name}
			for _, field := range message.Fields {
				contractMessage.Fields = append(contractMessage.Fields, buildField(field, all, mapEntries))
			}
			manifest.Messages = append(manifest.Messages, contractMessage)
		}
		appendMessages(manifest, pkg, nextParent, message.Nested, all, mapEntries)
	}
}

func appendEnums(manifest *Manifest, pkg, parent string, enums []enumDescriptor) {
	for _, item := range enums {
		contractEnum := Enum{Name: item.Name, FullName: fullName(pkg, parent, item.Name)}
		for _, value := range item.Values {
			contractEnum.Values = append(contractEnum.Values, EnumValue{Name: value.Name, Number: value.Number})
		}
		manifest.Enums = append(manifest.Enums, contractEnum)
	}
}

func appendNestedEnums(manifest *Manifest, pkg, parent string, messages []messageDescriptor) {
	for _, message := range messages {
		nextParent := message.Name
		if parent != "" {
			nextParent = parent + "." + message.Name
		}
		appendEnums(manifest, pkg, nextParent, message.Enums)
		appendNestedEnums(manifest, pkg, nextParent, message.Nested)
	}
}

func buildField(field fieldDescriptor, all, mapEntries map[string]messageDescriptor) Field {
	result := Field{
		Name:     field.Name,
		JSONName: field.JSONName,
		Number:   field.Number,
		Repeated: field.Label == 3,
		Required: field.Label == 2,
		Optional: field.Proto3Optional,
	}
	if scalar, ok := scalarType(field.Type); ok {
		result.Kind = "scalar"
		result.Type = scalar
		return result
	}
	if field.Type == 14 {
		result.Kind = "enum"
		result.Type = field.TypeName
		return result
	}
	if field.Type == 11 {
		if entry, ok := mapEntries[field.TypeName]; ok && len(entry.Fields) >= 2 {
			result.Kind = "map"
			result.Map = true
			result.Repeated = false
			key := buildField(entry.Fields[0], all, mapEntries)
			value := buildField(entry.Fields[1], all, mapEntries)
			result.MapKeyType = key.Type
			result.MapValueKind = value.Kind
			result.MapValueType = value.Type
			result.Type = "map"
			return result
		}
		result.Kind = "message"
		result.Type = field.TypeName
		return result
	}
	result.Kind = "unknown"
	result.Type = field.TypeName
	return result
}

func discoverProtoFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path != dir && strings.HasPrefix(entry.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.EqualFold(filepath.Ext(entry.Name()), ".proto") {
			rel, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}

func resolveProtoc(explicit string) (string, error) {
	candidates := []string{explicit, os.Getenv("PROTOC")}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}
	if path, err := exec.LookPath("protoc"); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("contract: protoc not found; install protoc or set PROTOC")
}

func standardProtoInclude(protoc string) string {
	resolved := protoc
	if path, err := exec.LookPath(protoc); err == nil {
		resolved = path
	}
	if path, err := filepath.EvalSymlinks(resolved); err == nil {
		resolved = path
	}
	candidates := []string{
		filepath.Join(filepath.Dir(filepath.Dir(resolved)), "include"),
		"/usr/local/include",
		"/usr/include",
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(filepath.Join(candidate, "google", "protobuf", "descriptor.proto")); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}
