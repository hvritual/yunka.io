package contract

import (
	"fmt"
	"strings"
)

func validateC9HTTPBindingPath(path string) error {
	if path != strings.TrimSpace(path) || path == "" || !strings.HasPrefix(path, "/") {
		return fmt.Errorf("HTTP path %q is invalid", path)
	}
	if _, err := simplePathFields(path); err != nil {
		return err
	}
	base, verb, err := splitC9CustomVerb(path)
	if err != nil {
		return err
	}
	if base == "/" {
		if verb != "" {
			return fmt.Errorf("HTTP path %q cannot attach a custom verb to root", path)
		}
		return nil
	}
	for _, segment := range strings.Split(strings.TrimPrefix(base, "/"), "/") {
		if segment == "" {
			return fmt.Errorf("HTTP path %q contains an empty segment", path)
		}
		if !strings.ContainsAny(segment, "{}") {
			continue
		}
		if !strings.HasPrefix(segment, "{") || !strings.HasSuffix(segment, "}") || strings.Count(segment, "{") != 1 || strings.Count(segment, "}") != 1 {
			return fmt.Errorf("HTTP path segment %q requires handwritten routing", segment)
		}
		name := segment[1 : len(segment)-1]
		if name == "" || strings.ContainsAny(name, "=/*{}") {
			return fmt.Errorf("HTTP path template %q requires handwritten routing", name)
		}
	}
	return nil
}

func splitC9CustomVerb(path string) (string, string, error) {
	depth := 0
	verbIndex := -1
	for index := 0; index < len(path); index++ {
		switch path[index] {
		case '{':
			depth++
			if depth > 1 {
				return "", "", fmt.Errorf("HTTP path %q has nested templates", path)
			}
		case '}':
			depth--
			if depth < 0 {
				return "", "", fmt.Errorf("HTTP path %q has unmatched '}'", path)
			}
		case ':':
			if depth == 0 {
				if verbIndex >= 0 {
					return "", "", fmt.Errorf("HTTP path %q has multiple custom verb delimiters", path)
				}
				verbIndex = index
			}
		}
	}
	if depth != 0 {
		return "", "", fmt.Errorf("HTTP path %q has unmatched '{'", path)
	}
	if verbIndex < 0 {
		return path, "", nil
	}
	if strings.Contains(path[verbIndex+1:], "/") {
		return "", "", fmt.Errorf("HTTP path %q custom verb must terminate the template", path)
	}
	verb := path[verbIndex+1:]
	if verb == "" || strings.ContainsAny(verb, "{}: \t\r\n") {
		return "", "", fmt.Errorf("HTTP path %q has invalid custom verb %q", path, verb)
	}
	return path[:verbIndex], verb, nil
}
