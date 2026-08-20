package devruntime

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	applicationgraph "yunka.io/pkg/applicationgraph"
	"yunka.io/pkg/contract"
)

type CheckStatus string

const (
	CheckPass CheckStatus = "pass"
	CheckWarn CheckStatus = "warn"
	CheckFail CheckStatus = "fail"
)

type Check struct {
	Name   string      `json:"name"`
	Status CheckStatus `json:"status"`
	Detail string      `json:"detail,omitempty"`
	Action string      `json:"action,omitempty"`
}

type DoctorReport struct {
	Root   string  `json:"root"`
	Checks []Check `json:"checks"`
}

func (report DoctorReport) Failed(strict bool) bool {
	for _, check := range report.Checks {
		if check.Status == CheckFail || (strict && check.Status == CheckWarn) {
			return true
		}
	}
	return false
}

type DoctorOptions struct {
	Root     string
	LookPath func(string) (string, error)
	Run      func(context.Context, string, ...string) (string, error)
}

func Doctor(ctx context.Context, options DoctorOptions) DoctorReport {
	if ctx == nil {
		ctx = context.Background()
	}
	root := strings.TrimSpace(options.Root)
	if root == "" {
		root = "."
	}
	if absolute, err := filepath.Abs(root); err == nil {
		root = absolute
	}
	lookPath := options.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	run := options.Run
	if run == nil {
		run = func(ctx context.Context, name string, args ...string) (string, error) {
			command := exec.CommandContext(ctx, name, args...)
			output, err := command.CombinedOutput()
			return strings.TrimSpace(string(output)), err
		}
	}
	report := DoctorReport{Root: root}
	add := func(check Check) { report.Checks = append(report.Checks, check) }

	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		add(Check{Name: "workspace.root", Status: CheckFail, Detail: "project root is not a directory", Action: "run yunka doctor from the repository root or pass --root"})
		return report
	}
	add(Check{Name: "workspace.root", Status: CheckPass, Detail: root})

	goWork := filepath.Join(root, "go.work")
	requiredGo, err := requiredToolchain(goWork)
	if err != nil {
		add(Check{Name: "workspace.go_work", Status: CheckFail, Detail: err.Error(), Action: "restore the repository go.work file"})
	} else {
		add(Check{Name: "workspace.go_work", Status: CheckPass, Detail: "toolchain " + requiredGo})
	}

	lockPath := filepath.Join(root, "tools", "toolchain.env")
	lock, lockErr := loadToolchainLock(lockPath)
	if lockErr != nil {
		add(Check{Name: "toolchain.lock", Status: CheckFail, Detail: lockErr.Error(), Action: "restore tools/toolchain.env from the repository"})
	} else if requiredGo != "" && strings.TrimPrefix(requiredGo, "go") != lock.GoVersion {
		add(Check{Name: "toolchain.lock", Status: CheckFail, Detail: fmt.Sprintf("go.work=%s lock=go%s", requiredGo, lock.GoVersion), Action: "make go.work and tools/toolchain.env agree exactly"})
	} else {
		add(Check{Name: "toolchain.lock", Status: CheckPass, Detail: fmt.Sprintf("go=%s protoc=%s govulncheck=%s", lock.GoVersion, lock.ProtocVersion, lock.GovulncheckVersion)})
	}

	checkTool := func(name, versionArg, expected, action string) {
		path, pathErr := lookPath(name)
		if pathErr != nil {
			add(Check{Name: "tool." + name, Status: CheckFail, Detail: "not found", Action: action})
			return
		}
		output, runErr := run(ctx, path, versionArg)
		if runErr != nil {
			add(Check{Name: "tool." + name, Status: CheckFail, Detail: runErr.Error(), Action: action})
			return
		}
		if expected != "" {
			current := extractVersion(output)
			if current != expected {
				add(Check{Name: "tool." + name, Status: CheckFail, Detail: firstLine(output), Action: fmt.Sprintf("install %s %s exactly", name, expected)})
				return
			}
		}
		add(Check{Name: "tool." + name, Status: CheckPass, Detail: firstLine(output)})
	}

	goExpected := strings.TrimPrefix(requiredGo, "go")
	protocExpected := "3.21.12"
	if lockErr == nil {
		goExpected = lock.GoVersion
		protocExpected = lock.ProtocVersion
	}
	if goExpected == "" {
		goExpected = "1.25.13"
	}
	checkTool("go", "version", goExpected, "install the exact Go toolchain locked by tools/toolchain.env")
	checkTool("protoc", "--version", protocExpected, "install the exact protoc release locked by tools/toolchain.env")
	checkTool("gcc", "--version", "", "install GCC for race/CGO verification")
	checkTool("git", "--version", "", "install Git")

	manifestPath := filepath.Join(root, "contracts", "generated", "manifest.json")
	manifest, err := contract.LoadManifest(manifestPath)
	if err != nil {
		add(Check{Name: "contract.manifest", Status: CheckFail, Detail: err.Error(), Action: "run yunka contract generate and commit deterministic artifacts"})
	} else {
		builder := applicationgraph.NewBuilder()
		graphErr := applicationgraph.AddContract(builder, manifest)
		if graphErr == nil {
			_, graphErr = builder.Build()
		}
		if graphErr != nil {
			add(Check{Name: "application_graph.contract", Status: CheckFail, Detail: graphErr.Error(), Action: "fix contract graph consistency before development"})
		} else {
			add(Check{Name: "contract.manifest", Status: CheckPass, Detail: fmt.Sprintf("services=%d messages=%d", len(manifest.Services), len(manifest.Messages))})
			add(Check{Name: "application_graph.contract", Status: CheckPass, Detail: "contract graph compiles"})
		}
	}

	if gitPath, err := lookPath("git"); err == nil {
		output, statusErr := run(ctx, gitPath, "-C", root, "status", "--porcelain")
		if statusErr != nil {
			add(Check{Name: "git.status", Status: CheckWarn, Detail: statusErr.Error(), Action: "check repository status manually"})
		} else if strings.TrimSpace(output) != "" {
			add(Check{Name: "git.status", Status: CheckWarn, Detail: "working tree has local changes", Action: "review git status before broad generation or cleanup commands"})
		} else {
			add(Check{Name: "git.status", Status: CheckPass, Detail: "working tree clean"})
		}
	}

	devConfig := filepath.Join(root, ".yunka", "dev.json")
	if _, err := os.Stat(devConfig); err == nil {
		add(Check{Name: "dev.manifest", Status: CheckPass, Detail: devConfig})
	} else {
		add(Check{Name: "dev.manifest", Status: CheckWarn, Detail: "optional .yunka/dev.json not found", Action: "create it when yunka dev should manage local processes"})
	}
	return report
}

