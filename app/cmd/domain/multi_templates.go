package domain

import (
	"fmt"
	"strings"
)

func renderMultiStructural(spec Spec, packageImport string) map[string]string {
	files := map[string]string{
		"application/zz_yunka_service_gen.go":                     multiApplicationTemplate(spec, packageImport),
		"ports/zz_yunka_repositories_gen.go":                      multiPortsTemplate(spec, packageImport),
		"infrastructure/persistence/zz_yunka_repositories_gen.go": multiRepositoriesTemplate(spec, packageImport),
		"wire/zz_yunka_wiring_gen.go":                             multiWireTemplate(spec, packageImport),
	}
	for _, object := range spec.Objects {
		files["domain/zz_yunka_"+object.Name+"_entity_gen.go"] = multiEntityTemplate(spec, object)
		files["infrastructure/persistence/zz_yunka_"+object.Name+"_record_gen.go"] = multiRecordTemplate(spec, object, packageImport)
	}
	if spec.REST.Enabled {
		files["transport/rest/zz_yunka_rest_gen.go"] = multiRESTTemplate(spec, packageImport)
	}
	if spec.RPC.Enabled {
		files["transport/rpc/domain.proto"] = multiProtoTemplate(spec, packageImport)
		files["transport/rpc/zz_yunka_grpc_bridge_gen.go"] = multiGRPCBridgeTemplate(spec, packageImport)
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

func multiApplicationTemplate(spec Spec, packageImport string) string {
	var b strings.Builder
	b.WriteString(generatedDomainMarker + "\n\npackage application\n\n")
	fmt.Fprintf(&b, "import (\n\t\"context\"\n\t\"crypto/rand\"\n\t\"encoding/hex\"\n\t\"errors\"\n\t\"time\"\n\tdomain %q\n\tports %q\n\t\"yunka.io/framework/requestscope\"\n)\n\n", packageImport+"/domain", packageImport+"/ports")
	b.WriteString("type Service interface {\n")
	for _, object := range spec.Objects {
		entity := objectGoName(object)
		plural := exportedIdentifier(pluralize(object.Name))
		fmt.Fprintf(&b, "\tCreate%s(context.Context, Create%sInput) (%sOutput,error)\n\tGet%s(context.Context, Get%sInput) (%sOutput,error)\n\tList%s(context.Context, List%sInput) (List%sOutput,error)\n\tUpdate%s(context.Context, Update%sInput) (%sOutput,error)\n\tDelete%s(context.Context, Delete%sInput) error\n", entity, entity, entity, entity, entity, entity, plural, plural, plural, entity, entity, entity, entity, entity)
	}
	b.WriteString("}\n\n")
	for _, object := range spec.Objects {
		writeMultiApplicationTypes(&b, object)
	}
	b.WriteString("type DefaultService struct { scopes requestscope.ScopeFactory[ports.Repositories] }\nfunc NewService(scopes requestscope.ScopeFactory[ports.Repositories]) (*DefaultService,error) { if scopes==nil { return nil,errors.New(\"domain application: request scope factory is required\") }; return &DefaultService{scopes:scopes},nil }\n\n")
	for _, object := range spec.Objects {
		writeMultiApplicationMethods(&b, object)
	}
	b.WriteString("func newID() string { var value [16]byte; if _,err:=rand.Read(value[:]); err!=nil { panic(err) }; return hex.EncodeToString(value[:]) }\n")
	return b.String()
}

func writeMultiApplicationTypes(b *strings.Builder, object ObjectSpec) {
	entity := objectGoName(object)
	plural := exportedIdentifier(pluralize(object.Name))
	fmt.Fprintf(b, "type Create%sInput struct {\n", entity)
	writeMultiFields(b, object.Fields)
	b.WriteString("}\n")
	fmt.Fprintf(b, "type Get%sInput struct { ID string }\ntype List%sInput struct { Limit int; Offset int }\n", entity, plural)
	fmt.Fprintf(b, "type Update%sInput struct { ID string; Version uint64\n", entity)
	writeMultiFields(b, object.Fields)
	b.WriteString("}\n")
	fmt.Fprintf(b, "type Delete%sInput struct { ID string; Version uint64 }\ntype %sOutput struct { Value domain.%s `json:\"value\"` }\ntype List%sOutput struct { Items []domain.%s `json:\"items\"` }\n\n", entity, entity, entity, plural, entity)
}

func writeMultiFields(b *strings.Builder, fields []Field) {
	for _, field := range fields {
		typeName, _ := goType(field.Type)
		fmt.Fprintf(b, "\t%s %s `json:\"%s\"`\n", fieldGoName(field), typeName, jsonName(field.Name))
	}
}

func writeMultiApplicationMethods(b *strings.Builder, object ObjectSpec) {
	entity := objectGoName(object)
	plural := exportedIdentifier(pluralize(object.Name))
	fmt.Fprintf(b, "func (service *DefaultService) Create%s(ctx context.Context,input Create%sInput)(%sOutput,error){ value:=domain.%s{ID:newID(),Version:1,CreatedAt:time.Now().UTC(),UpdatedAt:time.Now().UTC(),", entity, entity, entity, entity)
	for _, field := range object.Fields {
		f := fieldGoName(field)
		fmt.Fprintf(b, "%s:input.%s,", f, f)
	}
	fmt.Fprintf(b, "}; created,err:=requestscope.ExecuteValue(ctx,service.scopes,func(scope *requestscope.Scope[ports.Repositories])(domain.%s,error){ if err:=scope.Repositories().%s.Create(scope.Context(),&value); err!=nil { return domain.%s{},err }; return value,nil }); return %sOutput{Value:created},err }\n", entity, entity, entity, entity)
	fmt.Fprintf(b, "func (service *DefaultService) Get%s(ctx context.Context,input Get%sInput)(%sOutput,error){ value,err:=requestscope.ExecuteValue(ctx,service.scopes,func(scope *requestscope.Scope[ports.Repositories])(domain.%s,error){ return scope.Repositories().%s.Get(scope.Context(),input.ID) }); return %sOutput{Value:value},err }\n", entity, entity, entity, entity, entity, entity)
	fmt.Fprintf(b, "func (service *DefaultService) List%s(ctx context.Context,input List%sInput)(List%sOutput,error){ items,err:=requestscope.ExecuteValue(ctx,service.scopes,func(scope *requestscope.Scope[ports.Repositories])([]domain.%s,error){ return scope.Repositories().%s.List(scope.Context(),input.Limit,input.Offset) }); return List%sOutput{Items:items},err }\n", plural, plural, plural, entity, entity, plural)
	fmt.Fprintf(b, "func (service *DefaultService) Update%s(ctx context.Context,input Update%sInput)(%sOutput,error){ value:=domain.%s{ID:input.ID,Version:input.Version,UpdatedAt:time.Now().UTC(),", entity, entity, entity, entity)
	for _, field := range object.Fields {
		f := fieldGoName(field)
		fmt.Fprintf(b, "%s:input.%s,", f, f)
	}
	fmt.Fprintf(b, "}; updated,err:=requestscope.ExecuteValue(ctx,service.scopes,func(scope *requestscope.Scope[ports.Repositories])(domain.%s,error){ repository:=scope.Repositories().%s; if err:=repository.Update(scope.Context(),&value,input.Version); err!=nil { return domain.%s{},err }; return repository.Get(scope.Context(),input.ID) }); return %sOutput{Value:updated},err }\n", entity, entity, entity, entity)
	fmt.Fprintf(b, "func (service *DefaultService) Delete%s(ctx context.Context,input Delete%sInput)error{ return requestscope.Execute(ctx,service.scopes,func(scope *requestscope.Scope[ports.Repositories])error{ return scope.Repositories().%s.Delete(scope.Context(),input.ID,input.Version) }) }\n\n", entity, entity, entity)
}

func multiPortsTemplate(spec Spec, packageImport string) string {
	var b strings.Builder
	b.WriteString(generatedDomainMarker + "\n\npackage ports\n\n")
	fmt.Fprintf(&b, "import (\n\t\"context\"\n\t\"errors\"\n\tdomain %q\n)\nvar ( ErrNotFound=errors.New(\"domain repository: not found\"); ErrConflict=errors.New(\"domain repository: version conflict\") )\n\n", packageImport+"/domain")
	for _, object := range spec.Objects {
		entity := objectGoName(object)
		fmt.Fprintf(&b, "type %sRepository interface { Create(context.Context,*domain.%s)error; Get(context.Context,string)(domain.%s,error); List(context.Context,int,int)([]domain.%s,error); Update(context.Context,*domain.%s,uint64)error; Delete(context.Context,string,uint64)error }\n", entity, entity, entity, entity, entity)
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
	fmt.Fprintf(&b, "import (\n\t\"time\"\n\t\"gorm.io/gorm\"\n\tdomain %q\n)\n", packageImport+"/domain")
	fmt.Fprintf(&b, "type %s struct { ID string `gorm:\"column:id;type:varchar(64);primaryKey\"`\n", base)
	if spec.TenantScoped {
		b.WriteString("\tTenantID string `gorm:\"column:tenant_id;type:varchar(64);not null;index\"`\n")
	}
	if object.POEmbedsBase {
		for _, field := range object.Fields {
			typeName, _ := goType(field.Type)
			fmt.Fprintf(&b, "\t%s %s `gorm:\"column:%s;%s\"`\n", fieldGoName(field), typeName, field.Column, columnType(field))
		}
	}
	b.WriteString("\tVersion uint64 `gorm:\"column:version;type:bigint unsigned;not null;default:1\"`\n\tCreatedAt time.Time `gorm:\"column:created_at;type:datetime(3);not null\"`\n\tUpdatedAt time.Time `gorm:\"column:updated_at;type:datetime(3);not null\"`\n\tDeletedAt gorm.DeletedAt `gorm:\"column:deleted_at;type:datetime(3);index\"`\n}\n")
	if object.POEmbedsBase {
		fmt.Fprintf(&b, "type %s struct { %s `gorm:\"embedded\"` }\n", record, po)
	} else {
		fmt.Fprintf(&b, "type %s struct { %s `gorm:\"embedded\"`; %s `gorm:\"embedded\"` }\n", record, po, base)
	}
	fmt.Fprintf(&b, "func (%s) TableName() string { return %q }\n", record, object.TableName)
	fmt.Fprintf(&b, "func (record %s) Domain() domain.%s { return domain.%s{ID:record.ID,", record, entity, entity)
	if spec.TenantScoped {
		b.WriteString("TenantID:record.TenantID,")
	}
	for _, field := range object.Fields {
		f := fieldGoName(field)
		fmt.Fprintf(&b, "%s:record.%s.%s,", f, po, f)
	}
	b.WriteString("Version:record.Version,CreatedAt:record.CreatedAt,UpdatedAt:record.UpdatedAt} }\n")
	fmt.Fprintf(&b, "func %sRecordFromDomain(value domain.%s) %s { record:=%s{}; record.ID=value.ID;", lowerFirst(entity), entity, record, record)
	if spec.TenantScoped {
		b.WriteString("record.TenantID=value.TenantID;")
	}
	b.WriteString("record.Version=value.Version; record.CreatedAt=value.CreatedAt; record.UpdatedAt=value.UpdatedAt;")
	for _, field := range object.Fields {
		f := fieldGoName(field)
		fmt.Fprintf(&b, "record.%s.%s=value.%s;", po, f, f)
	}
	b.WriteString("return record }\n")
	return b.String()
}

func multiRepositoriesTemplate(spec Spec, packageImport string) string {
	var b strings.Builder
	b.WriteString(generatedDomainMarker + "\n\npackage persistence\n\n")
	fmt.Fprintf(&b, "import (\n\t\"context\"\n\t\"errors\"\n\t\"time\"\n\t\"gorm.io/gorm\"\n\tdomain %q\n\tports %q\n\t\"yunka.io/framework/requestscope\"\n", packageImport+"/domain", packageImport+"/ports")
	if spec.TenantScoped {
		b.WriteString("\tidentity \"yunka.io/framework/core/identity\"\n")
	}
	b.WriteString(")\n")
	b.WriteString("func AutoMigrate(ctx context.Context,database *gorm.DB)error{ if database==nil { return errors.New(\"domain persistence: database is required\") }; if ctx==nil { ctx=context.Background() }; return database.WithContext(ctx).AutoMigrate(")
	for i, object := range spec.Objects {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, "&%sPORecord{}", objectGoName(object))
	}
	b.WriteString(") }\n")
	if spec.TenantScoped {
		b.WriteString("func trustedTenant(ctx context.Context)(string,error){ principal,ok:=identity.FromContext(ctx); if !ok || !principal.Authenticated || principal.TenantID==\"\" { return \"\",errors.New(\"domain persistence: trusted tenant principal is required\") }; return principal.TenantID,nil }\n")
	}
	for _, object := range spec.Objects {
		writeMultiRepository(&b, spec, object)
	}
	b.WriteString("func NewScopeFactory(database *gorm.DB)(requestscope.ScopeFactory[ports.Repositories],error){ unit,err:=requestscope.NewGORMFactory(database,nil); if err!=nil { return nil,err }; return requestscope.NewFactory(requestscope.FactoryOptions[ports.Repositories]{UnitOfWork:unit,Repositories:requestscope.GORMRepositories(func(ctx context.Context,transaction *gorm.DB)(ports.Repositories,error){ repositories:=ports.Repositories{};")
	for _, object := range spec.Objects {
		entity := objectGoName(object)
		fmt.Fprintf(&b, "repositories.%s=&%sRepository{database:transaction};", entity, entity)
	}
	b.WriteString("return repositories,nil })}) }\n")
	return b.String()
}

func writeMultiRepository(b *strings.Builder, spec Spec, object ObjectSpec) {
	entity := objectGoName(object)
	record := entity + "PORecord"
	fmt.Fprintf(b, "type %sRepository struct{ database *gorm.DB }\nfunc (repository *%sRepository) scoped(ctx context.Context)(*gorm.DB,error){ query:=repository.database.WithContext(ctx)", entity, entity)
	if spec.TenantScoped {
		b.WriteString("; tenantID,err:=trustedTenant(ctx); if err!=nil { return nil,err }; query=query.Where(\"tenant_id = ?\",tenantID)")
	}
	b.WriteString("; return query,nil }\n")
	fmt.Fprintf(b, "func (repository *%sRepository) Create(ctx context.Context,value *domain.%s)error{ if value==nil { return errors.New(\"domain persistence: value is required\") }", entity, entity)
	if spec.TenantScoped {
		b.WriteString("; tenantID,err:=trustedTenant(ctx); if err!=nil{return err}; value.TenantID=tenantID")
	}
	b.WriteString("; if value.Version==0{value.Version=1}; now:=time.Now().UTC(); if value.CreatedAt.IsZero(){value.CreatedAt=now}; value.UpdatedAt=now;")
	fmt.Fprintf(b, "record:=%sRecordFromDomain(*value); if err:=repository.database.WithContext(ctx).Create(&record).Error; err!=nil{return err}; *value=record.Domain(); return nil }\n", lowerFirst(entity))
	fmt.Fprintf(b, "func (repository *%sRepository) Get(ctx context.Context,id string)(domain.%s,error){ query,err:=repository.scoped(ctx); if err!=nil{return domain.%s{},err}; var record %s; if err:=query.Where(\"id = ?\",id).First(&record).Error; err!=nil{if errors.Is(err,gorm.ErrRecordNotFound){return domain.%s{},ports.ErrNotFound};return domain.%s{},err}; return record.Domain(),nil }\n", entity, entity, entity, record, entity, entity)
	fmt.Fprintf(b, "func (repository *%sRepository) List(ctx context.Context,limit,offset int)([]domain.%s,error){ query,err:=repository.scoped(ctx);if err!=nil{return nil,err};var rows []%s;if limit>0{query=query.Limit(limit)};if offset>0{query=query.Offset(offset)};if err:=query.Order(\"created_at DESC\").Find(&rows).Error;err!=nil{return nil,err};result:=make([]domain.%s,0,len(rows));for _,row:=range rows{result=append(result,row.Domain())};return result,nil }\n", entity, entity, record, entity)
	fmt.Fprintf(b, "func (repository *%sRepository) Update(ctx context.Context,value *domain.%s,expected uint64)error{ if value==nil||value.ID==\"\"{return errors.New(\"domain persistence: update value/id is required\")};query,err:=repository.scoped(ctx);if err!=nil{return err};updates:=map[string]any{\"version\":gorm.Expr(\"version + 1\"),\"updated_at\":time.Now().UTC(),", entity, entity)
	for _, field := range object.Fields {
		fmt.Fprintf(b, "%q:value.%s,", field.Column, fieldGoName(field))
	}
	fmt.Fprintf(b, "};result:=query.Model(&%s{}).Where(\"id = ? AND version = ?\",value.ID,expected).Updates(updates);if result.Error!=nil{return result.Error};if result.RowsAffected!=1{return ports.ErrConflict};return nil }\n", record)
	fmt.Fprintf(b, "func (repository *%sRepository) Delete(ctx context.Context,id string,expected uint64)error{query,err:=repository.scoped(ctx);if err!=nil{return err};result:=query.Where(\"id = ? AND version = ?\",id,expected).Delete(&%s{});if result.Error!=nil{return result.Error};if result.RowsAffected!=1{return ports.ErrConflict};return nil}\n", entity, record)
}

func multiRESTTemplate(spec Spec, packageImport string) string {
	var b strings.Builder
	b.WriteString(generatedDomainMarker + "\n\npackage rest\n\n")
	b.WriteString("import(\n\t\"encoding/json\"\n\t\"net/http\"\n\t\"strconv\"\n")
	if multiSpecNeedsTime(spec) {
		b.WriteString("\t\"time\"\n")
	}
	fmt.Fprintf(&b, "\tapplication %q\n)\n", packageImport+"/application")
	b.WriteString("func Register(mux *http.ServeMux,service application.Service){\n")
	for _, object := range spec.Objects {
		entity := objectGoName(object)
		path := restBasePath(spec, object)
		fmt.Fprintf(&b, "mux.HandleFunc(\"POST %s\",func(w http.ResponseWriter,r *http.Request){create%s(w,r,service)});mux.HandleFunc(\"GET %s\",func(w http.ResponseWriter,r *http.Request){list%s(w,r,service)});mux.HandleFunc(\"GET %s/{id}\",func(w http.ResponseWriter,r *http.Request){get%s(w,r,service)});mux.HandleFunc(\"PUT %s/{id}\",func(w http.ResponseWriter,r *http.Request){update%s(w,r,service)});mux.HandleFunc(\"DELETE %s/{id}\",func(w http.ResponseWriter,r *http.Request){delete%s(w,r,service)});\n", path, entity, path, entity, path, entity, path, entity, path, entity)
	}
	b.WriteString("}\n")
	for _, object := range spec.Objects {
		writeMultiRESTObject(&b, object)
	}
	b.WriteString("func writeJSON(w http.ResponseWriter,value any,err error){if err!=nil{http.Error(w,err.Error(),http.StatusBadRequest);return};w.Header().Set(\"Content-Type\",\"application/json\");_=json.NewEncoder(w).Encode(value)}\n")
	return b.String()
}

func writeMultiRESTObject(b *strings.Builder, object ObjectSpec) {
	entity := objectGoName(object)
	plural := exportedIdentifier(pluralize(object.Name))
	fmt.Fprintf(b, "type create%sRequest struct{\n", entity)
	writeMultiFields(b, object.Fields)
	b.WriteString("}\n")
	fmt.Fprintf(b, "type update%sRequest struct{Version uint64 `json:\"version\"`\n", entity)
	writeMultiFields(b, object.Fields)
	b.WriteString("}\n")
	fmt.Fprintf(b, "func create%s(w http.ResponseWriter,r *http.Request,service application.Service){var wire create%sRequest;if err:=json.NewDecoder(r.Body).Decode(&wire);err!=nil{http.Error(w,err.Error(),http.StatusBadRequest);return};output,err:=service.Create%s(r.Context(),application.Create%sInput{", entity, entity, entity, entity)
	for _, field := range object.Fields {
		f := fieldGoName(field)
		fmt.Fprintf(b, "%s:wire.%s,", f, f)
	}
	b.WriteString("});writeJSON(w,output,err)}\n")
	fmt.Fprintf(b, "func get%s(w http.ResponseWriter,r *http.Request,service application.Service){output,err:=service.Get%s(r.Context(),application.Get%sInput{ID:r.PathValue(\"id\")});writeJSON(w,output,err)}\n", entity, entity, entity)
	fmt.Fprintf(b, "func list%s(w http.ResponseWriter,r *http.Request,service application.Service){limit,_:=strconv.Atoi(r.URL.Query().Get(\"limit\"));offset,_:=strconv.Atoi(r.URL.Query().Get(\"offset\"));output,err:=service.List%s(r.Context(),application.List%sInput{Limit:limit,Offset:offset});writeJSON(w,output,err)}\n", entity, plural, plural)
	fmt.Fprintf(b, "func update%s(w http.ResponseWriter,r *http.Request,service application.Service){var wire update%sRequest;if err:=json.NewDecoder(r.Body).Decode(&wire);err!=nil{http.Error(w,err.Error(),http.StatusBadRequest);return};output,err:=service.Update%s(r.Context(),application.Update%sInput{ID:r.PathValue(\"id\"),Version:wire.Version,", entity, entity, entity, entity)
	for _, field := range object.Fields {
		f := fieldGoName(field)
		fmt.Fprintf(b, "%s:wire.%s,", f, f)
	}
	b.WriteString("});writeJSON(w,output,err)}\n")
	fmt.Fprintf(b, "func delete%s(w http.ResponseWriter,r *http.Request,service application.Service){version,_:=strconv.ParseUint(r.URL.Query().Get(\"version\"),10,64);if err:=service.Delete%s(r.Context(),application.Delete%sInput{ID:r.PathValue(\"id\"),Version:version});err!=nil{http.Error(w,err.Error(),http.StatusBadRequest);return};w.WriteHeader(http.StatusNoContent)}\n", entity, entity, entity)
}

func multiProtoTemplate(spec Spec, packageImport string) string {
	var b strings.Builder
	b.WriteString(generatedDomainMarker + "\nsyntax = \"proto3\";\n\n")
	fmt.Fprintf(&b, "package %s.v1;\nimport \"google/protobuf/timestamp.proto\";\noption go_package = %q;\nservice %s {\n", strings.ReplaceAll(spec.Domain, "_", "."), packageImport+"/transport/rpc/pb;pb", exportedIdentifier(spec.Domain)+"Service")
	for _, object := range spec.Objects {
		entity := objectGoName(object)
		plural := exportedIdentifier(pluralize(object.Name))
		fmt.Fprintf(&b, "rpc Create%s(Create%sRequest) returns (%s);rpc Get%s(Get%sRequest) returns (%s);rpc List%s(List%sRequest) returns (List%sResponse);rpc Update%s(Update%sRequest) returns (%s);rpc Delete%s(Delete%sRequest) returns (Delete%sResponse);\n", entity, entity, entity, entity, entity, entity, plural, plural, plural, entity, entity, entity, entity, entity, entity)
	}
	b.WriteString("}\n")
	for _, object := range spec.Objects {
		writeMultiProtoObject(&b, spec, object)
	}
	return b.String()
}

func writeMultiProtoObject(b *strings.Builder, spec Spec, object ObjectSpec) {
	entity := objectGoName(object)
	plural := exportedIdentifier(pluralize(object.Name))
	fmt.Fprintf(b, "message %s { string id=1001;", entity)
	if spec.TenantScoped {
		b.WriteString("string tenant_id=1002;")
	}
	b.WriteString("uint64 version=1003;google.protobuf.Timestamp created_at=1004;google.protobuf.Timestamp updated_at=1005;")
	writeMultiReserved(b, object)
	for _, field := range object.Fields {
		fmt.Fprintf(b, "%s %s=%d;", protoType(field.Type), field.Name, field.ProtoNumber)
	}
	b.WriteString("}\n")
	fmt.Fprintf(b, "message Create%sRequest{", entity)
	writeMultiReserved(b, object)
	for _, field := range object.Fields {
		fmt.Fprintf(b, "%s %s=%d;", protoType(field.Type), field.Name, field.ProtoNumber)
	}
	b.WriteString("}\n")
	fmt.Fprintf(b, "message Get%sRequest{string id=1;}message List%sRequest{int32 limit=1;int32 offset=2;}message List%sResponse{repeated %s items=1;}message Update%sRequest{string id=1001;uint64 version=1003;", entity, plural, plural, entity, entity)
	writeMultiReserved(b, object)
	for _, field := range object.Fields {
		fmt.Fprintf(b, "%s %s=%d;", protoType(field.Type), field.Name, field.ProtoNumber)
	}
	fmt.Fprintf(b, "}message Delete%sRequest{string id=1;uint64 version=2;}message Delete%sResponse{}\n", entity, entity)
}

func writeMultiReserved(b *strings.Builder, object ObjectSpec) {
	if len(object.ReservedProtoNumbers) > 0 {
		b.WriteString("reserved ")
		for i, n := range object.ReservedProtoNumbers {
			if i > 0 {
				b.WriteString(",")
			}
			fmt.Fprintf(b, "%d", n)
		}
		b.WriteString(";")
	}
	if len(object.ReservedProtoNames) > 0 {
		b.WriteString("reserved ")
		for i, n := range object.ReservedProtoNames {
			if i > 0 {
				b.WriteString(",")
			}
			fmt.Fprintf(b, "%q", n)
		}
		b.WriteString(";")
	}
}

func multiGRPCBridgeTemplate(spec Spec, packageImport string) string {
	service := exportedIdentifier(spec.Domain) + "GRPCBridge"
	var b strings.Builder
	b.WriteString(generatedDomainMarker + "\n\npackage rpc\n\n")
	b.WriteString("import(\n\t\"context\"\n")
	if multiSpecNeedsTime(spec) {
		b.WriteString("\t\"time\"\n")
	}
	b.WriteString("\tgrpc \"google.golang.org/grpc\"\n\t\"google.golang.org/protobuf/types/known/timestamppb\"\n")
	fmt.Fprintf(&b, "\tapplication %q\n\tdomain %q\n\tpb %q\n)\n", packageImport+"/application", packageImport+"/domain", packageImport+"/transport/rpc/pb")
	fmt.Fprintf(&b, "type %s struct{pb.Unimplemented%sServer;service application.Service}\nfunc Register(registrar grpc.ServiceRegistrar,service application.Service){pb.Register%sServer(registrar,&%s{service:service})}\n", service, exportedIdentifier(spec.Domain)+"Service", exportedIdentifier(spec.Domain)+"Service", service)
	for _, object := range spec.Objects {
		writeMultiGRPCMethods(&b, spec, object, service)
	}
	return b.String()
}

func writeMultiGRPCMethods(b *strings.Builder, spec Spec, object ObjectSpec, service string) {
	entity := objectGoName(object)
	plural := exportedIdentifier(pluralize(object.Name))
	fmt.Fprintf(b, "func(server *%s)Create%s(ctx context.Context,request *pb.Create%sRequest)(*pb.%s,error){output,err:=server.service.Create%s(ctx,application.Create%sInput{", service, entity, entity, entity, entity, entity)
	for _, field := range object.Fields {
		fmt.Fprintf(b, "%s:%s,", fieldGoName(field), multiProtoRequestExpr("request", field))
	}
	fmt.Fprintf(b, "});if err!=nil{return nil,err};return %sToProto(output.Value),nil}\n", lowerFirst(entity))
	fmt.Fprintf(b, "func(server *%s)Get%s(ctx context.Context,request *pb.Get%sRequest)(*pb.%s,error){output,err:=server.service.Get%s(ctx,application.Get%sInput{ID:request.GetId()});if err!=nil{return nil,err};return %sToProto(output.Value),nil}\n", service, entity, entity, entity, entity, entity, lowerFirst(entity))
	fmt.Fprintf(b, "func(server *%s)List%s(ctx context.Context,request *pb.List%sRequest)(*pb.List%sResponse,error){output,err:=server.service.List%s(ctx,application.List%sInput{Limit:int(request.GetLimit()),Offset:int(request.GetOffset())});if err!=nil{return nil,err};response:=&pb.List%sResponse{Items:make([]*pb.%s,0,len(output.Items))};for _,item:=range output.Items{response.Items=append(response.Items,%sToProto(item))};return response,nil}\n", service, plural, plural, plural, plural, plural, plural, entity, lowerFirst(entity))
	fmt.Fprintf(b, "func(server *%s)Update%s(ctx context.Context,request *pb.Update%sRequest)(*pb.%s,error){output,err:=server.service.Update%s(ctx,application.Update%sInput{ID:request.GetId(),Version:request.GetVersion(),", service, entity, entity, entity, entity, entity)
	for _, field := range object.Fields {
		fmt.Fprintf(b, "%s:%s,", fieldGoName(field), multiProtoRequestExpr("request", field))
	}
	fmt.Fprintf(b, "});if err!=nil{return nil,err};return %sToProto(output.Value),nil}\n", lowerFirst(entity))
	fmt.Fprintf(b, "func(server *%s)Delete%s(ctx context.Context,request *pb.Delete%sRequest)(*pb.Delete%sResponse,error){if err:=server.service.Delete%s(ctx,application.Delete%sInput{ID:request.GetId(),Version:request.GetVersion()});err!=nil{return nil,err};return &pb.Delete%sResponse{},nil}\n", service, entity, entity, entity, entity, entity, entity)
	fmt.Fprintf(b, "func %sToProto(value domain.%s)*pb.%s{result:=&pb.%s{Id:value.ID,Version:value.Version,CreatedAt:timestamppb.New(value.CreatedAt),UpdatedAt:timestamppb.New(value.UpdatedAt),", lowerFirst(entity), entity, entity, entity)
	if spec.TenantScoped {
		b.WriteString("TenantId:value.TenantID,")
	}
	for _, field := range object.Fields {
		goField := fieldGoName(field)
		protoField := exportedIdentifier(field.Name)
		if field.Type == "time" {
			fmt.Fprintf(b, "%s:timestamppb.New(value.%s),", protoField, goField)
		} else {
			fmt.Fprintf(b, "%s:value.%s,", protoField, goField)
		}
	}
	b.WriteString("};return result}\n")
}

func multiProtoRequestExpr(receiver string, field Field) string {
	getter := receiver + ".Get" + exportedIdentifier(field.Name) + "()"
	if field.Type == "time" {
		return "func() time.Time { if value := " + getter + "; value != nil { return value.AsTime() }; return time.Time{} }()"
	}
	return getter
}

func multiSpecNeedsTime(spec Spec) bool {
	for _, object := range spec.Objects {
		for _, field := range object.Fields {
			if field.Type == "time" {
				return true
			}
		}
	}
	return false
}

func multiWireTemplate(spec Spec, packageImport string) string {
	var b strings.Builder
	b.WriteString(generatedDomainMarker + "\n\npackage wire\n\nimport(\n\t\"context\"\n\t\"fmt\"\n")
	if spec.REST.Enabled {
		b.WriteString("\t\"net/http\"\n")
	}
	if spec.RPC.Enabled {
		b.WriteString("\tgrpc \"google.golang.org/grpc\"\n")
	}
	b.WriteString("\t\"gorm.io/gorm\"\n")
	fmt.Fprintf(&b, "\tapplication %q\n\tpersistence %q\n", packageImport+"/application", packageImport+"/infrastructure/persistence")
	if spec.REST.Enabled {
		fmt.Fprintf(&b, "\trest %q\n", packageImport+"/transport/rest")
	}
	if spec.RPC.Enabled {
		fmt.Fprintf(&b, "\trpc %q\n", packageImport+"/transport/rpc")
	}
	b.WriteString(")\ntype Bundle struct{Service application.Service}\nfunc Build(database *gorm.DB)(Bundle,error){if err:=persistence.AutoMigrate(context.Background(),database);err!=nil{return Bundle{},fmt.Errorf(\"domain wire: auto migrate: %w\",err)};scopes,err:=persistence.NewScopeFactory(database);if err!=nil{return Bundle{},err};service,err:=application.NewService(scopes);if err!=nil{return Bundle{},err};return Bundle{Service:service},nil}\n")
	if spec.REST.Enabled {
		b.WriteString("func(bundle Bundle)RegisterREST(mux *http.ServeMux){rest.Register(mux,bundle.Service)}\n")
	}
	if spec.RPC.Enabled {
		b.WriteString("func(bundle Bundle)RegisterGRPC(registrar grpc.ServiceRegistrar){rpc.Register(registrar,bundle.Service)}\n")
	}
	return b.String()
}
