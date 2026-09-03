package advisor

import (
	"fmt"
	"os"
	"strings"

	"github.com/urfave/cli"
	"yunka.io/app/cmd/advisorcore"
	"yunka.io/app/cmd/audit"
)

const AppName = "advisor"

func Command() cli.Command {
	return cli.Command{
		Name:  AppName,
		Usage: "export and validate evidence-bound external architecture advice without invoking an LLM",
		Subcommands: []cli.Command{
			requestCommand(),
			validateCommand(),
		},
	}
}

func requestCommand() cli.Command {
	return cli.Command{
		Name:  "request",
		Usage: "export a deterministic advisory request envelope from canonical Audit evidence",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "root", Value: ".", Usage: "project root"},
			cli.StringFlag{Name: "base", Usage: "optional Git ref included through the canonical Audit debt delta"},
			cli.StringFlag{Name: "format", Value: "text", Usage: "output format: text, json, or agent-json"},
		},
		Action: func(c *cli.Context) error {
			request, err := BuildRequest(c.String("root"), c.String("base"))
			if err != nil {
				return err
			}
			output, err := RenderRequest(request, c.String("format"))
			if err != nil {
				return err
			}
			fmt.Print(output)
			return nil
		},
	}
}

func validateCommand() cli.Command {
	return cli.Command{
		Name:  "validate",
		Usage: "validate an external advisor response against the exact deterministic request evidence",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "request", Usage: "path to the advisor request JSON"},
			cli.StringFlag{Name: "response", Usage: "path to the external advisor response JSON"},
			cli.StringFlag{Name: "format", Value: "text", Usage: "output format: text, json, or agent-json"},
		},
		Action: func(c *cli.Context) error {
			attestation, err := ValidateFiles(c.String("request"), c.String("response"))
			if err != nil {
				return err
			}
			output, err := RenderAttestation(attestation, c.String("format"))
			if err != nil {
				return err
			}
			fmt.Print(output)
			return nil
		},
	}
}

func BuildRequest(root, baseRef string) (advisorcore.Request, error) {
	if strings.TrimSpace(baseRef) == "" {
		auditReport, err := audit.Build(root)
		if err != nil {
			return advisorcore.Request{}, err
		}
		return advisorcore.NewRequest(auditReport)
	}
	auditReport, err := audit.BuildWithBase(root, baseRef)
	if err != nil {
		return advisorcore.Request{}, err
	}
	return advisorcore.NewRequest(auditReport)
}

func ValidateFiles(requestPath, responsePath string) (advisorcore.Attestation, error) {
	requestPath = strings.TrimSpace(requestPath)
	responsePath = strings.TrimSpace(responsePath)
	if requestPath == "" || responsePath == "" {
		return advisorcore.Attestation{}, fmt.Errorf("advisor validate: --request and --response are required")
	}
	requestBytes, err := os.ReadFile(requestPath)
	if err != nil {
		return advisorcore.Attestation{}, fmt.Errorf("advisor validate: read request: %w", err)
	}
	responseBytes, err := os.ReadFile(responsePath)
	if err != nil {
		return advisorcore.Attestation{}, fmt.Errorf("advisor validate: read response: %w", err)
	}
	request, err := advisorcore.DecodeRequest(requestBytes)
	if err != nil {
		return advisorcore.Attestation{}, err
	}
	response, err := advisorcore.DecodeResponse(responseBytes)
	if err != nil {
		return advisorcore.Attestation{}, err
	}
	return advisorcore.ValidateResponse(request, response)
}

func RenderRequest(request advisorcore.Request, format string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json", "agent-json":
		contents, err := advisorcore.MarshalRequest(request)
		if err != nil {
			return "", err
		}
		return string(contents), nil
	case "", "text":
		debt := "none"
		if request.Audit.Debt != nil {
			debt = fmt.Sprintf("existing=%d new=%d fixed=%d", len(request.Audit.Debt.Existing), len(request.Audit.Debt.New), len(request.Audit.Debt.Fixed))
		}
		return fmt.Sprintf("ADVISOR REQUEST authority=%s digest=%s findings=%d debt=%s mutationAuthorized=false\n", request.Authority, request.RequestDigest, len(request.Audit.Findings), debt), nil
	default:
		return "", fmt.Errorf("advisor request: unsupported format %q; use text, json, or agent-json", format)
	}
}

func RenderAttestation(attestation advisorcore.Attestation, format string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json", "agent-json":
		contents, err := advisorcore.MarshalAttestation(attestation)
		if err != nil {
			return "", err
		}
		return string(contents), nil
	case "", "text":
		return fmt.Sprintf("ADVISOR VALID authority=%s request=%s response=%s bindings=%d result=%s\n", attestation.Authority, attestation.RequestDigest, attestation.ResponseDigest, len(attestation.Bindings), attestation.Result), nil
	default:
		return "", fmt.Errorf("advisor validate: unsupported format %q; use text, json, or agent-json", format)
	}
}
