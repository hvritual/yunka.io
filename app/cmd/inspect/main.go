package inspect

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/urfave/cli"
	"yunka.io/pkg/contract"
)

const AppName = "inspect"

func Command() cli.Command {
	return cli.Command{
		Name:  AppName,
		Usage: "inspect yunka runtime and contract diagnostics without mutating state",
		Subcommands: []cli.Command{
			runtimeCommand(),
			contractCommand(),
		},
	}
}

func runtimeCommand() cli.Command {
	return cli.Command{
		Name:  "runtime",
		Usage: "read a live yunka diagnostics endpoint",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "url", Value: "http://127.0.0.1:16667/_yunka/diagnostics", Usage: "diagnostics endpoint URL"},
			cli.StringFlag{Name: "token", EnvVar: "YUNKA_DIAGNOSTICS_TOKEN", Usage: "optional diagnostics Bearer token"},
			cli.DurationFlag{Name: "timeout", Value: 5 * time.Second, Usage: "request timeout"},
			cli.StringFlag{Name: "format", Value: "text", Usage: "text or json"},
		},
		Action: func(c *cli.Context) error {
			report, raw, err := loadRuntime(c.String("url"), c.String("token"), c.Duration("timeout"))
			if err != nil {
				return err
			}
			if strings.EqualFold(c.String("format"), "json") {
				var value any
				if err := json.Unmarshal(raw, &value); err != nil {
					return err
				}
				return writeJSON(os.Stdout, value)
			}
			printRuntime(report)
			return nil
		},
	}
}

func contractCommand() cli.Command {
	return cli.Command{
		Name:  "contract",
		Usage: "inspect a committed W06 manifest without recompiling protobuf",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "manifest", Value: "contracts/generated/manifest.json", Usage: "contract manifest path"},
			cli.StringFlag{Name: "format", Value: "text", Usage: "text or json"},
		},
		Action: func(c *cli.Context) error {
			manifest, err := contract.LoadManifest(c.String("manifest"))
			if err != nil {
				return err
			}
			summary := summarizeContract(manifest)
			if strings.EqualFold(c.String("format"), "json") {
				return writeJSON(os.Stdout, summary)
			}
			fmt.Printf("contract schema=%d files=%d messages=%d enums=%d services=%d methods=%d httpBindings=%d\n",
				summary.SchemaVersion, summary.Files, summary.Messages, summary.Enums, summary.Services, summary.Methods, summary.HTTPBindings)
			for _, operation := range summary.Operations {
				fmt.Printf("- %s %s", operation.Name, operation.RPCPath)
				if len(operation.HTTP) > 0 {
					fmt.Printf(" [%s]", strings.Join(operation.HTTP, ", "))
				}
				fmt.Println()
			}
			return nil
		},
	}
}

type runtimeReport struct {
	SchemaVersion int `json:"schemaVersion"`
	Core          struct {
		State  string `json:"state"`
		Health struct {
			State  string `json:"state"`
			Live   bool   `json:"live"`
			Ready  bool   `json:"ready"`
			Checks []struct {
				Name   string `json:"name"`
				Status string `json:"status"`
				Error  string `json:"error,omitempty"`
			} `json:"checks"`
		} `json:"health"`
		Modules []struct {
			Name string `json:"name"`
		} `json:"modules"`
		Routes  []string `json:"routes"`
		Runtime struct {
			RouteCount          int  `json:"routeCount"`
			RPCClientConfigured bool `json:"rpcClientConfigured"`
			RPCServerCount      int  `json:"rpcServerCount"`
			EventBusConfigured  bool `json:"eventBusConfigured"`
		} `json:"runtime"`
	} `json:"core"`
	Components []struct {
		Name   string          `json:"name"`
		Status string          `json:"status"`
		Error  string          `json:"error,omitempty"`
		Data   json.RawMessage `json:"data,omitempty"`
	} `json:"components"`
}

