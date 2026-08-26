package domain

import (
	"fmt"
	"strings"
)

func multiPolicyApplicationTemplate(spec Spec, packageImport string) string {
	var b strings.Builder
	b.WriteString(generatedDomainMarker + "\n\npackage application\n\n")
	fmt.Fprintf(&b, "import (\n\t\"context\"\n\t\"crypto/rand\"\n\t\"encoding/hex\"\n\t\"errors\"\n\t\"time\"\n\tdomain %q\n\tports %q\n\tidentity \"yunka.io/framework/core/identity\"\n\tpolicy \"yunka.io/framework/policy\"\n\t\"yunka.io/framework/requestscope\"\n)\n\n", packageImport+"/domain", packageImport+"/ports")

	for _, object := range spec.Objects {
		writePolicyUseCaseContract(&b, object)
	}
	b.WriteString("type Service interface {\n")
	for _, object := range spec.Objects {
		fmt.Fprintf(&b, "\t%sUseCases\n", objectGoName(object))
	}
	b.WriteString("}\n\n")

	for _, object := range spec.Objects {
		writeMultiApplicationTypes(&b, object)
		writePolicyContracts(&b, object)
	}

	b.WriteString("type serviceOptions struct {\n")
	for _, object := range spec.Objects {
		entity := objectGoName(object)
		fmt.Fprintf(&b, "\t%sPolicy %sPolicy\n\t%sRules %sRules\n", lowerFirst(entity), entity, lowerFirst(entity), entity)
	}
	b.WriteString("}\ntype Option func(*serviceOptions)\n")
	for _, object := range spec.Objects {
		entity := objectGoName(object)
		fmt.Fprintf(&b, "func With%sPolicy(value %sPolicy) Option { return func(options *serviceOptions){ options.%sPolicy=value } }\n", entity, entity, lowerFirst(entity))
		fmt.Fprintf(&b, "func With%sRules(value %sRules) Option { return func(options *serviceOptions){ options.%sRules=value } }\n", entity, entity, lowerFirst(entity))
	}
	b.WriteString("type DefaultService struct { scopes requestscope.ScopeFactory[ports.Repositories]\n")
	for _, object := range spec.Objects {
		entity := objectGoName(object)
		fmt.Fprintf(&b, "\t%sPolicy %sPolicy\n\t%sRules %sRules\n", lowerFirst(entity), entity, lowerFirst(entity), entity)
	}
	b.WriteString("}\n")
	b.WriteString("func NewService(scopes requestscope.ScopeFactory[ports.Repositories], supplied ...Option) (*DefaultService,error) { if scopes==nil{return nil,errors.New(\"domain application: request scope factory is required\")}; options:=serviceOptions{}; for _,apply:=range supplied{if apply!=nil{apply(&options)}}; service:=&DefaultService{scopes:scopes};")
	for _, object := range spec.Objects {
		entity := objectGoName(object)
		field := lowerFirst(entity)
		fmt.Fprintf(&b, "if options.%sPolicy==nil{service.%sPolicy=open%sPolicy{}}else{service.%sPolicy=options.%sPolicy};", field, field, entity, field, field)
		fmt.Fprintf(&b, "if options.%sRules==nil{service.%sRules=noop%sRules{}}else{service.%sRules=options.%sRules};", field, field, entity, field, field)
	}
	b.WriteString("return service,nil}\n\n")

	for _, object := range spec.Objects {
		writePolicyApplicationMethods(&b, object)
	}
	b.WriteString("func principalFromContext(ctx context.Context) identity.Principal { principal,_:=identity.FromContext(ctx); return principal }\n")
	b.WriteString("func newID() string { var value [16]byte; if _,err:=rand.Read(value[:]); err!=nil { panic(err) }; return hex.EncodeToString(value[:]) }\n")
	return b.String()
}

func writePolicyUseCaseContract(b *strings.Builder, object ObjectSpec) {
	entity := objectGoName(object)
	plural := exportedIdentifier(pluralize(object.Name))
	fmt.Fprintf(b, "type %sUseCases interface {\n\tCreate%s(context.Context,Create%sInput)(%sOutput,error)\n\tGet%s(context.Context,Get%sInput)(%sOutput,error)\n\tList%s(context.Context,List%sInput)(List%sOutput,error)\n\tUpdate%s(context.Context,Update%sInput)(%sOutput,error)\n\tDelete%s(context.Context,Delete%sInput)error\n}\n\n", entity, entity, entity, entity, entity, entity, entity, plural, plural, plural, entity, entity, entity, entity, entity)
}