type toolchainLock struct {
	GoVersion              string
	ProtocRelease          string
	ProtocVersion          string
	ProtocLinuxX8664SHA256 string
	GovulncheckVersion     string
}

func loadToolchainLock(path string) (toolchainLock, error) {
	file, err := os.Open(path)
	if err != nil {
		return toolchainLock{}, err
	}
	defer file.Close()

	var lock toolchainLock
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return toolchainLock{}, fmt.Errorf("toolchain lock contains invalid line %q", line)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		switch key {
		case "GO_VERSION":
			lock.GoVersion = value
		case "PROTOC_RELEASE":
			lock.ProtocRelease = value
		case "PROTOC_VERSION":
			lock.ProtocVersion = value
		case "PROTOC_LINUX_X86_64_SHA256":
			lock.ProtocLinuxX8664SHA256 = strings.ToLower(value)
		case "GOVULNCHECK_VERSION":
			lock.GovulncheckVersion = value
		default:
			return toolchainLock{}, fmt.Errorf("toolchain lock contains unknown key %q", key)
		}
	}
	if err := scanner.Err(); err != nil {
		return toolchainLock{}, err
	}
	if lock.GoVersion == "" || lock.ProtocRelease == "" || lock.ProtocVersion == "" || lock.ProtocLinuxX8664SHA256 == "" || lock.GovulncheckVersion == "" {
		return toolchainLock{}, fmt.Errorf("toolchain lock is incomplete")
	}
	if matched, _ := regexp.MatchString(`^[0-9a-f]{64}$`, lock.ProtocLinuxX8664SHA256); !matched {
		return toolchainLock{}, fmt.Errorf("toolchain lock protoc sha256 is invalid")
	}
	return lock, nil
}

func requiredToolchain(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	re := regexp.MustCompile(`(?m)^\s*toolchain\s+(go[0-9]+(?:\.[0-9]+){1,2})\s*$`)
	match := re.FindStringSubmatch(string(data))
	if len(match) == 2 {
		return match[1], nil
	}
	return "", fmt.Errorf("go.work does not declare a toolchain")
}

var versionRE = regexp.MustCompile(`([0-9]+(?:\.[0-9]+){1,2})`)

func extractVersion(value string) string {
	match := versionRE.FindStringSubmatch(value)
	if len(match) == 2 {
		return match[1]
	}
	return ""
}

func compareVersion(left, right string) int {
	l, r := versionParts(left), versionParts(right)
	for len(l) < 3 {
		l = append(l, 0)
	}
	for len(r) < 3 {
		r = append(r, 0)
	}
	for i := 0; i < 3; i++ {
		if l[i] < r[i] {
			return -1
		}
		if l[i] > r[i] {
			return 1
		}
	}
	return 0
}

func versionParts(value string) []int {
	parts := strings.Split(value, ".")
	result := make([]int, 0, len(parts))
	for _, part := range parts {
		number, _ := strconv.Atoi(part)
		result = append(result, number)
	}
	return result
}

func firstLine(value string) string {
	if index := strings.IndexByte(value, '\n'); index >= 0 {
		return strings.TrimSpace(value[:index])
	}
	return strings.TrimSpace(value)
}
