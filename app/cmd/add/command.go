package add

import (
	"fmt"
	"strings"

	"github.com/hvritual/yunka.io/pkg/diagnostic"
	"github.com/urfave/cli"
)

const (
	AppName       = "add"
	SchemaVersion = 1

	FormatText      = "text"
	FormatJSON      = "json"
	FormatAgentJSON = "agent-json"
)

type Mutation struct {
	Path   string `json:"path"`
	Action string `json:"action"`
	Owner  string `json:"owner"`
}

type Effect struct {
	Stage       string `json:"stage"`
	Path        string `json:"path,omitempty"`
	Scope       string `json:"scope,omitempty"`
	Conditional bool   `json:"conditional,omitempty"`
	Reason      string `json:"reason"`
}

type NextAction struct {
	Command string `json:"command"`
	Purpose string `json:"purpose"`
}

type OperationHTTPSemantics struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	Body   string `json:"body,omitempty"`
}

type OperationSemantics struct {
	UseCase             string                  `json:"useCase"`
	Access              string                  `json:"access"`
	Permissions         []string                `json:"permissions"`
	PermissionMode      string                  `json:"permissionMode,omitempty"`
	Tenant              string                  `json:"tenant"`
	Authentication      []string                `json:"authentication"`
	Transaction         string                  `json:"transaction"`
	Idempotency         string                  `json:"idempotency"`
	Composition         string                  `json:"composition"`
	RequiresOperations  []string                `json:"requiresOperations"`
	HTTP                *OperationHTTPSemantics `json:"http,omitempty"`
}

type Report struct {
	SchemaVersion     int                 `json:"schemaVersion"`
	Kind              string              `json:"kind"`
	Identity          map[string]string   `json:"identity"`
	Mutations         []Mutation          `json:"mutations"`
	Effects           []Effect            `json:"generatedEffects,omitempty"`
	ExplicitSemantics *OperationSemantics `json:"explicitSemantics,omitempty"`
	NextActions       []NextAction        `json:"nextActions,omitempty"`
	Notes             []string            `json:"notes,omitempty"`
}

type ApplicationOptions struct {
	Root   string
	Key    string
	Source string
}

type OperationOptions struct {
	Root               string
	ApplicationKey     string
	OperationID        string
	Source             string
	UseCase            string
	RPCName            string
	RequestType        string
	ResponseType       string
	Access             string
	Permissions        []string
	PermissionMode     string
	Tenant             string
	Authentication     []string
	Transaction        string
	Idempotency        string
	Composition        string
	RequiresOperations []string
	HTTPMethod         string
	HTTPPath           string
	HTTPBody           string
}

type EventOptions struct {
	Root    string
	Domain  string
	Name    string
	Message string
	Source  string
}

type ModuleOptions struct {
	Root      string
	Name      string
	Version   string
	ConfigKey string
	Logger    bool
	Databases []string
	EventBus  bool
	RPC       []string
	DependsOn []string
}

type FailureKind string

const (
	FailureRequest   FailureKind = "request"
	FailureSource    FailureKind = "source"
	FailureOwnership FailureKind = "ownership"
	FailureConflict  FailureKind = "conflict"
)

type Failure struct {
	Kind     FailureKind
	Location string
	Err      error
}

func (failure *Failure) Error() string {
	if failure == nil || failure.Err == nil {
		return "structural scaffold failed"
	}
	return failure.Err.Error()
}

func (failure *Failure) Unwrap() error {
	if failure == nil {
		return nil
	}
	return failure.Err
}

func Command() cli.Command {
	return cli.Command{
		Name:  AppName,
		Usage: "create developer-owned structural application artifacts without inventing business semantics",
		Subcommands: []cli.Command{
			applicationCommand(),
			operationCommand(),
			eventCommand(),
			moduleCommand(),
		},
	}
}

func applicationCommand() cli.Command {
	return cli.Command{
		Name:  "application",
		Usage: "add an empty typed Application service to an existing canonical domain contract",
		Flags: commonFlags(),
		Action: func(c *cli.Context) error {
			report, err := AddApplication(ApplicationOptions{Root: c.String("root"), Key: c.Args().Get(0), Source: c.String("source")})
			return finish(c, "yunka add application", report, err)
		},
	}
}