func writePolicyContracts(b *strings.Builder, object ObjectSpec) {
	entity := objectGoName(object)
	plural := exportedIdentifier(pluralize(object.Name))
	fmt.Fprintf(b, "type %sPolicy interface {\n\tAuthorizeCreate(context.Context,identity.Principal,Create%sInput)error\n\tListScope(context.Context,identity.Principal,List%sInput)(policy.Filter,error)\n\tAuthorizeGet(context.Context,identity.Principal,domain.%s)error\n\tAuthorizeUpdate(context.Context,identity.Principal,domain.%s,Update%sInput)error\n\tAuthorizeDelete(context.Context,identity.Principal,domain.%s)error\n}\n", entity, entity, plural, entity, entity, entity, entity)
	fmt.Fprintf(b, "type %sRules interface {\n\tValidateCreate(context.Context,Create%sInput)error\n\tValidateUpdate(context.Context,domain.%s,Update%sInput)error\n\tValidateDelete(context.Context,domain.%s)error\n}\n", entity, entity, entity, entity, entity)
	fmt.Fprintf(b, "type %sAccessPolicy struct { Resolver policy.Resolver; Create policy.Rule[Create%sInput]; Read policy.Rule[domain.%s]; Update policy.Rule[domain.%s]; Delete policy.Rule[domain.%s] }\n", entity, entity, entity, entity, entity)
	fmt.Fprintf(b, "func(value %sAccessPolicy)AuthorizeCreate(ctx context.Context,principal identity.Principal,input Create%sInput)error{return value.Create.Authorize(ctx,value.Resolver,principal,input)}\n", entity, entity)
	fmt.Fprintf(b, "func(value %sAccessPolicy)ListScope(ctx context.Context,principal identity.Principal,_ List%sInput)(policy.Filter,error){return value.Read.Scope(ctx,value.Resolver,principal)}\n", entity, plural)
	fmt.Fprintf(b, "func(value %sAccessPolicy)AuthorizeGet(ctx context.Context,principal identity.Principal,current domain.%s)error{return value.Read.Authorize(ctx,value.Resolver,principal,current)}\n", entity, entity)
	fmt.Fprintf(b, "func(value %sAccessPolicy)AuthorizeUpdate(ctx context.Context,principal identity.Principal,current domain.%s,_ Update%sInput)error{return value.Update.Authorize(ctx,value.Resolver,principal,current)}\n", entity, entity, entity)
	fmt.Fprintf(b, "func(value %sAccessPolicy)AuthorizeDelete(ctx context.Context,principal identity.Principal,current domain.%s)error{return value.Delete.Authorize(ctx,value.Resolver,principal,current)}\n", entity, entity)
	fmt.Fprintf(b, "type open%sPolicy struct{}\nfunc(open%sPolicy)AuthorizeCreate(context.Context,identity.Principal,Create%sInput)error{return nil}\nfunc(open%sPolicy)ListScope(context.Context,identity.Principal,List%sInput)(policy.Filter,error){return policy.Filter{All:true},nil}\nfunc(open%sPolicy)AuthorizeGet(context.Context,identity.Principal,domain.%s)error{return nil}\nfunc(open%sPolicy)AuthorizeUpdate(context.Context,identity.Principal,domain.%s,Update%sInput)error{return nil}\nfunc(open%sPolicy)AuthorizeDelete(context.Context,identity.Principal,domain.%s)error{return nil}\n", entity, entity, entity, entity, plural, entity, entity, entity, entity, entity, entity, entity)
	fmt.Fprintf(b, "type noop%sRules struct{}\nfunc(noop%sRules)ValidateCreate(context.Context,Create%sInput)error{return nil}\nfunc(noop%sRules)ValidateUpdate(context.Context,domain.%s,Update%sInput)error{return nil}\nfunc(noop%sRules)ValidateDelete(context.Context,domain.%s)error{return nil}\n\n", entity, entity, entity, entity, entity, entity, entity, entity)
}

