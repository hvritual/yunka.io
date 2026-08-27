package domain

import (
	"fmt"
	"strings"
)

// renderMultiStructural is deliberately persistence-only. C8.4 makes protobuf
// the API/Application DSL, so the domain compiler stops after Entity + basic
// Repository CRUD interface/implementation generation.
func renderMultiStructural(spec Spec, packageImport string) map[string]string {
	files := map[string]string{
		"ports/zz_yunka_repositories_gen.go":                      multiPortsTemplate(spec, packageImport),
		"infrastructure/persistence/zz_yunka_repositories_gen.go": multiRepositoriesTemplate(spec, packageImport),
	}
	for _, object := range spec.Objects {
		files["domain/zz_yunka_"+object.Name+"_entity_gen.go"] = multiEntityTemplate(spec, object)
		files["infrastructure/persistence/zz_yunka_"+object.Name+"_record_gen.go"] = multiRecordTemplate(spec, object, packageImport)
	}
	return files
}

func multiEntityTemplate(spec Spec, object ObjectSpec) string {
	entity := objectGoName(object)
	var b strings.Builder
	b.WriteString(generatedDomainMarker + "\n\npackage domain\n\nimport \"time\"\n\n")
	fmt.Fprintf(&b, "type %s struct {\n\tID string `json:\"id\"`\n", entity)
	if spec.TenantScoped {
		b.WriteString("\tTenantID string `json:\"tenantId\"`\n")
	}
	for _, field := range object.Fields {
		typeName, _ := goType(field.Type)
		fmt.Fprintf(&b, "\t%s %s `json:\"%s\"`\n", fieldGoName(field), typeName, jsonName(field.Name))
	}
	b.WriteString("\tVersion uint64 `json:\"version\"`\n\tCreatedAt time.Time `json:\"createdAt\"`\n\tUpdatedAt time.Time `json:\"updatedAt\"`\n}\n")
	return b.String()
}

func multiPortsTemplate(spec Spec, packageImport string) string {
	var b strings.Builder
	b.WriteString(generatedDomainMarker + "\n\npackage ports\n\n")
	fmt.Fprintf(&b, "import (\n\t\"context\"\n\t\"errors\"\n\tdomain %q\n)\n\n", packageImport+"/domain")
	b.WriteString("var (\n\tErrNotFound = errors.New(\"domain repository: not found\")\n\tErrConflict = errors.New(\"domain repository: version conflict\")\n)\n\n")
	for _, object := range spec.Objects {
		entity := objectGoName(object)
		fmt.Fprintf(&b, "type %sRepository interface {\n", entity)
		fmt.Fprintf(&b, "\tCreate(context.Context, *domain.%s) error\n", entity)
		fmt.Fprintf(&b, "\tGet(context.Context, string) (domain.%s, error)\n", entity)
		fmt.Fprintf(&b, "\tList(context.Context, int, int) ([]domain.%s, error)\n", entity)
		fmt.Fprintf(&b, "\tUpdate(context.Context, *domain.%s, uint64) error\n", entity)
		b.WriteString("\tDelete(context.Context, string, uint64) error\n}\n\n")
	}
	b.WriteString("type Repositories struct {\n")
	for _, object := range spec.Objects {
		entity := objectGoName(object)
		fmt.Fprintf(&b, "\t%s %sRepository\n", entity, entity)
	}
	b.WriteString("}\n")
	return b.String()
}

