package operationplan

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

const SchemaVersion = 2

type Set struct {
	SchemaVersion int    `json:"schemaVersion"`
	Operations    []Plan `json:"operations"`
}

type Plan struct {
	OperationID         string      `json:"operationId"`
	Domain              string      `json:"domain"`
	Application         string      `json:"application"`
	UseCase             string      `json:"useCase"`
	RequestType         string      `json:"requestType"`
	ResponseType        string      `json:"responseType"`
	Security            Security    `json:"security"`
	Execution           Execution   `json:"execution"`
	Composition         Composition `json:"composition"`
	ApplicationRequires []string    `json:"applicationRequires,omitempty"`
	Bindings            Bindings    `json:"bindings"`
}

type Security struct {
	Public         bool     `json:"public,omitempty"`
	TenantRequired bool     `json:"tenantRequired,omitempty"`
	Authentication []string `json:"authentication,omitempty"`
	Permissions    []string `json:"permissions,omitempty"`
	PermissionMode string   `json:"permissionMode,omitempty"`
}

type Execution struct {
	Transaction string `json:"transaction"`
	Idempotency string `json:"idempotency"`
}

type Composition struct {
	Boundary           string   `json:"boundary,omitempty"`
	RequiresOperations []string `json:"requiresOperations,omitempty"`
	PermissionClosure  []string `json:"permissionClosure,omitempty"`
}

type Bindings struct {
	RPC  string        `json:"rpc,omitempty"`
	HTTP []HTTPBinding `json:"http,omitempty"`
}

type HTTPBinding struct {
	Method       string `json:"method"`
	Path         string `json:"path"`
	Body         string `json:"body,omitempty"`
	ResponseBody string `json:"responseBody,omitempty"`
}

func Normalize(set Set) Set {
	result := Set{SchemaVersion: set.SchemaVersion, Operations: make([]Plan, 0, len(set.Operations))}
	if result.SchemaVersion == 0 || result.SchemaVersion == 1 {
		result.SchemaVersion = SchemaVersion
	}
	for _, item := range set.Operations {
		item.OperationID = strings.TrimSpace(item.OperationID)
		item.Domain = strings.TrimSpace(item.Domain)
		item.Application = strings.TrimSpace(item.Application)
		item.UseCase = strings.TrimSpace(item.UseCase)
		item.RequestType = strings.TrimSpace(item.RequestType)
		item.ResponseType = strings.TrimSpace(item.ResponseType)
		item.Security.Authentication = stableStrings(item.Security.Authentication)
		item.Security.Permissions = stableStrings(item.Security.Permissions)
		item.Security.PermissionMode = strings.TrimSpace(item.Security.PermissionMode)
		item.Execution.Transaction = strings.TrimSpace(item.Execution.Transaction)
		if item.Execution.Transaction == "" {
			item.Execution.Transaction = "none"
		}
		item.Execution.Idempotency = strings.TrimSpace(item.Execution.Idempotency)
		if item.Execution.Idempotency == "" {
			item.Execution.Idempotency = "none"
		}
		if item.Security.PermissionMode == "" {
			item.Security.PermissionMode = "all"
		}
		item.Composition.Boundary = strings.TrimSpace(item.Composition.Boundary)
		item.Composition.RequiresOperations = stableStrings(item.Composition.RequiresOperations)
		item.Composition.PermissionClosure = stableStrings(item.Composition.PermissionClosure)
		item.ApplicationRequires = stableStrings(item.ApplicationRequires)
		item.Bindings.RPC = strings.TrimSpace(item.Bindings.RPC)
		item.Bindings.HTTP = append([]HTTPBinding(nil), item.Bindings.HTTP...)
		for i := range item.Bindings.HTTP {
			item.Bindings.HTTP[i].Method = strings.ToUpper(strings.TrimSpace(item.Bindings.HTTP[i].Method))
			item.Bindings.HTTP[i].Path = strings.TrimSpace(item.Bindings.HTTP[i].Path)
			item.Bindings.HTTP[i].Body = strings.TrimSpace(item.Bindings.HTTP[i].Body)
			item.Bindings.HTTP[i].ResponseBody = strings.TrimSpace(item.Bindings.HTTP[i].ResponseBody)
		}
		sort.Slice(item.Bindings.HTTP, func(i, j int) bool {
			left := item.Bindings.HTTP[i]
			right := item.Bindings.HTTP[j]
			if left.Method != right.Method {
				return left.Method < right.Method
			}
			if left.Path != right.Path {
				return left.Path < right.Path
			}
			if left.Body != right.Body {
				return left.Body < right.Body
			}
			return left.ResponseBody < right.ResponseBody
		})
		result.Operations = append(result.Operations, item)
	}
	sort.Slice(result.Operations, func(i, j int) bool {
		return result.Operations[i].OperationID < result.Operations[j].OperationID
	})
	return result
}

