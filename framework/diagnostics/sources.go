package diagnostics

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/hvritual/yunka.io/framework/core/resilience"
	"github.com/hvritual/yunka.io/pkg/contract"
	"github.com/hvritual/yunka.io/pkg/selector"
)

type ContractSnapshot struct {
	SchemaVersion int                         `json:"schemaVersion"`
	Files         int                         `json:"files"`
	Messages      int                         `json:"messages"`
	Enums         int                         `json:"enums"`
	Services      int                         `json:"services"`
	Methods       int                         `json:"methods"`
	HTTPBindings  int                         `json:"httpBindings"`
	Operations    []ContractOperationSnapshot `json:"operations,omitempty"`
}

type ContractOperationSnapshot struct {
	Name         string                 `json:"name"`
	RPCPath      string                 `json:"rpcPath"`
	Request      string                 `json:"request"`
	Response     string                 `json:"response"`
	HTTP         []contract.HTTPBinding `json:"http,omitempty"`
	ClientStream bool                   `json:"clientStreaming,omitempty"`
	ServerStream bool                   `json:"serverStreaming,omitempty"`
}

func Contract(manifest contract.Manifest) Source {
	return SourceFunc{SourceName: "contract", Func: func(context.Context) (any, error) {
		snapshot := ContractSnapshot{
			SchemaVersion: manifest.SchemaVersion,
			Files:         len(manifest.Files),
			Messages:      len(manifest.Messages),
			Enums:         len(manifest.Enums),
			Services:      len(manifest.Services),
		}
		for _, service := range manifest.Services {
			for _, method := range service.Methods {
				snapshot.Methods++
				snapshot.HTTPBindings += len(method.HTTP)
				snapshot.Operations = append(snapshot.Operations, ContractOperationSnapshot{
					Name:         method.FullName,
					RPCPath:      "/" + strings.TrimPrefix(service.FullName, ".") + "/" + method.Name,
					Request:      method.Request,
					Response:     method.Response,
					HTTP:         append([]contract.HTTPBinding(nil), method.HTTP...),
					ClientStream: method.ClientStreaming,
					ServerStream: method.ServerStreaming,
				})
			}
		}
		sort.Slice(snapshot.Operations, func(i, j int) bool { return snapshot.Operations[i].Name < snapshot.Operations[j].Name })
		return snapshot, nil
	}}
}

type ResilienceOperationSnapshot struct {
	Key      string                       `json:"key"`
	Active   bool                         `json:"active"`
	Circuit  resilience.CircuitSnapshot   `json:"circuit"`
	Rate     resilience.RateLimitSnapshot `json:"rateLimit"`
	LoadShed resilience.LoadShedSnapshot  `json:"loadShed"`
}

type ResilienceSnapshot struct {
	Operations []ResilienceOperationSnapshot `json:"operations"`
}

func Resilience(name string, policy *resilience.RPCPolicy, keys ...string) Source {
	keys = stableStrings(keys)
	return SourceFunc{SourceName: name, Func: func(context.Context) (any, error) {
		snapshot := ResilienceSnapshot{}
		if policy == nil {
			return snapshot, nil
		}
		for _, key := range keys {
			state, active := policy.PeekSnapshot(key)
			snapshot.Operations = append(snapshot.Operations, ResilienceOperationSnapshot{
				Key: key, Active: active, Circuit: state.Circuit, Rate: state.Rate, LoadShed: state.Load,
			})
		}
		return snapshot, nil
	}}
}

type SelectorSnapshot struct {
	Services []SelectorServiceSnapshot `json:"services"`
}

type SelectorServiceSnapshot struct {
	Name  string                 `json:"name"`
	Nodes []SelectorNodeSnapshot `json:"nodes"`
}

type SelectorNodeSnapshot struct {
	Version             string  `json:"version,omitempty"`
	NodeID              string  `json:"nodeId,omitempty"`
	Address             string  `json:"address,omitempty"`
	EWMAMillis          float64 `json:"ewmaMillis"`
	Score               float64 `json:"score"`
	InFlight            int64   `json:"inFlight"`
	ConsecutiveFailures int     `json:"consecutiveFailures"`
	Ejected             bool    `json:"ejected"`
	EjectedUntil        string  `json:"ejectedUntil,omitempty"`
	EjectionCount       int     `json:"ejectionCount"`
	LastOutcome         string  `json:"lastOutcome"`
	Selections          uint64  `json:"selections"`
	Successes           uint64  `json:"successes"`
	Failures            uint64  `json:"failures"`
	Ignored             uint64  `json:"ignored"`
}

func Selector(name string, snapshotter selector.Snapshotter, services ...string) Source {
	services = stableStrings(services)
	return SourceFunc{SourceName: name, Func: func(context.Context) (any, error) {
		result := SelectorSnapshot{}
		if snapshotter == nil {
			return result, nil
		}
		for _, service := range services {
			entry := SelectorServiceSnapshot{Name: service}
			for _, node := range snapshotter.Snapshot(service) {
				converted := SelectorNodeSnapshot{
					Version: node.Version, NodeID: node.NodeID, Address: node.Address,
					EWMAMillis: float64(node.EWMA) / float64(time.Millisecond),
					Score:      node.Score, InFlight: node.InFlight,
					ConsecutiveFailures: node.ConsecutiveFailures, Ejected: node.Ejected,
					EjectionCount: node.EjectionCount, LastOutcome: outcomeName(node.LastOutcome),
					Selections: node.Selections, Successes: node.Successes, Failures: node.Failures, Ignored: node.Ignored,
				}
				if !node.EjectedUntil.IsZero() {
					converted.EjectedUntil = node.EjectedUntil.UTC().Format(time.RFC3339Nano)
				}
				entry.Nodes = append(entry.Nodes, converted)
			}
			result.Services = append(result.Services, entry)
		}
		return result, nil
	}}
}

func stableStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func outcomeName(outcome selector.Outcome) string {
	switch outcome {
	case selector.OutcomeSuccess:
		return "success"
	case selector.OutcomeFailure:
		return "failure"
	case selector.OutcomeIgnore:
		return "ignore"
	case selector.OutcomeEject:
		return "eject"
	default:
		return "auto"
	}
}
