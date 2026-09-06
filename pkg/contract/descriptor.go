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
	Dependencies []string
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
		case 3:
			file.Dependencies = append(file.Dependencies, string(field.Bytes))
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
	info := sourceInfoDescriptor{Comments: map[string]string{}}
	err := scanWire(data, func(field wireField) error {
		if field.Number != 1 || field.Type != 2 {
			return nil
		}
		path, leading, trailing, err := parseSourceLocation(field.Bytes)
		if err != nil {
			return err
		}
		comment := strings.TrimSpace(strings.Join([]string{leading, trailing}, "\n"))
		if len(path) > 0 && comment != "" {
			info.Comments[pathKey(path)] = comment
		}
		return nil
	})
	return info, err
}

func parseSourceLocation(data []byte) ([]int32, string, string, error) {
	var path []int32
	var leading, trailing string
	err := scanWire(data, func(field wireField) error {
		switch field.Number {
		case 1:
			if field.Type == 2 {
				values, err := parsePackedInt32(field.Bytes)
				if err != nil {
					return err
				}
				path = append(path, values...)
			} else if field.Type == 0 {
				path = append(path, int32(field.Varint))
			}
		case 3:
			leading = string(field.Bytes)
		case 4:
			trailing = string(field.Bytes)
		}
		return nil
	})
	return path, leading, trailing, err
}

func parsePackedInt32(data []byte) ([]int32, error) {
	var values []int32
	for len(data) > 0 {
		value, n := readVarint(data)
		if n <= 0 {
			return nil, fmt.Errorf("contract: invalid packed int32")
		}
		values = append(values, int32(value))
		data = data[n:]
	}
	return values, nil
}

func pathKey(path []int32) string {
	parts := make([]string, len(path))
	for i, item := range path {
		parts[i] = strconv.FormatInt(int64(item), 10)
	}
	return strings.Join(parts, ".")
}