func writePolicyApplicationMethods(b *strings.Builder, object ObjectSpec) {
	entity := objectGoName(object)
	plural := exportedIdentifier(pluralize(object.Name))
	field := lowerFirst(entity)

	fmt.Fprintf(b, "func(service *DefaultService)Create%s(ctx context.Context,input Create%sInput)(%sOutput,error){principal:=principalFromContext(ctx);if err:=service.%sPolicy.AuthorizeCreate(ctx,principal,input);err!=nil{return %sOutput{},err};if err:=service.%sRules.ValidateCreate(ctx,input);err!=nil{return %sOutput{},err};value:=domain.%s{ID:newID(),Version:1,CreatedAt:time.Now().UTC(),UpdatedAt:time.Now().UTC(),", entity, entity, entity, field, entity, field, entity, entity)
	for _, current := range object.Fields {
		name := fieldGoName(current)
		fmt.Fprintf(b, "%s:input.%s,", name, name)
	}
	fmt.Fprintf(b, "};created,err:=requestscope.ExecuteValue(ctx,service.scopes,func(scope *requestscope.Scope[ports.Repositories])(domain.%s,error){if err:=scope.Repositories().%s.Create(scope.Context(),&value);err!=nil{return domain.%s{},err};return value,nil});return %sOutput{Value:created},err}\n", entity, entity, entity, entity)

	fmt.Fprintf(b, "func(service *DefaultService)Get%s(ctx context.Context,input Get%sInput)(%sOutput,error){principal:=principalFromContext(ctx);value,err:=requestscope.ExecuteValue(ctx,service.scopes,func(scope *requestscope.Scope[ports.Repositories])(domain.%s,error){current,err:=scope.Repositories().%s.Get(scope.Context(),input.ID);if err!=nil{return domain.%s{},err};if err:=service.%sPolicy.AuthorizeGet(scope.Context(),principal,current);err!=nil{return domain.%s{},err};return current,nil});return %sOutput{Value:value},err}\n", entity, entity, entity, entity, entity, entity, field, entity, entity)

	fmt.Fprintf(b, "func(service *DefaultService)List%s(ctx context.Context,input List%sInput)(List%sOutput,error){principal:=principalFromContext(ctx);filter,err:=service.%sPolicy.ListScope(ctx,principal,input);if err!=nil{return List%sOutput{},err};items,err:=requestscope.ExecuteValue(ctx,service.scopes,func(scope *requestscope.Scope[ports.Repositories])([]domain.%s,error){return scope.Repositories().%s.List(scope.Context(),filter,input.Limit,input.Offset)});return List%sOutput{Items:items},err}\n", plural, plural, plural, field, plural, entity, entity, plural)

	fmt.Fprintf(b, "func(service *DefaultService)Update%s(ctx context.Context,input Update%sInput)(%sOutput,error){principal:=principalFromContext(ctx);updated,err:=requestscope.ExecuteValue(ctx,service.scopes,func(scope *requestscope.Scope[ports.Repositories])(domain.%s,error){repository:=scope.Repositories().%s;current,err:=repository.Get(scope.Context(),input.ID);if err!=nil{return domain.%s{},err};if err:=service.%sPolicy.AuthorizeUpdate(scope.Context(),principal,current,input);err!=nil{return domain.%s{},err};if err:=service.%sRules.ValidateUpdate(scope.Context(),current,input);err!=nil{return domain.%s{},err};value:=current;value.Version=input.Version;value.UpdatedAt=time.Now().UTC();", entity, entity, entity, entity, entity, entity, field, entity, field, entity)
	for _, current := range object.Fields {
		name := fieldGoName(current)
		fmt.Fprintf(b, "value.%s=input.%s;", name, name)
	}
	fmt.Fprintf(b, "if err:=repository.Update(scope.Context(),&value,input.Version);err!=nil{return domain.%s{},err};return repository.Get(scope.Context(),input.ID)});return %sOutput{Value:updated},err}\n", entity, entity)

	fmt.Fprintf(b, "func(service *DefaultService)Delete%s(ctx context.Context,input Delete%sInput)error{principal:=principalFromContext(ctx);return requestscope.Execute(ctx,service.scopes,func(scope *requestscope.Scope[ports.Repositories])error{repository:=scope.Repositories().%s;current,err:=repository.Get(scope.Context(),input.ID);if err!=nil{return err};if err:=service.%sPolicy.AuthorizeDelete(scope.Context(),principal,current);err!=nil{return err};if err:=service.%sRules.ValidateDelete(scope.Context(),current);err!=nil{return err};return repository.Delete(scope.Context(),input.ID,input.Version)})}\n\n", entity, entity, entity, field, field)
}

