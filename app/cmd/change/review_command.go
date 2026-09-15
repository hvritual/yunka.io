package change

import (
	"context"
	"fmt"
	"strings"

	"github.com/urfave/cli"
)

func reviewCommand() cli.Command {
	return cli.Command{
		Name:        "review",
		Usage:       "build, reconcile, and govern exact-candidate human review evidence",
		Subcommands: []cli.Command{reviewBuildCommand(), reviewCheckCommand(), qualityWaiverCommand()},
	}
}

func reviewBuildCommand() cli.Command {
	return cli.Command{
		Name:  "build",
		Usage: "project declared intent and verified change evidence before raw-diff review",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "root", Value: ".", Usage: "project root"},
			sourceProtocFlag(), sourceIncludesFlag(),
			cli.StringFlag{Name: "contract", Value: DefaultChangeContractPath, Usage: "change contract path"},
			cli.StringFlag{Name: "attestation", Value: DefaultChangeAttestationPath, Usage: "verified change attestation path"},
			cli.StringFlag{Name: "output", Value: DefaultReviewPacketPath, Usage: "review packet output path"},
			cli.StringFlag{Name: "problem", Usage: "problem the change is intended to solve"},
			cli.StringSliceFlag{Name: "current-concept", Usage: "current concept/responsibility; may be repeated"},
			cli.StringSliceFlag{Name: "desired-ownership", Usage: "desired ownership/responsibility; may be repeated"},
			cli.StringFlag{Name: "why", Usage: "why the change is needed"},
			cli.StringFlag{Name: "what", Usage: "what the change does"},
			cli.StringFlag{Name: "boundary", Usage: "what the change intentionally does not change"},
			cli.StringSliceFlag{Name: "invariant", Usage: "declared affected invariant; may be repeated"},
			cli.StringSliceFlag{Name: "risk", Usage: "declared review risk; may be repeated"},
			cli.StringSliceFlag{Name: "unresolved", Usage: "declared unresolved finding; may be repeated"},
			cli.StringFlag{Name: "format", Value: FormatText, Usage: "output format: text, json, or agent-json"},
		},
		Action: func(c *cli.Context) error {
			format := strings.ToLower(strings.TrimSpace(c.String("format")))
			if format != FormatText && format != FormatJSON && format != FormatAgentJSON {
				return fmt.Errorf("change review build: unsupported format %q", format)
			}
			packet, root, err := BuildReviewPacket(context.Background(), sourceCompilerOptions(c), c.String("contract"), c.String("attestation"), ReviewNarrative{
				Problem:            c.String("problem"),
				CurrentConcepts:    c.StringSlice("current-concept"),
				DesiredOwnership:   c.StringSlice("desired-ownership"),
				Why:                c.String("why"),
				What:               c.String("what"),
				Boundary:           c.String("boundary"),
				AffectedInvariants: c.StringSlice("invariant"),
				Risks:              c.StringSlice("risk"),
				UnresolvedFindings: c.StringSlice("unresolved"),
			})
			if err != nil {
				return err
			}
			path, err := WriteReviewPacket(root, c.String("output"), packet)
			if err != nil {
				return err
			}
			output, err := RenderReviewPacket(packet, path, format)
			if err != nil {
				return err
			}
			fmt.Print(output)
			return nil
		},
	}
}

func reviewCheckCommand() cli.Command {
	return cli.Command{
		Name:  "check",
		Usage: "reject a stale or tampered review packet against the exact current candidate",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "root", Value: ".", Usage: "project root"},
			sourceProtocFlag(), sourceIncludesFlag(),
			cli.StringFlag{Name: "contract", Value: DefaultChangeContractPath, Usage: "change contract path"},
			cli.StringFlag{Name: "attestation", Value: DefaultChangeAttestationPath, Usage: "verified change attestation path"},
			cli.StringFlag{Name: "packet", Value: DefaultReviewPacketPath, Usage: "review packet path"},
			cli.StringFlag{Name: "format", Value: FormatText, Usage: "output format: text, json, or agent-json"},
		},
		Action: func(c *cli.Context) error {
			packet, err := CheckReviewPacket(context.Background(), sourceCompilerOptions(c), c.String("contract"), c.String("attestation"), c.String("packet"))
			if err != nil {
				return err
			}
			output, err := RenderReviewPacket(packet, c.String("packet"), c.String("format"))
			if err != nil {
				return err
			}
			fmt.Print(output)
			return nil
		},
	}
}