func loadRuntime(endpoint, token string, timeout time.Duration) (runtimeReport, []byte, error) {
	var report runtimeReport
	parsed, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil {
		return report, nil, err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return report, nil, fmt.Errorf("inspect: unsupported diagnostics URL scheme %q", parsed.Scheme)
	}
	if parsed.Host == "" {
		return report, nil, errors.New("inspect: diagnostics URL host is required")
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	request, err := http.NewRequest(http.MethodGet, parsed.String(), nil)
	if err != nil {
		return report, nil, err
	}
	if token = strings.TrimSpace(token); token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{Timeout: timeout}
	response, err := client.Do(request)
	if err != nil {
		return report, nil, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return report, nil, err
	}
	if response.StatusCode != http.StatusOK {
		return report, nil, fmt.Errorf("inspect: diagnostics endpoint returned %s: %s", response.Status, strings.TrimSpace(string(body)))
	}
	if err := json.Unmarshal(body, &report); err != nil {
		return report, nil, err
	}
	return report, body, nil
}

func printRuntime(report runtimeReport) {
	fmt.Printf("runtime state=%s live=%v ready=%v routes=%d modules=%d rpcClient=%v rpcServers=%d eventBus=%v\n",
		report.Core.State, report.Core.Health.Live, report.Core.Health.Ready,
		report.Core.Runtime.RouteCount, len(report.Core.Modules), report.Core.Runtime.RPCClientConfigured,
		report.Core.Runtime.RPCServerCount, report.Core.Runtime.EventBusConfigured)
	for _, check := range report.Core.Health.Checks {
		fmt.Printf("health %-32s %s", check.Name, check.Status)
		if check.Error != "" {
			fmt.Printf(" (%s)", check.Error)
		}
		fmt.Println()
	}
	if len(report.Core.Modules) > 0 {
		names := make([]string, 0, len(report.Core.Modules))
		for _, module := range report.Core.Modules {
			names = append(names, module.Name)
		}
		fmt.Printf("modules: %s\n", strings.Join(names, ", "))
	}
	for _, route := range report.Core.Routes {
		fmt.Printf("route %s\n", route)
	}
	for _, component := range report.Components {
		fmt.Printf("component %-24s %s", component.Name, component.Status)
		if component.Error != "" {
			fmt.Printf(" (%s)", component.Error)
		}
		fmt.Println()
	}
}

type contractSummary struct {
	SchemaVersion int                        `json:"schemaVersion"`
	Files         int                        `json:"files"`
	Messages      int                        `json:"messages"`
	Enums         int                        `json:"enums"`
	Services      int                        `json:"services"`
	Methods       int                        `json:"methods"`
	HTTPBindings  int                        `json:"httpBindings"`
	Operations    []contractOperationSummary `json:"operations"`
}

type contractOperationSummary struct {
	Name    string   `json:"name"`
	RPCPath string   `json:"rpcPath"`
	HTTP    []string `json:"http,omitempty"`
}

func summarizeContract(manifest contract.Manifest) contractSummary {
	manifest.Normalize()
	summary := contractSummary{SchemaVersion: manifest.SchemaVersion, Files: len(manifest.Files), Messages: len(manifest.Messages), Enums: len(manifest.Enums), Services: len(manifest.Services)}
	for _, service := range manifest.Services {
		for _, method := range service.Methods {
			summary.Methods++
			summary.HTTPBindings += len(method.HTTP)
			operation := contractOperationSummary{Name: method.FullName, RPCPath: "/" + strings.TrimPrefix(service.FullName, ".") + "/" + method.Name}
			for _, binding := range method.HTTP {
				operation.HTTP = append(operation.HTTP, strings.ToUpper(binding.Method)+" "+binding.Path)
			}
			summary.Operations = append(summary.Operations, operation)
		}
	}
	sort.Slice(summary.Operations, func(i, j int) bool { return summary.Operations[i].Name < summary.Operations[j].Name })
	return summary
}

func writeJSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
