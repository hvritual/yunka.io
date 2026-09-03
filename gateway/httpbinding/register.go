package httpbinding

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
)

const syntheticPathValuePrefix = "__yunka_http_"

type segment struct {
	literal  string
	variable string
}

type template struct {
	raw        string
	segments   []segment
	customVerb string
}

type variableBinding struct {
	name      string
	synthetic string
	stripVerb bool
}

type registeredRoute struct {
	raw        string
	customVerb string
	bindings   []variableBinding
	handler    http.HandlerFunc
}

type routeGroup struct {
	mu      sync.RWMutex
	routes  []registeredRoute
	matches map[string]struct{}
}

type muxRegistry struct {
	mu     sync.Mutex
	groups map[string]*routeGroup
}

var registries sync.Map

// Register compiles one canonical google.api.http-style simple binding into a
// Go http.ServeMux registration. Simple {field} variables are supported. A
// custom verb attached to the final variable segment, for example
// /v1/resources/{id}:revoke, is multiplexed behind one ServeMux-compatible
// wildcard so multiple verbs can coexist without registration panics.
func Register(mux *http.ServeMux, method, path string, handler http.HandlerFunc) error {
	if mux == nil {
		return fmt.Errorf("httpbinding: mux is required")
	}
	if handler == nil {
		return fmt.Errorf("httpbinding: handler is required")
	}
	parsed, err := parseTemplate(path)
	if err != nil {
		return err
	}
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" || strings.ContainsAny(method, " \t\r\n") {
		return fmt.Errorf("httpbinding: invalid HTTP method %q", method)
	}

	pattern, bindings, multiplexed := parsed.serveMuxPattern(method)
	if !multiplexed {
		return safeHandleFunc(mux, pattern, handler)
	}

	registry := registryFor(mux)
	registry.mu.Lock()
	group := registry.groups[pattern]
	created := false
	if group == nil {
		group = &routeGroup{matches: map[string]struct{}{}}
		registry.groups[pattern] = group
		created = true
	}
	registry.mu.Unlock()

	route := registeredRoute{
		raw:        parsed.raw,
		customVerb: parsed.customVerb,
		bindings:   bindings,
		handler:    handler,
	}
	if err := group.add(route); err != nil {
		if created {
			registry.mu.Lock()
			delete(registry.groups, pattern)
			registry.mu.Unlock()
		}
		return err
	}
	if !created {
		return nil
	}
	if err := safeHandleFunc(mux, pattern, group.serveHTTP); err != nil {
		registry.mu.Lock()
		delete(registry.groups, pattern)
		registry.mu.Unlock()
		return err
	}
	return nil
}

func registryFor(mux *http.ServeMux) *muxRegistry {
	if current, ok := registries.Load(mux); ok {
		return current.(*muxRegistry)
	}
	created := &muxRegistry{groups: map[string]*routeGroup{}}
	actual, _ := registries.LoadOrStore(mux, created)
	return actual.(*muxRegistry)
}

func (group *routeGroup) add(route registeredRoute) error {
	group.mu.Lock()
	defer group.mu.Unlock()
	key := route.customVerb
	if _, exists := group.matches[key]; exists {
		return fmt.Errorf("httpbinding: duplicate binding %q", route.raw)
	}
	group.matches[key] = struct{}{}
	group.routes = append(group.routes, route)
	sort.SliceStable(group.routes, func(i, j int) bool {
		return group.routes[i].customVerb != "" && group.routes[j].customVerb == ""
	})
	return nil
}

func (group *routeGroup) serveHTTP(writer http.ResponseWriter, request *http.Request) {
	group.mu.RLock()
	routes := append([]registeredRoute(nil), group.routes...)
	group.mu.RUnlock()
	for _, route := range routes {
		if route.apply(request) {
			route.handler(writer, request)
			return
		}
	}
	http.NotFound(writer, request)
}

func (route registeredRoute) apply(request *http.Request) bool {
	for _, binding := range route.bindings {
		value := request.PathValue(binding.synthetic)
		if binding.stripVerb {
			suffix := ":" + route.customVerb
			if !strings.HasSuffix(value, suffix) {
				return false
			}
			value = strings.TrimSuffix(value, suffix)
			if value == "" {
				return false
			}
		}
		request.SetPathValue(binding.name, value)
	}
	return true
}