func Validate(set Set) error {
	set = Normalize(set)
	if set.SchemaVersion != SchemaVersion {
		return fmt.Errorf("operationplan: unsupported schemaVersion %d", set.SchemaVersion)
	}
	index := make(map[string]Plan, len(set.Operations))
	for _, item := range set.Operations {
		if item.OperationID == "" {
			return errors.New("operationplan: operationId is required")
		}
		if _, exists := index[item.OperationID]; exists {
			return fmt.Errorf("operationplan: duplicate operationId %s", item.OperationID)
		}
		if item.Domain == "" || item.Application == "" {
			return fmt.Errorf("operationplan: operation %s requires domain and application", item.OperationID)
		}
		if item.UseCase == "" {
			return fmt.Errorf("operationplan: operation %s requires useCase", item.OperationID)
		}
		if item.RequestType == "" || item.ResponseType == "" {
			return fmt.Errorf("operationplan: operation %s requires requestType and responseType", item.OperationID)
		}
		switch item.Security.PermissionMode {
		case "all", "any":
		default:
			return fmt.Errorf("operationplan: operation %s has invalid permissionMode %q", item.OperationID, item.Security.PermissionMode)
		}
		switch item.Execution.Transaction {
		case "none", "read_only", "local":
		default:
			return fmt.Errorf("operationplan: operation %s has invalid transaction policy %q", item.OperationID, item.Execution.Transaction)
		}
		switch item.Execution.Idempotency {
		case "none", "required":
		default:
			return fmt.Errorf("operationplan: operation %s has invalid idempotency policy %q", item.OperationID, item.Execution.Idempotency)
		}
		if item.Execution.Idempotency == "required" && item.Execution.Transaction != "local" {
			return fmt.Errorf("operationplan: operation %s requires local transaction for durable idempotency", item.OperationID)
		}
		switch item.Composition.Boundary {
		case "", "local", "remote_saga":
		default:
			return fmt.Errorf("operationplan: operation %s has invalid composition boundary %q", item.OperationID, item.Composition.Boundary)
		}
		if item.Bindings.RPC == "" {
			return fmt.Errorf("operationplan: operation %s requires an RPC binding", item.OperationID)
		}
		index[item.OperationID] = item
	}
	for _, item := range set.Operations {
		for _, required := range item.Composition.RequiresOperations {
			if required == item.OperationID {
				return fmt.Errorf("operationplan: operation %s cannot require itself", item.OperationID)
			}
			if _, ok := index[required]; !ok {
				return fmt.Errorf("operationplan: operation %s requires unknown operation %s", item.OperationID, required)
			}
		}
	}
	if cycle := firstCycle(index); len(cycle) > 0 {
		return fmt.Errorf("operationplan: operation dependency cycle: %s", strings.Join(cycle, " -> "))
	}
	for _, item := range set.Operations {
		requiredPermissions := make(map[string]struct{})
		collectPermissions(item.OperationID, index, map[string]bool{}, requiredPermissions)
		declared := make(map[string]struct{}, len(item.Security.Permissions))
		for _, permission := range item.Security.Permissions {
			declared[permission] = struct{}{}
		}
		missing := make([]string, 0)
		for permission := range requiredPermissions {
			if _, ok := declared[permission]; !ok {
				missing = append(missing, permission)
			}
		}
		sort.Strings(missing)
		if len(missing) > 0 {
			return fmt.Errorf("operationplan: operation %s permission closure missing: %s", item.OperationID, strings.Join(missing, ","))
		}
		wantClosure := make([]string, 0, len(requiredPermissions))
		for permission := range requiredPermissions {
			wantClosure = append(wantClosure, permission)
		}
		sort.Strings(wantClosure)
		if strings.Join(wantClosure, "\x00") != strings.Join(item.Composition.PermissionClosure, "\x00") {
			return fmt.Errorf("operationplan: operation %s has stale permission closure", item.OperationID)
		}
	}
	return nil
}

func CanonicalJSON(set Set) ([]byte, error) {
	set = Normalize(set)
	if err := Validate(set); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(set, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func Digest(set Set) (string, error) {
	data, err := CanonicalJSON(set)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}

func stableStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func collectPermissions(id string, index map[string]Plan, active map[string]bool, result map[string]struct{}) {
	if active[id] {
		return
	}
	active[id] = true
	item, ok := index[id]
	if !ok {
		delete(active, id)
		return
	}
	for _, required := range item.Composition.RequiresOperations {
		target, ok := index[required]
		if !ok {
			continue
		}
		for _, permission := range target.Security.Permissions {
			result[permission] = struct{}{}
		}
		collectPermissions(required, index, active, result)
	}
	delete(active, id)
}

func firstCycle(index map[string]Plan) []string {
	state := make(map[string]uint8, len(index))
	positions := map[string]int{}
	stack := []string{}
	var cycle []string
	var visit func(string) bool
	visit = func(id string) bool {
		if state[id] == 2 {
			return false
		}
		if state[id] == 1 {
			start := positions[id]
			cycle = append(append([]string(nil), stack[start:]...), id)
			return true
		}
		state[id] = 1
		positions[id] = len(stack)
		stack = append(stack, id)
		item := index[id]
		for _, next := range item.Composition.RequiresOperations {
			if visit(next) {
				return true
			}
		}
		stack = stack[:len(stack)-1]
		delete(positions, id)
		state[id] = 2
		return false
	}
	ids := make([]string, 0, len(index))
	for id := range index {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if visit(id) {
			return cycle
		}
	}
	return nil
}
