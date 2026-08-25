package domain

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	domainProtocVersion        = "3.21.12"
	domainProtocGenGoVersion   = "1.36.11"
	domainProtocGenGRPCVersion = "1.6.2"
)

func attachPinnedRPCGenerated(files map[string]string, projectRoot string) error {
	proto, ok := files["transport/rpc/domain.proto"]
	if !ok {
		return errors.New("domain: RPC enabled but domain.proto was not rendered")
	}
	temporary, err := os.MkdirTemp("", ".yunka-domain-rpc-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporary)
	rpcRoot := filepath.Join(temporary, "rpc")
	if err := os.MkdirAll(rpcRoot, 0o750); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(rpcRoot, "domain.proto"), []byte(proto), 0o640); err != nil {
		return err
	}
	protoc, include, err := resolvePinnedProtoc()
	if err != nil {
		return err
	}
	toolDir := strings.TrimSpace(os.Getenv("YUNKA_DOMAIN_TOOL_DIR"))
	if toolDir == "" {
		toolDir = filepath.Join(projectRoot, ".yunka", "bin")
	}
	if err := os.MkdirAll(toolDir, 0o750); err != nil {
		return err
	}
	goPlugin, err := ensurePinnedGoTool(toolDir, "protoc-gen-go", "google.golang.org/protobuf/cmd/protoc-gen-go@v"+domainProtocGenGoVersion, domainProtocGenGoVersion)
	if err != nil {
		return err
	}
	grpcPlugin, err := ensurePinnedGoTool(toolDir, "protoc-gen-go-grpc", "google.golang.org/grpc/cmd/protoc-gen-go-grpc@v"+domainProtocGenGRPCVersion, domainProtocGenGRPCVersion)
	if err != nil {
		return err
	}
	pbRoot := filepath.Join(rpcRoot, "pb")
	if err := os.MkdirAll(pbRoot, 0o750); err != nil {
		return err
	}
	args := []string{"-I", rpcRoot}
	if include != "" {
		args = append(args, "-I", include)
	}
	args = append(args,
		"--plugin=protoc-gen-go="+goPlugin,
		"--plugin=protoc-gen-go-grpc="+grpcPlugin,
		"--go_out="+pbRoot, "--go_opt=paths=source_relative",
		"--go-grpc_out="+pbRoot, "--go-grpc_opt=paths=source_relative",
		"domain.proto",
	)
	command := exec.Command(protoc, args...)
	command.Dir = rpcRoot
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("domain: pinned protoc generation failed: %w\n%s", err, output)
	}
	for _, name := range []string{"domain.pb.go", "domain_grpc.pb.go"} {
		contents, err := os.ReadFile(filepath.Join(pbRoot, name))
		if err != nil {
			return fmt.Errorf("domain: read generated %s: %w", name, err)
		}
		files["transport/rpc/pb/"+name] = string(contents)
	}
	return nil
}

func resolvePinnedProtoc() (string, string, error) {
	binary := strings.TrimSpace(os.Getenv("PROTOC"))
	if binary == "" {
		var err error
		binary, err = exec.LookPath("protoc")
		if err != nil {
			return "", "", errors.New("domain: pinned protoc 3.21.12 is required; install the repository toolchain or set PROTOC")
		}
	}
	output, err := exec.Command(binary, "--version").CombinedOutput()
	if err != nil {
		return "", "", fmt.Errorf("domain: run protoc --version: %w", err)
	}
	actual := strings.TrimSpace(string(output))
	want := "libprotoc " + domainProtocVersion
	if actual != want {
		return "", "", fmt.Errorf("domain: protoc=%q want %q", actual, want)
	}
	include := strings.TrimSpace(os.Getenv("YUNKA_PROTOC_INCLUDE"))
	if include == "" {
		candidate := filepath.Clean(filepath.Join(filepath.Dir(binary), "..", "include"))
		if _, err := os.Stat(filepath.Join(candidate, "google", "protobuf", "timestamp.proto")); err == nil {
			include = candidate
		}
	}
	if include == "" {
		for _, candidate := range []string{"/usr/local/include", "/usr/include"} {
			if _, err := os.Stat(filepath.Join(candidate, "google", "protobuf", "timestamp.proto")); err == nil {
				include = candidate
				break
			}
		}
	}
	if include == "" {
		return "", "", errors.New("domain: google/protobuf include directory is required; set YUNKA_PROTOC_INCLUDE")
	}
	return binary, include, nil
}

func ensurePinnedGoTool(dir, name, moduleVersion, expected string) (string, error) {
	suffix := ""
	if os.PathSeparator == '\\' {
		suffix = ".exe"
	}
	path := filepath.Join(dir, name+suffix)
	if output, err := exec.Command(path, "--version").CombinedOutput(); err == nil && strings.Contains(string(output), expected) {
		return path, nil
	}
	command := exec.Command("go", "install", moduleVersion)
	command.Env = append(os.Environ(), "GOBIN="+dir)
	if output, err := command.CombinedOutput(); err != nil {
		return "", fmt.Errorf("domain: install pinned %s: %w\n%s", name, err, output)
	}
	output, err := exec.Command(path, "--version").CombinedOutput()
	if err != nil || !strings.Contains(string(output), expected) {
		return "", fmt.Errorf("domain: %s version verification failed: %s", name, output)
	}
	return path, nil
}
