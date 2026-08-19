package contract

import (
	"fmt"
	"strconv"
	"strings"
)

const googleHTTPOptionField = 72295728

type fileDescriptorSet struct {
	Files []fileDescriptor
}

type fileDescriptor struct {
	Name       string
	Package    string
	Syntax     string
	GoPackage  string
	Messages   []messageDescriptor
	Enums      []enumDescriptor
	Services   []serviceDescriptor
	SourceInfo sourceInfoDescriptor
}

type messageDescriptor struct {
	Name     string
	Fields   []fieldDescriptor
	Nested   []messageDescriptor
	Enums    []enumDescriptor
	MapEntry bool
}

type fieldDescriptor struct {
	Name           string
	Number         int32
	Label          int32
	Type           int32
	TypeName       string
	JSONName       string
	OneofIndex     *int32
	Proto3Optional bool
}

type enumDescriptor struct {
	Name   string
	Values []enumValueDescriptor
}

type enumValueDescriptor struct {
	Name   string
	Number int32
}

type serviceDescriptor struct {
	Name    string
	Methods []methodDescriptor
}

type methodDescriptor struct {
	Name            string
	InputType       string
	OutputType      string
	ClientStreaming bool
	ServerStreaming bool
	Options         []byte
}

type sourceInfoDescriptor struct {
	Comments map[string]string
}

func parseDescriptorSet(data []byte) (fileDescriptorSet, error) {
	var set fileDescriptorSet
	err := scanWire(data, func(field wireField) error {
		if field.Number != 1 || field.Type != 2 {
			return nil
		}
		file, err := parseFileDescriptor(field.Bytes)
		if err != nil {
			return err
		}
		set.Files = append(set.Files, file)
		return nil
	})
	return set, err
}

func parseFileDescriptor(data []byte) (fileDescriptor, error) {
	var file fileDescriptor
	err := scanWire(data, func(field wireField) error {
		switch field.Number {
		case 1:
			file.Name = string(field.Bytes)
		case 2:
			file.Package = string(field.Bytes)
		case 4:
			message, err := parseMessageDescriptor(field.Bytes)
			if err != nil {
				return err
			}
			file.Messages = append(file.Messages, message)
		case 5:
			enum, err := parseEnumDescriptor(field.Bytes)
			if err != nil {
				return err
			}
			file.Enums = append(file.Enums, enum)
		case 6:
			service, err := parseServiceDescriptor(field.Bytes)
			if err != nil {
				return err
			}
			file.Services = append(file.Services, service)
		case 8:
			goPackage, err := parseFileOptions(field.Bytes)
			if err != nil {
				return err
			}
			file.GoPackage = goPackage
		case 9:
			sourceInfo, err := parseSourceInfo(field.Bytes)
			if err != nil {
				return err
			}
			file.SourceInfo = sourceInfo
		case 12:
			file.Syntax = string(field.Bytes)
		}
		return nil
	})
	return file, err
}

func parseFileOptions(data []byte) (string, error) {
	var goPackage string
	err := scanWire(data, func(field wireField) error {
		if field.Number == 11 && field.Type == 2 {
			goPackage = string(field.Bytes)
		}
		return nil
	})
	return goPackage, err
}

func parseMessageDescriptor(data []byte) (messageDescriptor, error) {
	var message messageDescriptor
	var options []byte
	err := scanWire(data, func(field wireField) error {
		switch field.Number {
		case 1:
			message.Name = string(field.Bytes)
		case 2:
			item, err := parseFieldDescriptor(field.Bytes)
			if err != nil {
				return err
			}
			message.Fields = append(message.Fields, item)
		case 3:
			nested, err := parseMessageDescriptor(field.Bytes)
			if err != nil {
				return err
			}
			message.Nested = append(message.Nested, nested)
		case 4:
			enum, err := parseEnumDescriptor(field.Bytes)
			if err != nil {
				return err
			}
			message.Enums = append(message.Enums, enum)
		case 7:
			options = append([]byte(nil), field.Bytes...)
		}
		return nil
	})
	if err != nil {
		return message, err
	}
	if len(options) > 0 {
		if err := scanWire(options, func(field wireField) error {
			if field.Number == 7 && field.Type == 0 {
				message.MapEntry = field.Varint != 0
			}
			return nil
		}); err != nil {
			return message, err
		}
	}
	return message, nil
}

