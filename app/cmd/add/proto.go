package add

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

var (
	packagePattern      = regexp.MustCompile(`(?m)^\s*package\s+([A-Za-z_][A-Za-z0-9_.]*)\s*;`)
	goPackagePattern    = regexp.MustCompile(`(?m)^\s*option\s+go_package\s*=\s*"([^"]+)"\s*;`)
	domainOptionPattern = regexp.MustCompile(`(?s)option\s+\(yunka\.dsl\.v1\.domain\)\s*=\s*\{.*?name\s*:\s*"([^"]+)".*?\}\s*;`)
	applicationPattern  = regexp.MustCompile(`(?s)option\s+\(yunka\.dsl\.v1\.application\)\s*=\s*\{.*?name\s*:\s*"([^"]+)".*?\}\s*;`)
	operationPattern    = regexp.MustCompile(`(?s)option\s+\(yunka\.dsl\.v1\.operation\)\s*=\s*\{.*?id\s*:\s*"([^"]+)".*?\}\s*;`)
	dtoPattern          = regexp.MustCompile(`(?s)option\s+\(yunka\.dsl\.v1\.dto\)\s*=\s*\{.*?kind\s*:\s*(DTO_[A-Z_]+).*?\}\s*;`)
	serviceStartPattern = regexp.MustCompile(`\bservice\s+([A-Za-z_][A-Za-z0-9_]*)\s*\{`)
	messageStartPattern = regexp.MustCompile(`\bmessage\s+([A-Za-z_][A-Za-z0-9_]*)\s*\{`)
	goIdentifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

func protoPackage(contents string) string {
	matches := packagePattern.FindStringSubmatch(contents)
	if len(matches) != 2 {
		return ""
	}
	return strings.TrimSpace(matches[1])
}

func protoGoPackage(contents string) string {
	matches := goPackagePattern.FindStringSubmatch(contents)
	if len(matches) != 2 {
		return ""
	}
	return strings.TrimSpace(matches[1])
}

func domainName(contents string) string {
	matches := domainOptionPattern.FindStringSubmatch(contents)
	if len(matches) != 2 {
		return ""
	}
	return strings.TrimSpace(matches[1])
}

func applicationServices(contents string) []applicationService {
	matches := serviceStartPattern.FindAllStringSubmatchIndex(contents, -1)
	result := make([]applicationService, 0, len(matches))
	for _, match := range matches {
		if len(match) < 4 {
			continue
		}
		open := match[1] - 1
		close, ok := matchingBrace(contents, open)
		if !ok {
			continue
		}
		body := contents[open+1 : close]
		application := applicationPattern.FindStringSubmatch(body)
		if len(application) != 2 {
			continue
		}
		result = append(result, applicationService{
			Name:        contents[match[2]:match[3]],
			Application: strings.TrimSpace(application[1]),
			Start:       match[0],
			Open:        open,
			Close:       close,
			Body:        body,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Application != result[j].Application {
			return result[i].Application < result[j].Application
		}
		return result[i].Name < result[j].Name
	})
	return result
}

func findService(contents, name string) (applicationService, bool) {
	for _, service := range applicationServices(contents) {
		if service.Name == name {
			return service, true
		}
	}
	return applicationService{}, false
}

func serviceExists(contents, name string) bool {
	_, ok := findService(contents, name)
	if ok {
		return true
	}
	for _, match := range serviceStartPattern.FindAllStringSubmatch(contents, -1) {
		if len(match) == 2 && match[1] == name {
			return true
		}
	}
	return false
}

func rpcExists(serviceBody, name string) bool {
	pattern := regexp.MustCompile(`\brpc\s+` + regexp.QuoteMeta(name) + `\s*\(`)
	return pattern.MatchString(serviceBody)
}

func operationIDExists(contents, id string) bool {
	for _, match := range operationPattern.FindAllStringSubmatch(contents, -1) {
		if len(match) == 2 && strings.TrimSpace(match[1]) == strings.TrimSpace(id) {
			return true
		}
	}
	return false
}

func dtoMessageKind(contents, name string) (bool, string) {
	for _, match := range messageStartPattern.FindAllStringSubmatchIndex(contents, -1) {
		if len(match) < 4 || contents[match[2]:match[3]] != name {
			continue
		}
		open := match[1] - 1
		close, ok := matchingBrace(contents, open)
		if !ok {
			return true, ""
		}
		body := contents[open+1 : close]
		option := dtoPattern.FindStringSubmatch(body)
		if len(option) != 2 {
			return true, ""
		}
		return true, strings.ToLower(strings.TrimPrefix(option[1], "DTO_"))
	}
	return false, ""
}

func insertServiceMember(contents, serviceName, member string) (string, error) {
	service, ok := findService(contents, serviceName)
	if !ok {
		return "", fmt.Errorf("structural scaffold: service %s not found", serviceName)
	}
	member = strings.TrimRight(member, " \t\r\n") + "\n"
	prefix := contents[:service.Close]
	if !strings.HasSuffix(prefix, "\n") {
		prefix += "\n"
	}
	return prefix + member + contents[service.Close:], nil
}

func appendProtoBlock(contents, block string) string {
	contents = strings.TrimRight(contents, " \t\r\n")
	block = strings.Trim(block, " \t\r\n")
	if contents == "" {
		return block + "\n"
	}
	return contents + "\n\n" + block + "\n"
}

func matchingBrace(contents string, open int) (int, bool) {
	if open < 0 || open >= len(contents) || contents[open] != '{' {
		return 0, false
	}
	depth := 0
	inString := false
	lineComment := false
	blockComment := false
	escaped := false
	for i := open; i < len(contents); i++ {
		current := contents[i]
		next := byte(0)
		if i+1 < len(contents) {
			next = contents[i+1]
		}
		if lineComment {
			if current == '\n' {
				lineComment = false
			}
			continue
		}
		if blockComment {
			if current == '*' && next == '/' {
				blockComment = false
				i++
			}
			continue
		}
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if current == '\\' {
				escaped = true
				continue
			}
			if current == '"' {
				inString = false
			}
			continue
		}
		if current == '/' && next == '/' {
			lineComment = true
			i++
			continue
		}
		if current == '/' && next == '*' {
			blockComment = true
			i++
			continue
		}
		if current == '"' {
			inString = true
			continue
		}
		switch current {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i, true
			}
		}
	}
	return 0, false
}

func applicationServiceName(application string) string {
	name := exportedIdentifier(application)
	if !strings.HasSuffix(name, "Application") {
		name += "Application"
	}
	return name
}

func exportedIdentifier(value string) string {
	parts := strings.FieldsFunc(strings.TrimSpace(value), func(current rune) bool {
		return !unicode.IsLetter(current) && !unicode.IsDigit(current)
	})
	var builder strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		runes := []rune(part)
		runes[0] = unicode.ToUpper(runes[0])
		builder.WriteString(string(runes))
	}
	result := builder.String()
	if result == "" {
		return "Operation"
	}
	if unicode.IsDigit([]rune(result)[0]) {
		return "N" + result
	}
	return result
}

func lastKeyPart(value string) string {
	parts := strings.FieldsFunc(strings.TrimSpace(value), func(current rune) bool {
		return current == '.' || current == ':' || current == '/' || current == '-' || current == '_'
	})
	if len(parts) == 0 {
		return value
	}
	return parts[len(parts)-1]
}
