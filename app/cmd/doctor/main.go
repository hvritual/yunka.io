package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/urfave/cli"
	"yunka.io/pkg/devruntime"
)

const AppName = "doctor"

func Command() cli.Command {
	return cli.Command{
		Name:  AppName,
		Usage: "check the yunka developer environment without mutating it",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "root", Value: "."},
			cli.StringFlag{Name: "format", Value: "text", Usage: "text or json"},
			cli.BoolFlag{Name: "strict", Usage: "treat warnings as failures"},
		},
		Action: func(c *cli.Context) error {
			report := devruntime.Doctor(context.Background(), devruntime.DoctorOptions{Root: c.String("root")})
			if strings.EqualFold(c.String("format"), "json") {
				encoder := json.NewEncoder(os.Stdout)
				encoder.SetIndent("", "  ")
				if err := encoder.Encode(report); err != nil {
					return err
				}
			} else {
				for _, check := range report.Checks {
					fmt.Printf("%-5s %-28s %s", strings.ToUpper(string(check.Status)), check.Name, check.Detail)
					if check.Action != "" {
						fmt.Printf("\n      action: %s", check.Action)
					}
					fmt.Println()
				}
			}
			if report.Failed(c.Bool("strict")) {
				return cli.NewExitError("yunka doctor found blocking checks", 1)
			}
			return nil
		},
	}
}