func parseFieldDescriptor(data []byte) (fieldDescriptor, error) {
	var field fieldDescriptor
	err := scanWire(data, func(item wireField) error {
		switch item.Number {
		case 1:
			field.Name = string(item.Bytes)
		case 3:
			field.Number = int32(item.Varint)
		case 4:
			field.Label = int32(item.Varint)
		case 5:
			field.Type = int32(item.Varint)
		case 6:
			field.TypeName = normalizeTypeName(string(item.Bytes))
		case 9:
			index := int32(item.Varint)
			field.OneofIndex = &index
		case 10:
			field.JSONName = string(item.Bytes)
		case 17:
			field.Proto3Optional = item.Varint != 0
		}
		return nil
	})
	if field.JSONName == "" {
		field.JSONName = lowerCamel(field.Name)
	}
	return field, err
}

func parseEnumDescriptor(data []byte) (enumDescriptor, error) {
	var enum enumDescriptor
	err := scanWire(data, func(field wireField) error {
		switch field.Number {
		case 1:
			enum.Name = string(field.Bytes)
		case 2:
			value, err := parseEnumValue(field.Bytes)
			if err != nil {
				return err
			}
			enum.Values = append(enum.Values, value)
		}
		return nil
	})
	return enum, err
}

func parseEnumValue(data []byte) (enumValueDescriptor, error) {
	var value enumValueDescriptor
	err := scanWire(data, func(field wireField) error {
		switch field.Number {
		case 1:
			value.Name = string(field.Bytes)
		case 2:
			value.Number = int32(field.Varint)
		}
		return nil
	})
	return value, err
}

func parseServiceDescriptor(data []byte) (serviceDescriptor, error) {
	var service serviceDescriptor
	err := scanWire(data, func(field wireField) error {
		switch field.Number {
		case 1:
			service.Name = string(field.Bytes)
		case 2:
			method, err := parseMethodDescriptor(field.Bytes)
			if err != nil {
				return err
			}
			service.Methods = append(service.Methods, method)
		}
		return nil
	})
	return service, err
}

func parseMethodDescriptor(data []byte) (methodDescriptor, error) {
	var method methodDescriptor
	err := scanWire(data, func(field wireField) error {
		switch field.Number {
		case 1:
			method.Name = string(field.Bytes)
		case 2:
			method.InputType = normalizeTypeName(string(field.Bytes))
		case 3:
			method.OutputType = normalizeTypeName(string(field.Bytes))
		case 4:
			method.Options = append([]byte(nil), field.Bytes...)
		case 5:
			method.ClientStreaming = field.Varint != 0
		case 6:
			method.ServerStreaming = field.Varint != 0
		}
		return nil
	})
	return method, err
}

func parseSourceInfo(data []byte) (sourceInfoDescriptor, error) {
	info := sourceInfoDescriptor{Comments: make(map[string]string)}
	err := scanWire(data, func(field wireField) error {
		if field.Number != 1 || field.Type != 2 {
			return nil
		}
		path, comment, err := parseSourceLocation(field.Bytes)
		if err != nil {
			return err
		}
		if len(path) > 0 && strings.TrimSpace(comment) != "" {
			info.Comments[pathKey(path)] = comment
		}
		return nil
	})
	return info, err
}

func parseSourceLocation(data []byte) ([]int32, string, error) {
	var path []int32
	var comments []string
	err := scanWire(data, func(field wireField) error {
		switch field.Number {
		case 1:
			if field.Type == 0 {
				path = append(path, int32(field.Varint))
			} else if field.Type == 2 {
				values, err := packedInt32(field.Bytes)
				if err != nil {
					return err
				}
				path = append(path, values...)
			}
		case 3, 4, 6:
			if field.Type == 2 {
				comments = append(comments, string(field.Bytes))
			}
		}
		return nil
	})
	return path, strings.Join(comments, "\n"), err
}

func pathKey(path []int32) string {
	parts := make([]string, len(path))
	for i, value := range path {
		parts[i] = strconv.FormatInt(int64(value), 10)
	}
	return strings.Join(parts, ",")
}

func parseHTTPBindings(options []byte) ([]HTTPBinding, error) {
	if len(options) == 0 {
		return nil, nil
	}
	var bindings []HTTPBinding
	err := scanWire(options, func(field wireField) error {
		if field.Number != googleHTTPOptionField || field.Type != 2 {
			return nil
		}
		rules, err := parseHTTPRule(field.Bytes)
		if err != nil {
			return err
		}
		bindings = append(bindings, rules...)
		return nil
	})
	return bindings, err
}

