package domain

import (
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"
)

func ensureInitialPO(domainRoot string, options Options) error {
	persistenceRoot := filepath.Join(domainRoot, "infrastructure", "persistence")
	if err := os.MkdirAll(persistenceRoot, 0o750); err != nil {
		return err
	}
	hasPO, err := containsPO(persistenceRoot)
	if err != nil {
		return err
	}
	if hasPO {
		return nil
	}
	object := strings.TrimSpace(options.Object)
	if object == "" {
		object = strings.TrimSpace(options.Name)
	}
	if !namePattern.MatchString(object) {
		return fmt.Errorf("domain: object %q must match %s", object, namePattern)
	}
	fields, err := parseFields(options.Fields)
	if err != nil {
		return err
	}
	path := filepath.Join(persistenceRoot, snakeCase(object)+".go")
	formatted, err := format.Source([]byte(developerPOFileTemplate(object, fields)))
	if err != nil {
		return fmt.Errorf("domain: format initial PO %s: %w", object, err)
	}
	return os.WriteFile(path, formatted, 0o640)
}

func containsPO(persistenceRoot string) (bool, error) {
	entries, err := os.ReadDir(persistenceRoot)
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" || strings.HasPrefix(entry.Name(), "zz_yunka_") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		_, found, err := scanPOFile(filepath.Join(persistenceRoot, entry.Name()), entry.Name(), nil)
		if err != nil {
			return false, err
		}
		if found {
			return true, nil
		}
	}
	return false, nil
}

func developerPOFileTemplate(object string, fields []Field) string {
	objectType := exportedIdentifier(object)
	usesTime := fieldNeedsTime(fields)
	var builder strings.Builder
	builder.WriteString("// Scaffolded by yunka domain. Application-owned; safe to edit.\n\n")
	builder.WriteString("package persistence\n\n")
	if usesTime {
		builder.WriteString("import \"time\"\n\n")
	}
	fmt.Fprintf(&builder, "// %sPO is the developer-owned persistence schema.\n", objectType)
	builder.WriteString("// Exported supported scalar fields are mirrored only into the generated Domain Entity and basic Repository CRUD.\n")
	builder.WriteString("// API DTO/RPC/REST/Application declarations belong to protobuf, never to this PO.\n")
	builder.WriteString("// Add `yunka:\"-\"` to keep a field persistence-only while retaining it in the GORM record.\n")
	fmt.Fprintf(&builder, "type %sPO struct {\n", objectType)
	for _, field := range fields {
		typeName, _ := goType(field.Type)
		fmt.Fprintf(&builder, "\t%s %s `gorm:\"column:%s;%s\"`\n", exportedIdentifier(field.Name), typeName, field.Name, columnType(field))
	}
	builder.WriteString("}\n")
	return builder.String()
}