func multiPolicyPortsTemplate(spec Spec, packageImport string) string {
	var b strings.Builder
	b.WriteString(generatedDomainMarker + "\n\npackage ports\n\n")
	fmt.Fprintf(&b, "import(\n\t\"context\"\n\t\"errors\"\n\tdomain %q\n\tpolicy \"yunka.io/framework/policy\"\n)\nvar(ErrNotFound=errors.New(\"domain repository: not found\");ErrConflict=errors.New(\"domain repository: version conflict\"))\n", packageImport+"/domain")
	for _, object := range spec.Objects {
		entity := objectGoName(object)
		fmt.Fprintf(&b, "type %sRepository interface{Create(context.Context,*domain.%s)error;Get(context.Context,string)(domain.%s,error);List(context.Context,policy.Filter,int,int)([]domain.%s,error);Update(context.Context,*domain.%s,uint64)error;Delete(context.Context,string,uint64)error}\n", entity, entity, entity, entity, entity)
	}
	b.WriteString("type Repositories struct{\n")
	for _, object := range spec.Objects {
		entity := objectGoName(object)
		fmt.Fprintf(&b, "\t%s %sRepository\n", entity, entity)
	}
	b.WriteString("}\n")
	return b.String()
}

func multiPolicyRepositoriesTemplate(spec Spec, packageImport string) string {
	var b strings.Builder
	b.WriteString(generatedDomainMarker + "\n\npackage persistence\n\n")
	fmt.Fprintf(&b, "import(\n\t\"context\"\n\t\"errors\"\n\t\"strings\"\n\t\"time\"\n\t\"gorm.io/gorm\"\n\tdomain %q\n\tports %q\n\tpolicy \"yunka.io/framework/policy\"\n\t\"yunka.io/framework/requestscope\"\n", packageImport+"/domain", packageImport+"/ports")
	if spec.TenantScoped {
		b.WriteString("\tidentity \"yunka.io/framework/core/identity\"\n")
	}
	b.WriteString(")\n")
	b.WriteString("func AutoMigrate(ctx context.Context,database *gorm.DB)error{if database==nil{return errors.New(\"domain persistence: database is required\")};if ctx==nil{ctx=context.Background()};return database.WithContext(ctx).AutoMigrate(")
	for index, object := range spec.Objects {
		if index > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, "&%sPORecord{}", objectGoName(object))
	}
	b.WriteString(")}\n")
	if spec.TenantScoped {
		b.WriteString("func trustedTenant(ctx context.Context)(string,error){principal,ok:=identity.FromContext(ctx);if !ok||!principal.Authenticated||principal.TenantID==\"\"{return \"\",errors.New(\"domain persistence: trusted tenant principal is required\")};return principal.TenantID,nil}\n")
	}
	for _, object := range spec.Objects {
		writePolicyRepository(&b, spec, object)
	}
	b.WriteString("func NewScopeFactory(database *gorm.DB)(requestscope.ScopeFactory[ports.Repositories],error){unit,err:=requestscope.NewGORMFactory(database,nil);if err!=nil{return nil,err};return requestscope.NewFactory(requestscope.FactoryOptions[ports.Repositories]{UnitOfWork:unit,Repositories:requestscope.GORMRepositories(func(ctx context.Context,transaction *gorm.DB)(ports.Repositories,error){repositories:=ports.Repositories{};")
	for _, object := range spec.Objects {
		entity := objectGoName(object)
		fmt.Fprintf(&b, "repositories.%s=&%sRepository{database:transaction};", entity, entity)
	}
	b.WriteString("return repositories,nil})})}\n")
	return b.String()
}

