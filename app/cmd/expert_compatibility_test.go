package main

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/urfave/cli"
	"yunka.io/app/cmd/assembly"
	"yunka.io/app/cmd/contract"
	"yunka.io/app/cmd/dependency"
	"yunka.io/app/cmd/domain"
	"yunka.io/app/cmd/module"
)

func TestC116CExpertCommandCompatibilitySnapshots(t *testing.T) {
	tests := []struct {
		name    string
		command cli.Command
		want    string
	}{
		{
			name:    "contract",
			command: contract.Command(),
			want: `command=contract aliases=- flags=- action=false
sub=check aliases=- flags=application-import,application-out,baseline,file,out,proto-dir,proto-path,protoc,repo-root,sources,title,version action=true
sub=diff aliases=- flags=baseline,current,fail-on-breaking,format action=true
sub=generate aliases=- flags=application-import,application-out,file,out,proto-dir,proto-path,protoc,repo-root,sources,title,version action=true
sub=inspect aliases=- flags=file,proto-dir,proto-path,protoc,repo-root,sources action=true
sub=lint aliases=- flags=file,proto-dir,proto-path,protoc,repo-root,sources action=true`,
		},
		{
			name:    "assembly",
			command: assembly.Command(),
			want: `command=assembly aliases=- flags=- action=false
sub=check aliases=- flags=code-import,code-out,file,module-root,out,proto-dir,proto-path,protoc,repo-root,sources action=true
sub=generate aliases=- flags=code-import,code-out,file,module-root,out,proto-dir,proto-path,protoc,repo-root,sources action=true
sub=inspect aliases=- flags=code-import,file,module-root,proto-dir,proto-path,protoc,repo-root,sources action=true`,
		},
		{
			name:    "module",
			command: module.Command(),
			want: `command=module aliases=- flags=name action=true
sub=check aliases=- flags=root action=true
sub=new aliases=- flags=config-key,database,depends-on,event-bus,n|name,no-config,no-logger,root,rpc action=true`,
		},
		{
			name:    "domain",
			command: domain.Command(),
			want: `command=domain aliases=- flags=- action=false
sub=check aliases=- flags=root action=true
sub=generate aliases=- flags=p|path action=true
sub=new aliases=- flags=field,global,n|name,o|object,root action=true`,
		},
		{
			name:    "dependency",
			command: dependency.Command(),
			want: `command=dependency aliases=- flags=- action=false
sub=check aliases=- flags=go,policy,repo-root action=true`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := renderExpertCommandSurface(test.command); got != test.want {
				t.Fatalf("expert CLI surface drifted:\n--- got ---\n%s\n--- want ---\n%s", got, test.want)
			}
		})
	}
}

func renderExpertCommandSurface(command cli.Command) string {
	var lines []string
	lines = append(lines, fmt.Sprintf(
		"command=%s aliases=%s flags=%s action=%t",
		command.Name,
		joinOrDash(sortedStrings(command.Aliases)),
		joinOrDash(flagSurface(command.Flags)),
		command.Action != nil,
	))

	subcommands := append([]cli.Command(nil), command.Subcommands...)
	sort.Slice(subcommands, func(i, j int) bool { return subcommands[i].Name < subcommands[j].Name })
	for _, subcommand := range subcommands {
		lines = append(lines, fmt.Sprintf(
			"sub=%s aliases=%s flags=%s action=%t",
			subcommand.Name,
			joinOrDash(sortedStrings(subcommand.Aliases)),
			joinOrDash(flagSurface(subcommand.Flags)),
			subcommand.Action != nil,
		))
	}
	return strings.Join(lines, "\n")
}

func flagSurface(flags []cli.Flag) []string {
	result := make([]string, 0, len(flags))
	for _, item := range flags {
		parts := strings.Split(item.GetName(), ",")
		for index := range parts {
			parts[index] = strings.TrimSpace(parts[index])
		}
		parts = sortedStrings(parts)
		result = append(result, strings.Join(parts, "|"))
	}
	return sortedStrings(result)
}

func sortedStrings(values []string) []string {
	result := append([]string(nil), values...)
	sort.Strings(result)
	return result
}

func joinOrDash(values []string) string {
	if len(values) == 0 {
		return "-"
	}
	return strings.Join(values, ",")
}