func parseHTTPRule(data []byte) ([]HTTPBinding, error) {
	var binding HTTPBinding
	var additional [][]byte
	err := scanWire(data, func(field wireField) error {
		switch field.Number {
		case 2:
			binding.Method, binding.Path = "GET", string(field.Bytes)
		case 3:
			binding.Method, binding.Path = "PUT", string(field.Bytes)
		case 4:
			binding.Method, binding.Path = "POST", string(field.Bytes)
		case 5:
			binding.Method, binding.Path = "DELETE", string(field.Bytes)
		case 6:
			binding.Method, binding.Path = "PATCH", string(field.Bytes)
		case 7:
			binding.Body = string(field.Bytes)
		case 8:
			method, path, err := parseCustomHTTPPattern(field.Bytes)
			if err != nil {
				return err
			}
			binding.Method, binding.Path = method, path
		case 11:
			additional = append(additional, append([]byte(nil), field.Bytes...))
		case 12:
			binding.ResponseBody = string(field.Bytes)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	bindings := make([]HTTPBinding, 0, 1+len(additional))
	if binding.Method != "" && binding.Path != "" {
		bindings = append(bindings, binding)
	}
	for _, raw := range additional {
		items, err := parseHTTPRule(raw)
		if err != nil {
			return nil, err
		}
		bindings = append(bindings, items...)
	}
	return bindings, nil
}

func parseCustomHTTPPattern(data []byte) (string, string, error) {
	var method, path string
	err := scanWire(data, func(field wireField) error {
		switch field.Number {
		case 1:
			method = strings.ToUpper(string(field.Bytes))
		case 2:
			path = string(field.Bytes)
		}
		return nil
	})
	return method, path, err
}

func parseDirectives(comment string) map[string]string {
	result := make(map[string]string)
	for _, raw := range strings.Split(comment, "\n") {
		line := strings.TrimSpace(raw)
		line = strings.TrimLeft(line, "*/ ")
		if !strings.HasPrefix(line, "@yunka.") {
			continue
		}
		line = strings.TrimPrefix(line, "@yunka.")
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}
		key := parts[0]
		value := strings.TrimSpace(strings.TrimPrefix(line, key))
		result[key] = value
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func directiveHTTPBinding(directives map[string]string) (HTTPBinding, bool) {
	value := strings.TrimSpace(directives["http"])
	if value == "" {
		return HTTPBinding{}, false
	}
	parts := strings.Fields(value)
	if len(parts) < 2 {
		return HTTPBinding{}, false
	}
	binding := HTTPBinding{Method: strings.ToUpper(parts[0]), Path: parts[1]}
	for _, option := range parts[2:] {
		key, val, ok := strings.Cut(option, "=")
		if !ok {
			continue
		}
		switch key {
		case "body":
			binding.Body = val
		case "response_body":
			binding.ResponseBody = val
		}
	}
	return binding, true
}

func lowerCamel(value string) string {
	if value == "" {
		return ""
	}
	parts := strings.Split(value, "_")
	if len(parts) == 1 {
		return strings.ToLower(value[:1]) + value[1:]
	}
	var builder strings.Builder
	builder.WriteString(strings.ToLower(parts[0]))
	for _, part := range parts[1:] {
		if part == "" {
			continue
		}
		builder.WriteString(strings.ToUpper(part[:1]))
		builder.WriteString(part[1:])
	}
	return builder.String()
}

func scalarType(fieldType int32) (string, bool) {
	switch fieldType {
	case 1:
		return "double", true
	case 2:
		return "float", true
	case 3:
		return "int64", true
	case 4:
		return "uint64", true
	case 5:
		return "int32", true
	case 6:
		return "fixed64", true
	case 7:
		return "fixed32", true
	case 8:
		return "bool", true
	case 9:
		return "string", true
	case 12:
		return "bytes", true
	case 13:
		return "uint32", true
	case 15:
		return "sfixed32", true
	case 16:
		return "sfixed64", true
	case 17:
		return "sint32", true
	case 18:
		return "sint64", true
	default:
		return "", false
	}
}

func validateHTTPMethod(method string) error {
	switch strings.ToUpper(method) {
	case "GET", "PUT", "POST", "DELETE", "PATCH", "HEAD", "OPTIONS":
		return nil
	default:
		return fmt.Errorf("unsupported HTTP method %q", method)
	}
}