func writePolicyRepository(b *strings.Builder, spec Spec, object ObjectSpec) {
	entity := objectGoName(object)
	record := entity + "PORecord"
	siteColumn, ownerColumn := policyScopeColumns(object)
	fmt.Fprintf(b, "type %sRepository struct{database *gorm.DB}\nfunc(repository *%sRepository)scoped(ctx context.Context)(*gorm.DB,error){query:=repository.database.WithContext(ctx)", entity, entity)
	if spec.TenantScoped {
		b.WriteString(";tenantID,err:=trustedTenant(ctx);if err!=nil{return nil,err};query=query.Where(\"tenant_id = ?\",tenantID)")
	}
	b.WriteString(";return query,nil}\n")
	fmt.Fprintf(b, "func(repository *%sRepository)Create(ctx context.Context,value *domain.%s)error{if value==nil{return errors.New(\"domain persistence: value is required\")}", entity, entity)
	if spec.TenantScoped {
		b.WriteString(";tenantID,err:=trustedTenant(ctx);if err!=nil{return err};value.TenantID=tenantID")
	}
	b.WriteString(";if value.Version==0{value.Version=1};now:=time.Now().UTC();if value.CreatedAt.IsZero(){value.CreatedAt=now};value.UpdatedAt=now;")
	fmt.Fprintf(b, "record:=%sRecordFromDomain(*value);if err:=repository.database.WithContext(ctx).Create(&record).Error;err!=nil{return err};*value=record.Domain();return nil}\n", lowerFirst(entity))
	fmt.Fprintf(b, "func(repository *%sRepository)Get(ctx context.Context,id string)(domain.%s,error){query,err:=repository.scoped(ctx);if err!=nil{return domain.%s{},err};var record %s;if err:=query.Where(\"id = ?\",id).First(&record).Error;err!=nil{if errors.Is(err,gorm.ErrRecordNotFound){return domain.%s{},ports.ErrNotFound};return domain.%s{},err};return record.Domain(),nil}\n", entity, entity, entity, record, entity, entity)
	fmt.Fprintf(b, "func(repository *%sRepository)List(ctx context.Context,filter policy.Filter,limit,offset int)([]domain.%s,error){query,err:=repository.scoped(ctx);if err!=nil{return nil,err};filter=filter.Normalize();if !filter.All{conditions:=make([]string,0,2);args:=make([]any,0,2);", entity, entity)
	if siteColumn != "" {
		fmt.Fprintf(b, "if filter.UseSites{conditions=append(conditions,%q);args=append(args,filter.SiteIDs)};", siteColumn+" IN ?")
	} else {
		b.WriteString("if filter.UseSites{return nil,errors.New(\"domain persistence: site scope is not supported by this object\")};")
	}
	if ownerColumn != "" {
		fmt.Fprintf(b, "if filter.UseSelf{conditions=append(conditions,%q);args=append(args,filter.OwnerID)};", ownerColumn+" = ?")
	} else {
		b.WriteString("if filter.UseSelf{return nil,errors.New(\"domain persistence: self scope is not supported by this object\")};")
	}
	b.WriteString("if len(conditions)==0{return []domain.")
	b.WriteString(entity)
	b.WriteString("{},nil};query=query.Where(\"(\"+strings.Join(conditions,\" OR \")+\")\",args...)};var rows []")
	b.WriteString(record)
	b.WriteString(";if limit>0{query=query.Limit(limit)};if offset>0{query=query.Offset(offset)};if err:=query.Order(\"created_at DESC\").Find(&rows).Error;err!=nil{return nil,err};result:=make([]domain.")
	b.WriteString(entity)
	b.WriteString(",0,len(rows));for _,row:=range rows{result=append(result,row.Domain())};return result,nil}\n")
	fmt.Fprintf(b, "func(repository *%sRepository)Update(ctx context.Context,value *domain.%s,expected uint64)error{if value==nil||value.ID==\"\"{return errors.New(\"domain persistence: update value/id is required\")};query,err:=repository.scoped(ctx);if err!=nil{return err};updates:=map[string]any{\"version\":gorm.Expr(\"version + 1\"),\"updated_at\":time.Now().UTC(),", entity, entity)
	for _, current := range object.Fields {
		fmt.Fprintf(b, "%q:value.%s,", current.Column, fieldGoName(current))
	}
	fmt.Fprintf(b, "};result:=query.Model(&%s{}).Where(\"id = ? AND version = ?\",value.ID,expected).Updates(updates);if result.Error!=nil{return result.Error};if result.RowsAffected!=1{return ports.ErrConflict};return nil}\n", record)
	fmt.Fprintf(b, "func(repository *%sRepository)Delete(ctx context.Context,id string,expected uint64)error{query,err:=repository.scoped(ctx);if err!=nil{return err};result:=query.Where(\"id = ? AND version = ?\",id,expected).Delete(&%s{});if result.Error!=nil{return result.Error};if result.RowsAffected!=1{return ports.ErrConflict};return nil}\n", entity, record)
}