func operationCommand() cli.Command {
	flags := append(commonFlags(),
		cli.BoolFlag{Name: "plan", Usage: "validate and print prospective structural mutations/effects without writing files"},
		cli.StringFlag{Name: "use-case", Usage: "explicit stable use_case business key"},
		cli.StringFlag{Name: "rpc-name", Usage: "optional protobuf RPC method name; defaults structurally from operation ID"},
		cli.StringFlag{Name: "request-type", Usage: "optional request DTO message name; defaults to <RPC>Request"},
		cli.StringFlag{Name: "response-type", Usage: "optional response DTO message name; defaults to <RPC>Response"},
		cli.StringFlag{Name: "access", Usage: "required: public or protected"},
		cli.StringSliceFlag{Name: "permission", Usage: "explicit permission key; repeatable for protected operations"},
		cli.StringFlag{Name: "permission-mode", Usage: "required for protected operations: all or any"},
		cli.StringFlag{Name: "tenant", Usage: "required: required or optional"},
		cli.StringSliceFlag{Name: "authentication", Usage: "explicit authentication mode: jwt, api-key, or service; repeatable"},
		cli.StringFlag{Name: "transaction", Usage: "required: none, read-only, or local"},
		cli.StringFlag{Name: "idempotency", Usage: "required: none or required"},
		cli.StringFlag{Name: "composition", Usage: "required: none, local, or remote-saga"},
		cli.StringSliceFlag{Name: "requires-operation", Usage: "explicit canonical operation dependency; repeatable"},
		cli.StringFlag{Name: "http-method", Usage: "optional explicit HTTP method"},
		cli.StringFlag{Name: "http-path", Usage: "optional explicit HTTP path"},
		cli.StringFlag{Name: "http-body", Usage: "optional HTTP body mapping; typically *"},
	)
	return cli.Command{
		Name:  "operation",
		Usage: "add a typed RPC Operation using only explicit semantic facts supplied by the caller",
		Flags: flags,
		Action: func(c *cli.Context) error {
			options := OperationOptions{
				Root:               c.String("root"),
				ApplicationKey:     c.Args().Get(0),
				OperationID:        c.Args().Get(1),
				Source:             c.String("source"),
				UseCase:            c.String("use-case"),
				RPCName:            c.String("rpc-name"),
				RequestType:        c.String("request-type"),
				ResponseType:       c.String("response-type"),
				Access:             c.String("access"),
				Permissions:        c.StringSlice("permission"),
				PermissionMode:     c.String("permission-mode"),
				Tenant:             c.String("tenant"),
				Authentication:     c.StringSlice("authentication"),
				Transaction:        c.String("transaction"),
				Idempotency:        c.String("idempotency"),
				Composition:        c.String("composition"),
				RequiresOperations: c.StringSlice("requires-operation"),
				HTTPMethod:         c.String("http-method"),
				HTTPPath:           c.String("http-path"),
				HTTPBody:           c.String("http-body"),
			}
			if c.Bool("plan") {
				report, err := PlanOperation(options)
				return finish(c, "yunka add operation --plan", report, err)
			}
			report, err := AddOperation(options)
			return finish(c, "yunka add operation", report, err)
		},
	}
}

func eventCommand() cli.Command {
	flags := append(commonFlags(), cli.StringFlag{Name: "message", Usage: "optional protobuf event message name; defaults structurally from event name"})
	return cli.Command{
		Name:  "event",
		Usage: "add a DTO_EVENT message skeleton without declaring publication, broker, Outbox, or delivery semantics",
		Flags: flags,
		Action: func(c *cli.Context) error {
			report, err := AddEvent(EventOptions{Root: c.String("root"), Domain: c.Args().Get(0), Name: c.Args().Get(1), Message: c.String("message"), Source: c.String("source")})
			return finish(c, "yunka add event", report, err)
		},
	}
}

func moduleCommand() cli.Command {
	return cli.Command{
		Name:  "module",
		Usage: "add a declarative module skeleton; capabilities are included only when explicitly requested",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "root", Value: ".", Usage: "project root"},
			cli.StringFlag{Name: "format", Value: FormatText, Usage: "output format: text, json, or agent-json"},
			cli.StringFlag{Name: "version", Value: "v0.1.0", Usage: "module contract version"},
			cli.StringFlag{Name: "config-key", Usage: "explicit configuration key"},
			cli.BoolFlag{Name: "logger", Usage: "explicitly require logger capability"},
			cli.StringSliceFlag{Name: "database", Usage: "explicit named GORM database; repeatable"},
			cli.BoolFlag{Name: "event-bus", Usage: "explicitly require application event bus"},
			cli.StringSliceFlag{Name: "rpc", Usage: "explicit named gRPC connection; repeatable"},
			cli.StringSliceFlag{Name: "depends-on", Usage: "explicit module dependency; repeatable"},
		},
		Action: func(c *cli.Context) error {
			report, err := AddModule(ModuleOptions{
				Root:      c.String("root"),
				Name:      c.Args().Get(0),
				Version:   c.String("version"),
				ConfigKey: c.String("config-key"),
				Logger:    c.Bool("logger"),
				Databases: c.StringSlice("database"),
				EventBus:  c.Bool("event-bus"),
				RPC:       c.StringSlice("rpc"),
				DependsOn: c.StringSlice("depends-on"),
			})
			return finish(c, "yunka add module", report, err)
		},
	}
}

func commonFlags() []cli.Flag {
	return []cli.Flag{
		cli.StringFlag{Name: "root", Value: ".", Usage: "project root"},
		cli.StringFlag{Name: "source", Usage: "exact project-relative canonical .proto source; required when source selection is ambiguous"},
		cli.StringFlag{Name: "format", Value: FormatText, Usage: "output format: text, json, or agent-json"},
	}
}

func finish(c *cli.Context, command string, report Report, err error) error {
	format := strings.ToLower(strings.TrimSpace(c.String("format")))
	if format == "" {
		format = FormatText
	}
	if format != FormatText && format != FormatJSON && format != FormatAgentJSON {
		item := diagnostic.MustDefinition(diagnostic.CodeUnsupportedOutputFormat).Diagnostic(diagnostic.SeverityError)
		item.Detail = fmt.Sprintf("format %q is unsupported; use text, json, or agent-json", format)
		return printFailure(command, format, item, 2)
	}
	if err != nil {
		return printFailure(command, format, Diagnose(err), 1)
	}
	output, err := Render(report, format)
	if err != nil {
		return err
	}
	fmt.Print(output)
	return nil
}