func multiRecordTemplate(spec Spec, object ObjectSpec, packageImport string) string {
	entity := objectGoName(object)
	po := entity + "PO"
	base := entity + "POBase"
	record := entity + "PORecord"
	var b strings.Builder
	b.WriteString(generatedDomainMarker + "\n\npackage persistence\n\n")
	fmt.Fprintf(&b, "import (\n\t\"time\"\n\t\"gorm.io/gorm\"\n\tdomain %q\n)\n\n", packageImport+"/domain")
	fmt.Fprintf(&b, "type %s struct {\n\tID string `gorm:\"column:id;type:varchar(64);primaryKey\"`\n", base)
	if spec.TenantScoped {
		b.WriteString("\tTenantID string `gorm:\"column:tenant_id;type:varchar(64);not null;index\"`\n")
	}
	if object.POEmbedsBase {
		for _, field := range object.Fields {
			typeName, _ := goType(field.Type)
			fmt.Fprintf(&b, "\t%s %s `gorm:\"column:%s;%s\"`\n", fieldGoName(field), typeName, field.Column, columnType(field))
		}
	}
	b.WriteString("\tVersion uint64 `gorm:\"column:version;type:bigint unsigned;not null;default:1\"`\n")
	b.WriteString("\tCreatedAt time.Time `gorm:\"column:created_at;type:datetime(3);not null\"`\n")
	b.WriteString("\tUpdatedAt time.Time `gorm:\"column:updated_at;type:datetime(3);not null\"`\n")
	b.WriteString("\tDeletedAt gorm.DeletedAt `gorm:\"column:deleted_at;type:datetime(3);index\"`\n")
	b.WriteString("}\n\n")
	if object.POEmbedsBase {
		fmt.Fprintf(&b, "type %s struct { %s `gorm:\"embedded\"` }\n\n", record, po)
	} else {
		fmt.Fprintf(&b, "type %s struct {\n\t%s `gorm:\"embedded\"`\n\t%s `gorm:\"embedded\"`\n}\n\n", record, po, base)
	}
	fmt.Fprintf(&b, "func (%s) TableName() string { return %q }\n\n", record, object.TableName)
	fmt.Fprintf(&b, "func (record %s) Domain() domain.%s {\n\treturn domain.%s{\n\t\tID: record.ID,\n", record, entity, entity)
	if spec.TenantScoped {
		b.WriteString("\t\tTenantID: record.TenantID,\n")
	}
	for _, field := range object.Fields {
		f := fieldGoName(field)
		fmt.Fprintf(&b, "\t\t%s: record.%s.%s,\n", f, po, f)
	}
	b.WriteString("\t\tVersion: record.Version,\n\t\tCreatedAt: record.CreatedAt,\n\t\tUpdatedAt: record.UpdatedAt,\n\t}\n}\n\n")
	fmt.Fprintf(&b, "func %sRecordFromDomain(value domain.%s) %s {\n\trecord := %s{}\n\trecord.ID = value.ID\n", lowerFirst(entity), entity, record, record)
	if spec.TenantScoped {
		b.WriteString("\trecord.TenantID = value.TenantID\n")
	}
	b.WriteString("\trecord.Version = value.Version\n\trecord.CreatedAt = value.CreatedAt\n\trecord.UpdatedAt = value.UpdatedAt\n")
	for _, field := range object.Fields {
		f := fieldGoName(field)
		fmt.Fprintf(&b, "\trecord.%s.%s = value.%s\n", po, f, f)
	}
	b.WriteString("\treturn record\n}\n")
	return b.String()
}

func multiRepositoriesTemplate(spec Spec, packageImport string) string {
	var b strings.Builder
	b.WriteString(generatedDomainMarker + "\n\npackage persistence\n\n")
	fmt.Fprintf(&b, "import (\n\t\"context\"\n\t\"errors\"\n\t\"time\"\n\n\t\"gorm.io/gorm\"\n\tdomain %q\n\tports %q\n\t\"yunka.io/framework/requestscope\"\n", packageImport+"/domain", packageImport+"/ports")
	if spec.TenantScoped {
		b.WriteString("\tidentity \"yunka.io/framework/core/identity\"\n")
	}
	b.WriteString(")\n\n")
	b.WriteString("// AutoMigrate is an explicit persistence helper. Application bootstrap decides when migrations run.\n")
	b.WriteString("func AutoMigrate(ctx context.Context, database *gorm.DB) error {\n\tif database == nil { return errors.New(\"domain persistence: database is required\") }\n\tif ctx == nil { ctx = context.Background() }\n\treturn database.WithContext(ctx).AutoMigrate(")
	for i, object := range spec.Objects {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "&%sPORecord{}", objectGoName(object))
	}
	b.WriteString(")\n}\n\n")
	if spec.TenantScoped {
		b.WriteString("func trustedTenant(ctx context.Context) (string, error) {\n\tprincipal, ok := identity.FromContext(ctx)\n\tif !ok || !principal.Authenticated || principal.TenantID == \"\" { return \"\", errors.New(\"domain persistence: trusted tenant principal is required\") }\n\treturn principal.TenantID, nil\n}\n\n")
	}
	for _, object := range spec.Objects {
		writeMultiRepository(&b, spec, object)
	}
	b.WriteString("// NewScopeFactory preserves request-owned transaction and repository composition.\n")
	b.WriteString("func NewScopeFactory(database *gorm.DB) (requestscope.ScopeFactory[ports.Repositories], error) {\n")
	b.WriteString("\tunit, err := requestscope.NewGORMFactory(database, nil)\n\tif err != nil { return nil, err }\n")
	b.WriteString("\treturn requestscope.NewFactory(requestscope.FactoryOptions[ports.Repositories]{\n\t\tUnitOfWork: unit,\n\t\tRepositories: requestscope.GORMRepositories(func(ctx context.Context, transaction *gorm.DB) (ports.Repositories, error) {\n\t\t\trepositories := ports.Repositories{}\n")
	for _, object := range spec.Objects {
		entity := objectGoName(object)
		fmt.Fprintf(&b, "\t\t\t%sRepository, err := New%sRepository(transaction)\n\t\t\tif err != nil { return ports.Repositories{}, err }\n\t\t\trepositories.%s = %sRepository\n", lowerFirst(entity), entity, entity, lowerFirst(entity))
	}
	b.WriteString("\t\t\treturn repositories, nil\n\t\t}),\n\t})\n}\n")
	return b.String()
}