func policyScopeColumns(object ObjectSpec) (string, string) {
	var site, owner string
	for _, field := range object.Fields {
		switch field.Column {
		case "site_id":
			site = field.Column
		case "created_by", "owner_id":
			if owner == "" {
				owner = field.Column
			}
		}
	}
	return site, owner
}

func multiPolicyRESTTemplate(spec Spec, packageImport string) string {
	var b strings.Builder
	b.WriteString(generatedDomainMarker + "\n\npackage rest\n\n")
	b.WriteString("import(\n\t\"encoding/json\"\n\t\"errors\"\n\t\"net/http\"\n\t\"strconv\"\n")
	if multiSpecNeedsTime(spec) {
		b.WriteString("\t\"time\"\n")
	}
	fmt.Fprintf(&b, "\tapplication %q\n\tports %q\n\tpolicy \"yunka.io/framework/policy\"\n)\n", packageImport+"/application", packageImport+"/ports")
	b.WriteString("type Middleware func(http.Handler)http.Handler\nfunc Register(mux *http.ServeMux,service application.Service,middleware ...Middleware){wrap:=func(handler http.Handler)http.Handler{for index:=len(middleware)-1;index>=0;index--{if middleware[index]!=nil{handler=middleware[index](handler)}};return handler};\n")
	for _, object := range spec.Objects {
		entity := objectGoName(object)
		path := restBasePath(spec, object)
		fmt.Fprintf(&b, "mux.Handle(\"POST %s\",wrap(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){create%s(w,r,service)})));mux.Handle(\"GET %s\",wrap(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){list%s(w,r,service)})));mux.Handle(\"GET %s/{id}\",wrap(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){get%s(w,r,service)})));mux.Handle(\"PUT %s/{id}\",wrap(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){update%s(w,r,service)})));mux.Handle(\"DELETE %s/{id}\",wrap(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){delete%s(w,r,service)})));\n", path, entity, path, entity, path, entity, path, entity, path, entity)
	}
	b.WriteString("}\n")
	for _, object := range spec.Objects {
		writeMultiRESTObject(&b, object)
	}
	b.WriteString("func writeJSON(w http.ResponseWriter,value any,err error){if err!=nil{writeError(w,err);return};w.Header().Set(\"Content-Type\",\"application/json\");_=json.NewEncoder(w).Encode(value)}\nfunc writeError(w http.ResponseWriter,err error){status:=http.StatusBadRequest;switch{case errors.Is(err,policy.ErrUnauthorized):status=http.StatusUnauthorized;case errors.Is(err,policy.ErrForbidden):status=http.StatusForbidden;case errors.Is(err,ports.ErrNotFound):status=http.StatusNotFound;case errors.Is(err,ports.ErrConflict):status=http.StatusConflict};http.Error(w,http.StatusText(status),status)}\n")
	return b.String()
}

func multiPolicyGRPCBridgeTemplate(spec Spec, packageImport string) string {
	service := exportedIdentifier(spec.Domain) + "GRPCBridge"
	var b strings.Builder
	b.WriteString(generatedDomainMarker + "\n\npackage rpc\n\n")
	b.WriteString("import(\n\t\"context\"\n\t\"errors\"\n")
	if multiSpecNeedsTime(spec) {
		b.WriteString("\t\"time\"\n")
	}
	b.WriteString("\tgrpc \"google.golang.org/grpc\"\n\t\"google.golang.org/grpc/codes\"\n\t\"google.golang.org/grpc/status\"\n\t\"google.golang.org/protobuf/types/known/timestamppb\"\n")
	fmt.Fprintf(&b, "\tapplication %q\n\tdomain %q\n\tports %q\n\tpb %q\n\tpolicy \"yunka.io/framework/policy\"\n)\n", packageImport+"/application", packageImport+"/domain", packageImport+"/ports", packageImport+"/transport/rpc/pb")
	fmt.Fprintf(&b, "type %s struct{pb.Unimplemented%sServer;service application.Service}\nfunc Register(registrar grpc.ServiceRegistrar,service application.Service){pb.Register%sServer(registrar,&%s{service:service})}\n", service, exportedIdentifier(spec.Domain)+"Service", exportedIdentifier(spec.Domain)+"Service", service)
	for _, object := range spec.Objects {
		writePolicyGRPCMethods(&b, spec, object, service)
	}
	b.WriteString("func rpcError(err error)error{switch{case err==nil:return nil;case errors.Is(err,policy.ErrUnauthorized):return status.Error(codes.Unauthenticated,\"unauthenticated\");case errors.Is(err,policy.ErrForbidden):return status.Error(codes.PermissionDenied,\"permission denied\");case errors.Is(err,ports.ErrNotFound):return status.Error(codes.NotFound,\"not found\");case errors.Is(err,ports.ErrConflict):return status.Error(codes.Aborted,\"version conflict\");default:return err}}\n")
	return b.String()
}