func (value template) serveMuxPattern(method string) (string, []variableBinding, bool) {
	parts := make([]string, 0, len(value.segments))
	bindings := make([]variableBinding, 0, len(value.segments))
	variableIndex := 0
	finalVariable := len(value.segments) > 0 && value.segments[len(value.segments)-1].variable != ""
	for index, current := range value.segments {
		if current.variable == "" {
			parts = append(parts, current.literal)
			continue
		}
		synthetic := fmt.Sprintf("%s%d", syntheticPathValuePrefix, variableIndex)
		variableIndex++
		parts = append(parts, "{"+synthetic+"}")
		bindings = append(bindings, variableBinding{
			name:      current.variable,
			synthetic: synthetic,
			stripVerb: value.customVerb != "" && index == len(value.segments)-1,
		})
	}
	path := "/" + strings.Join(parts, "/")
	if len(value.segments) == 0 {
		path = "/"
	}
	if value.customVerb != "" && !finalVariable {
		path += ":" + value.customVerb
	}
	multiplexed := len(bindings) > 0
	if !multiplexed {
		path = value.raw
	}
	return method + " " + path, bindings, multiplexed
}

func parseTemplate(raw string) (template, error) {
	if raw != strings.TrimSpace(raw) || raw == "" || !strings.HasPrefix(raw, "/") {
		return template{}, fmt.Errorf("httpbinding: invalid HTTP path %q", raw)
	}
	base, verb, err := splitCustomVerb(raw)
	if err != nil {
		return template{}, err
	}
	if base == "/" {
		if verb != "" {
			return template{}, fmt.Errorf("httpbinding: custom verb requires a non-root path in %q", raw)
		}
		return template{raw: raw}, nil
	}

	rawSegments := strings.Split(strings.TrimPrefix(base, "/"), "/")
	segments := make([]segment, 0, len(rawSegments))
	for _, current := range rawSegments {
		if current == "" {
			return template{}, fmt.Errorf("httpbinding: HTTP path %q contains an empty segment", raw)
		}
		if strings.HasPrefix(current, "{") || strings.HasSuffix(current, "}") || strings.ContainsAny(current, "{}") {
			if !strings.HasPrefix(current, "{") || !strings.HasSuffix(current, "}") || strings.Count(current, "{") != 1 || strings.Count(current, "}") != 1 {
				return template{}, fmt.Errorf("httpbinding: HTTP path segment %q requires handwritten routing", current)
			}
			name := current[1 : len(current)-1]
			if name == "" || strings.ContainsAny(name, "=/*{}") {
				return template{}, fmt.Errorf("httpbinding: HTTP path template %q requires handwritten routing", name)
			}
			segments = append(segments, segment{variable: name})
			continue
		}
		segments = append(segments, segment{literal: current})
	}
	return template{raw: raw, segments: segments, customVerb: verb}, nil
}

func splitCustomVerb(raw string) (string, string, error) {
	depth := 0
	verbIndex := -1
	for index := 0; index < len(raw); index++ {
		switch raw[index] {
		case '{':
			depth++
			if depth > 1 {
				return "", "", fmt.Errorf("httpbinding: nested HTTP path template in %q", raw)
			}
		case '}':
			depth--
			if depth < 0 {
				return "", "", fmt.Errorf("httpbinding: unmatched '}' in HTTP path %q", raw)
			}
		case ':':
			if depth == 0 {
				if verbIndex >= 0 {
					return "", "", fmt.Errorf("httpbinding: multiple custom verb delimiters in %q", raw)
				}
				verbIndex = index
			}
		}
	}
	if depth != 0 {
		return "", "", fmt.Errorf("httpbinding: unmatched '{' in HTTP path %q", raw)
	}
	if verbIndex < 0 {
		return raw, "", nil
	}
	if strings.Contains(raw[verbIndex+1:], "/") {
		return "", "", fmt.Errorf("httpbinding: custom verb must terminate HTTP path %q", raw)
	}
	verb := raw[verbIndex+1:]
	if verb == "" || strings.ContainsAny(verb, "{}: \t\r\n") {
		return "", "", fmt.Errorf("httpbinding: invalid custom verb %q", verb)
	}
	return raw[:verbIndex], verb, nil
}

func safeHandleFunc(mux *http.ServeMux, pattern string, handler http.HandlerFunc) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("httpbinding: register pattern %q: %v", pattern, recovered)
		}
	}()
	mux.HandleFunc(pattern, handler)
	return nil
}