func writeMultiRepository(b *strings.Builder, spec Spec, object ObjectSpec) {
	entity := objectGoName(object)
	record := entity + "PORecord"
	fmt.Fprintf(b, "type %sRepository struct { database *gorm.DB }\n\n", entity)
	fmt.Fprintf(b, "func New%sRepository(database *gorm.DB) (*%sRepository, error) {\n\tif database == nil { return nil, errors.New(\"domain persistence: database is required\") }\n\treturn &%sRepository{database: database}, nil\n}\n\n", entity, entity, entity)
	fmt.Fprintf(b, "func (repository *%sRepository) scoped(ctx context.Context) (*gorm.DB, error) {\n\tquery := repository.database.WithContext(ctx)\n", entity)
	if spec.TenantScoped {
		b.WriteString("\ttenantID, err := trustedTenant(ctx)\n\tif err != nil { return nil, err }\n\tquery = query.Where(\"tenant_id = ?\", tenantID)\n")
	}
	b.WriteString("\treturn query, nil\n}\n\n")
	fmt.Fprintf(b, "func (repository *%sRepository) Create(ctx context.Context, value *domain.%s) error {\n\tif value == nil { return errors.New(\"domain persistence: value is required\") }\n", entity, entity)
	if spec.TenantScoped {
		b.WriteString("\ttenantID, err := trustedTenant(ctx)\n\tif err != nil { return err }\n\tvalue.TenantID = tenantID\n")
	}
	b.WriteString("\tif value.Version == 0 { value.Version = 1 }\n\tnow := time.Now().UTC()\n\tif value.CreatedAt.IsZero() { value.CreatedAt = now }\n\tvalue.UpdatedAt = now\n")
	fmt.Fprintf(b, "\trecord := %sRecordFromDomain(*value)\n\tif err := repository.database.WithContext(ctx).Create(&record).Error; err != nil { return err }\n\t*value = record.Domain()\n\treturn nil\n}\n\n", lowerFirst(entity))
	fmt.Fprintf(b, "func (repository *%sRepository) Get(ctx context.Context, id string) (domain.%s, error) {\n\tquery, err := repository.scoped(ctx)\n\tif err != nil { return domain.%s{}, err }\n\tvar record %s\n\tif err := query.Where(\"id = ?\", id).First(&record).Error; err != nil {\n\t\tif errors.Is(err, gorm.ErrRecordNotFound) { return domain.%s{}, ports.ErrNotFound }\n\t\treturn domain.%s{}, err\n\t}\n\treturn record.Domain(), nil\n}\n\n", entity, entity, entity, record, entity, entity)
	fmt.Fprintf(b, "func (repository *%sRepository) List(ctx context.Context, limit, offset int) ([]domain.%s, error) {\n\tquery, err := repository.scoped(ctx)\n\tif err != nil { return nil, err }\n\tvar rows []%s\n\tif limit > 0 { query = query.Limit(limit) }\n\tif offset > 0 { query = query.Offset(offset) }\n\tif err := query.Order(\"created_at DESC\").Find(&rows).Error; err != nil { return nil, err }\n\tresult := make([]domain.%s, 0, len(rows))\n\tfor _, row := range rows { result = append(result, row.Domain()) }\n\treturn result, nil\n}\n\n", entity, entity, record, entity)
	fmt.Fprintf(b, "func (repository *%sRepository) Update(ctx context.Context, value *domain.%s, expectedVersion uint64) error {\n\tif value == nil || value.ID == \"\" { return errors.New(\"domain persistence: update value/id is required\") }\n\tquery, err := repository.scoped(ctx)\n\tif err != nil { return err }\n\tupdates := map[string]any{\"version\": gorm.Expr(\"version + 1\"), \"updated_at\": time.Now().UTC(),\n", entity, entity)
	for _, field := range object.Fields {
		fmt.Fprintf(b, "\t\t%q: value.%s,\n", field.Column, fieldGoName(field))
	}
	fmt.Fprintf(b, "\t}\n\tresult := query.Model(&%s{}).Where(\"id = ? AND version = ?\", value.ID, expectedVersion).Updates(updates)\n\tif result.Error != nil { return result.Error }\n\tif result.RowsAffected != 1 { return ports.ErrConflict }\n\treturn nil\n}\n\n", record)
	fmt.Fprintf(b, "func (repository *%sRepository) Delete(ctx context.Context, id string, expectedVersion uint64) error {\n\tquery, err := repository.scoped(ctx)\n\tif err != nil { return err }\n\tresult := query.Where(\"id = ? AND version = ?\", id, expectedVersion).Delete(&%s{})\n\tif result.Error != nil { return result.Error }\n\tif result.RowsAffected != 1 { return ports.ErrConflict }\n\treturn nil\n}\n\n", entity, record)
}