func writePolicyGRPCMethods(b *strings.Builder, spec Spec, object ObjectSpec, service string) {
	entity := objectGoName(object)
	plural := exportedIdentifier(pluralize(object.Name))
	fmt.Fprintf(b, "func(server *%s)Create%s(ctx context.Context,request *pb.Create%sRequest)(*pb.%s,error){output,err:=server.service.Create%s(ctx,application.Create%sInput{", service, entity, entity, entity, entity, entity)
	for _, field := range object.Fields {
		fmt.Fprintf(b, "%s:%s,", fieldGoName(field), multiProtoRequestExpr("request", field))
	}
	fmt.Fprintf(b, "});if err!=nil{return nil,rpcError(err)};return %sToProto(output.Value),nil}\n", lowerFirst(entity))
	fmt.Fprintf(b, "func(server *%s)Get%s(ctx context.Context,request *pb.Get%sRequest)(*pb.%s,error){output,err:=server.service.Get%s(ctx,application.Get%sInput{ID:request.GetId()});if err!=nil{return nil,rpcError(err)};return %sToProto(output.Value),nil}\n", service, entity, entity, entity, entity, entity, lowerFirst(entity))
	fmt.Fprintf(b, "func(server *%s)List%s(ctx context.Context,request *pb.List%sRequest)(*pb.List%sResponse,error){output,err:=server.service.List%s(ctx,application.List%sInput{Limit:int(request.GetLimit()),Offset:int(request.GetOffset())});if err!=nil{return nil,rpcError(err)};response:=&pb.List%sResponse{Items:make([]*pb.%s,0,len(output.Items))};for _,item:=range output.Items{response.Items=append(response.Items,%sToProto(item))};return response,nil}\n", service, plural, plural, plural, plural, plural, plural, entity, lowerFirst(entity))
	fmt.Fprintf(b, "func(server *%s)Update%s(ctx context.Context,request *pb.Update%sRequest)(*pb.%s,error){output,err:=server.service.Update%s(ctx,application.Update%sInput{ID:request.GetId(),Version:request.GetVersion(),", service, entity, entity, entity, entity, entity)
	for _, field := range object.Fields {
		fmt.Fprintf(b, "%s:%s,", fieldGoName(field), multiProtoRequestExpr("request", field))
	}
	fmt.Fprintf(b, "});if err!=nil{return nil,rpcError(err)};return %sToProto(output.Value),nil}\n", lowerFirst(entity))
	fmt.Fprintf(b, "func(server *%s)Delete%s(ctx context.Context,request *pb.Delete%sRequest)(*pb.Delete%sResponse,error){if err:=server.service.Delete%s(ctx,application.Delete%sInput{ID:request.GetId(),Version:request.GetVersion()});err!=nil{return nil,rpcError(err)};return &pb.Delete%sResponse{},nil}\n", service, entity, entity, entity, entity, entity, entity)
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

func multiPolicyWireTemplate(spec Spec, packageImport string) string {
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
	b.WriteString(")\ntype Bundle struct{Service application.Service}\nfunc Build(database *gorm.DB,options ...application.Option)(Bundle,error){if err:=persistence.AutoMigrate(context.Background(),database);err!=nil{return Bundle{},fmt.Errorf(\"domain wire: auto migrate: %w\",err)};scopes,err:=persistence.NewScopeFactory(database);if err!=nil{return Bundle{},err};service,err:=application.NewService(scopes,options...);if err!=nil{return Bundle{},err};return Bundle{Service:service},nil}\n")
	if spec.REST.Enabled {
		b.WriteString("func(bundle Bundle)RegisterREST(mux *http.ServeMux,middleware ...rest.Middleware){rest.Register(mux,bundle.Service,middleware...)}\n")
	}
	if spec.RPC.Enabled {
		b.WriteString("func(bundle Bundle)RegisterGRPC(registrar grpc.ServiceRegistrar){rpc.Register(registrar,bundle.Service)}\n")
	}
	return b.String()
}
