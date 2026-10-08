package contract

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// This fixture executes generated adapters over HTTP and bufconn gRPC. It is
// registered in dsl-check rather than silently relying on an opt-in local run.
func TestGeneratedHTTPBindingTransportParity(t *testing.T) {
	if os.Getenv("YUNKA_REQUIRE_C9_RUNTIME") != "1" {
		t.Skip("generated HTTP binding parity is enforced by make dsl-check")
	}
	protoc, goPlugin, grpcPlugin := os.Getenv("PROTOC"), os.Getenv("PROTOC_GEN_GO"), os.Getenv("PROTOC_GEN_GO_GRPC")
	for name, path := range map[string]string{"PROTOC": protoc, "PROTOC_GEN_GO": goPlugin, "PROTOC_GEN_GO_GRPC": grpcPlugin} {
		if path == "" {
			t.Fatalf("%s is required", name)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	repositoryRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	contracts := filepath.Join(root, "contracts")
	writeC84FixtureFile(t, filepath.Join(contracts, "query.proto"), httpBindingFixtureProto)
	writeC84FixtureFile(t, filepath.Join(contracts, "http_annotations.proto"), httpBindingAnnotationFixture)
	writeC84FixtureFile(t, filepath.Join(root, "go.mod"), fmt.Sprintf(`module example.com/bindingfixture

go 1.25.0

require (
 github.com/hvritual/yunka.io/pkg v0.0.0
 github.com/hvritual/yunka.io/framework v0.0.0
 github.com/hvritual/yunka.io/gateway v0.0.0
 google.golang.org/grpc v1.83.2
 google.golang.org/protobuf v1.36.11
)
replace github.com/hvritual/yunka.io/pkg => %s
replace github.com/hvritual/yunka.io/framework => %s
replace github.com/hvritual/yunka.io/gateway => %s
`, filepath.ToSlash(filepath.Join(repositoryRoot, "pkg")), filepath.ToSlash(filepath.Join(repositoryRoot, "framework")), filepath.ToSlash(filepath.Join(repositoryRoot, "gateway"))))
	args := []string{"-I", contracts, "-I", filepath.Join(repositoryRoot, "contracts", "proto")}
	if include := standardProtoInclude(protoc); include != "" {
		args = append(args, "-I", include)
	}
	args = append(args, "--plugin=protoc-gen-go="+goPlugin, "--plugin=protoc-gen-go-grpc="+grpcPlugin, "--go_out="+root, "--go_opt=module=example.com/bindingfixture", "--go-grpc_out="+root, "--go-grpc_opt=module=example.com/bindingfixture,require_unimplemented_servers=false", "query.proto", "http_annotations.proto")
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, protoc, args...)
	cmd.Dir = contracts
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("locked protobuf generation: %v\n%s", err, output)
	}
	compiled, err := Compile(ctx, CompileOptions{Dir: contracts, ProtoPaths: []string{filepath.Join(repositoryRoot, "contracts", "proto")}, Files: []string{"query.proto"}, Protoc: protoc})
	if err != nil {
		t.Fatal(err)
	}
	if diagnostics := Lint(compiled.Manifest); HasErrors(diagnostics) {
		t.Fatalf("fixture lint: %#v", diagnostics)
	}
	files, err := RenderC9ApplicationCode(compiled.Manifest, ApplicationCodeOptions{RootImport: "example.com/bindingfixture/internal"})
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteApplicationCode(filepath.Join(root, "internal"), files); err != nil {
		t.Fatal(err)
	}
	repeated, err := RenderC9ApplicationCode(compiled.Manifest, ApplicationCodeOptions{RootImport: "example.com/bindingfixture/internal"})
	if err != nil || !reflect.DeepEqual(files, repeated) {
		t.Fatalf("adapter generation drift: %v", err)
	}
	first, err := RenderArtifacts(compiled.Manifest, ArtifactOptions{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := RenderArtifacts(compiled.Manifest, ArtifactOptions{})
	if err != nil || !reflect.DeepEqual(first, second) {
		t.Fatalf("contract artifact drift: %v", err)
	}
	writeC84FixtureFile(t, filepath.Join(root, "transport_test.go"), httpBindingFixtureTests)
	cmd = exec.CommandContext(ctx, "go", "test", "-mod=mod", "-race", "-count=1", "-timeout=90s", "-v", "./...")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOWORK=off")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated HTTP/gRPC binding parity failed: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "TestRepeatedQueryParametersReachApplication") {
		t.Fatalf("required runtime case did not execute: %s", output)
	}
	t.Logf("generated HTTP/gRPC parity with real Executor and authorization:\n%s", output)
}

// Minimal test descriptor of the standard google.api.http wire contract. It
// provides annotations to real protoc, not a second HTTP parser or transport.
const httpBindingAnnotationFixture = `syntax = "proto3";
package google.api;
import "google/protobuf/descriptor.proto";
option go_package = "example.com/bindingfixture/httpannotations;httpannotations";
message HttpRule {
 string selector = 1;
 oneof pattern { string get = 2; string put = 3; string post = 4; string delete = 5; string patch = 6; }
 string body = 7;
 repeated HttpRule additional_bindings = 11;
 string response_body = 12;
}
extend google.protobuf.MethodOptions { HttpRule http = 72295728; }
`

const httpBindingFixtureProto = `syntax = "proto3";
package query.v1;
import "yunka/dsl/v1/options.proto";
import "http_annotations.proto";
option go_package = "example.com/bindingfixture/contracts;queryv1";
option (yunka.dsl.v1.domain) = { name: "query" };
message QueryRequest {
 string tenant_id = 1;
 repeated string capability_codes = 2;
 repeated sint32 signed_values = 3;
 repeated uint64 counts = 4;
 repeated bool flags = 5;
 repeated float ratings = 6;
 repeated bytes payloads = 7;
 int32 page_size = 8;
 string display_name = 9 [json_name = "label"];
 repeated double ratios = 10;
 repeated int64 versions = 11;
}
message QueryResponse { QueryRequest received = 1; }
service QueryApplication {
 option (yunka.dsl.v1.application) = { name: "query" };
 rpc Read(QueryRequest) returns (QueryResponse) {
  option (google.api.http) = { get:"/v1/tenants/{tenant_id}/query" };
  option (yunka.dsl.v1.operation) = { id:"query.read" use_case:"read" permissions:"query.read" permission_mode:PERMISSION_ALL tenant_required:true authentication:AUTHENTICATION_JWT };
 }
 rpc Submit(QueryRequest) returns (QueryResponse) {
  option (google.api.http) = { post:"/v1/tenants/{tenant_id}/query" body:"*" };
  option (yunka.dsl.v1.operation) = { id:"query.submit" use_case:"submit" permissions:"query.read" permission_mode:PERMISSION_ALL tenant_required:true authentication:AUTHENTICATION_JWT };
 }
}
`

const httpBindingFixtureTests = `package bindingfixture

import (
 "context"
 "fmt"
 "io"
 "net"
 "net/http"
 "net/http/httptest"
 "net/url"
 "strings"
 "sync"
 "testing"
 "time"

 queryv1 "example.com/bindingfixture/contracts"
 rest "example.com/bindingfixture/internal/query/transport/rest"
 rpc "example.com/bindingfixture/internal/query/transport/rpc"
 "github.com/hvritual/yunka.io/framework/core/identity"
 "github.com/hvritual/yunka.io/framework/operation"
 "github.com/hvritual/yunka.io/gateway/authz"
 grpcgo "google.golang.org/grpc"
 "google.golang.org/grpc/codes"
 "google.golang.org/grpc/credentials/insecure"
 "google.golang.org/grpc/status"
 "google.golang.org/grpc/test/bufconn"
 "google.golang.org/protobuf/encoding/protojson"
 "google.golang.org/protobuf/proto"
)

type grants struct{}
func (grants) HasPermissions(_ context.Context, tenant string, _ []string, _ []authz.PermissionKey, _ authz.PermissionMode) (bool,error) {return tenant=="tenant-a",nil}

type application struct {mu sync.Mutex; received []*queryv1.QueryRequest}
func (a *application) accept(ctx context.Context, id string, req *queryv1.QueryRequest) (*queryv1.QueryResponse,error) {
 if _,err:=authz.RequireAuthorizedOperation(ctx,authz.OperationID(id));err!=nil{return nil,err}
 a.mu.Lock();a.received=append(a.received,proto.Clone(req).(*queryv1.QueryRequest));a.mu.Unlock()
 if req.TenantId!="tenant-a" {return nil,status.Error(codes.PermissionDenied,"resource tenant mismatch")}
 if len(req.CapabilityCodes)>128 {return nil,status.Error(codes.InvalidArgument,"too many capability codes")}
 for _,code:=range req.CapabilityCodes {if code=="unknown.capability" {return nil,status.Error(codes.InvalidArgument,"unknown capability")}}
 return &queryv1.QueryResponse{Received:proto.Clone(req).(*queryv1.QueryRequest)},nil
}
func (a *application) Read(ctx context.Context, req *queryv1.QueryRequest) (*queryv1.QueryResponse,error) {return a.accept(ctx,"query.read",req)}
func (a *application) Submit(ctx context.Context, req *queryv1.QueryRequest) (*queryv1.QueryResponse,error) {return a.accept(ctx,"query.submit",req)}
func (a *application) count() int {a.mu.Lock();defer a.mu.Unlock();return len(a.received)}

func principal() identity.Principal {return identity.Principal{Subject:"user",TenantID:"tenant-a",UserID:"user",Roles:[]string{"reader"},AuthMethod:identity.AuthMethodJWT,Authenticated:true}}

type transports struct {http *httptest.Server; rpc queryv1.QueryApplicationClient; app *application}
func start(t *testing.T, p identity.Principal) transports {
 t.Helper()
 authorizer,err:=authz.NewRBACAuthorizer(grants{});if err!=nil{t.Fatal(err)}
 security,err:=authz.NewExecutionSecurity(authorizer,nil);if err!=nil{t.Fatal(err)}
 executor:=operation.NewExecutor(security,nil);app:=&application{}
 mux:=http.NewServeMux();if err=rest.RegisterOperationExecutor(mux,app,executor);err!=nil{t.Fatal(err)}
 httpServer:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){mux.ServeHTTP(w,r.WithContext(identity.WithPrincipal(r.Context(),p)))}));t.Cleanup(httpServer.Close)
 listener:=bufconn.Listen(1024*1024)
 server:=grpcgo.NewServer(grpcgo.UnaryInterceptor(func(ctx context.Context,request any,_ *grpcgo.UnaryServerInfo,next grpcgo.UnaryHandler)(any,error){return next(identity.WithPrincipal(ctx,p),request)}))
 if err=rpc.RegisterOperationExecutor(server,app,executor);err!=nil{t.Fatal(err)}
 go func(){_ = server.Serve(listener)}();t.Cleanup(server.Stop)
 ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second);defer cancel()
 conn,err:=grpcgo.DialContext(ctx,"bufnet",grpcgo.WithContextDialer(func(ctx context.Context,_ string)(net.Conn,error){return listener.DialContext(ctx)}),grpcgo.WithTransportCredentials(insecure.NewCredentials()),grpcgo.WithBlock());if err!=nil{t.Fatal(err)};t.Cleanup(func(){_ = conn.Close()})
 return transports{http:httpServer,rpc:queryv1.NewQueryApplicationClient(conn),app:app}
}
func (tr transports) request(t *testing.T, method, rawQuery, body string) (int,*queryv1.QueryRequest) {
 t.Helper();req,err:=http.NewRequest(method,tr.http.URL+"/v1/tenants/tenant-a/query",strings.NewReader(body));if err!=nil{t.Fatal(err)};req.URL.RawQuery=rawQuery
 response,err:=tr.http.Client().Do(req);if err!=nil{t.Fatal(err)};defer response.Body.Close();data,err:=io.ReadAll(response.Body);if err!=nil{t.Fatal(err)}
 if response.StatusCode!=http.StatusOK{return response.StatusCode,nil}
 decoded:=&queryv1.QueryResponse{};if err=protojson.Unmarshal(data,decoded);err!=nil{t.Fatalf("invalid protojson response %q: %v",data,err)}
 return response.StatusCode,decoded.Received
}

func TestRepeatedQueryParametersReachApplication(t *testing.T) {
 tr:=start(t,principal())
 for _,key:=range []string{"capabilityCodes","capability_codes"} {
  t.Run(key,func(t *testing.T){
   want:=&queryv1.QueryRequest{TenantId:"tenant-a",CapabilityCodes:[]string{"coffee.read","device.read","coffee.read",""}}
   query:=url.Values{key:want.CapabilityCodes}
   code,got:=tr.request(t,"GET",query.Encode(),"")
   rpcReply,err:=tr.rpc.Read(context.Background(),want);if err!=nil{t.Fatal(err)}
   if code!=200||!proto.Equal(want,got)||!proto.Equal(rpcReply.Received,got){t.Fatalf("HTTP_BINDING_PARAMETER_LOSS: status=%d HTTP=%v gRPC=%v expected=%v",code,got,rpcReply,want)}
  })
 }
}
func TestScalarQueriesRespectTypesAndJSONNames(t *testing.T) {
 tr:=start(t,principal())
 values:=url.Values{"signedValues":{"-2147483648","2147483647"},"counts":{"0","18446744073709551615"},"flags":{"true","false"},"ratings":{"1.5","-2"},"payloads":{"YQ==","Yg","-_8="},"pageSize":{"17"},"label":{"human"},"ratios":{"1.25","-3.5"},"versions":{"-9223372036854775808","9223372036854775807"}}
 want:=&queryv1.QueryRequest{TenantId:"tenant-a",SignedValues:[]int32{-2147483648,2147483647},Counts:[]uint64{0,^uint64(0)},Flags:[]bool{true,false},Ratings:[]float32{1.5,-2},Payloads:[][]byte{[]byte("a"),[]byte("b"),{251,255}},PageSize:17,DisplayName:"human",Ratios:[]float64{1.25,-3.5},Versions:[]int64{-9223372036854775808,9223372036854775807}}
 code,got:=tr.request(t,"GET",values.Encode(),"");rpcReply,err:=tr.rpc.Read(context.Background(),want);if err!=nil{t.Fatal(err)}
 if code!=200||!proto.Equal(want,got)||!proto.Equal(got,rpcReply.Received){t.Fatalf("typed query mismatch: HTTP=%v gRPC=%v status=%d",got,rpcReply,code)}
 code,got=tr.request(t,"GET","","");if code!=200||!proto.Equal(got,&queryv1.QueryRequest{TenantId:"tenant-a"}){t.Fatalf("missing fields changed: %v",got)}
}
func TestAmbiguousOrMalformedQueriesNeverReachApplication(t *testing.T) {
 tr:=start(t,principal())
 for _,query:=range []string{"capabilityCodes=a&capability_codes=b","pageSize=1&pageSize=2","label=a&display_name=b","signedValues=2147483648","counts=-1","flags=notbool","ratings=huge","payloads=%24","pageSize=","capabilityCodes=%zz","capabilityCodes=a;bad=b"} {
  t.Run(query,func(t *testing.T){before:=tr.app.count();code,_:=tr.request(t,"GET",query,"");if code!=400||tr.app.count()!=before{t.Fatalf("invalid query reached Application: %q status=%d",query,code)}})
 }
}
func TestUnknownAndExcessValuesReachTheSameApplicationValidator(t *testing.T) {
 tr:=start(t,principal())
 for _,values:=range [][]string{{"unknown.capability"},strings.Split(strings.Repeat("coffee.read,",128)+"coffee.read",",")} {
  t.Run(fmt.Sprint(len(values)),func(t *testing.T){before:=tr.app.count();code,_:=tr.request(t,"GET",url.Values{"capabilityCodes":values}.Encode(),"");_,err:=tr.rpc.Read(context.Background(),&queryv1.QueryRequest{TenantId:"tenant-a",CapabilityCodes:values});if code!=400||status.Code(err)!=codes.InvalidArgument||tr.app.count()!=before+2{t.Fatalf("validator divergence: http=%d grpc=%v",code,err)}})
 }
}
func TestPathAndWholeBodyOwnTheirFields(t *testing.T) {
 tr:=start(t,principal())
 want:=&queryv1.QueryRequest{TenantId:"tenant-a",CapabilityCodes:[]string{"body-code"},DisplayName:"body"}
 code,got:=tr.request(t,"POST","tenant_id=tenant-b&capabilityCodes=query-code",` + "`" + `{"tenantId":"body-tenant","capabilityCodes":["body-code"],"label":"body"}` + "`" + `)
 reply,err:=tr.rpc.Submit(context.Background(),want);if err!=nil{t.Fatal(err)}
 if code!=200||!proto.Equal(got,want)||!proto.Equal(reply.Received,got){t.Fatalf("path/body precedence mismatch: status=%d got=%v",code,got)}
 code,got=tr.request(t,"GET","tenant_id=tenant-b&tenantId=tenant-c&capabilityCodes=coffee.read","")
 if code!=200||got.TenantId!="tenant-a" {t.Fatalf("query overrode path: %d %v",code,got)}
}
func TestQueryInputCannotEstablishAuthenticationOrTenantAuthority(t *testing.T) {
 for _,tc:=range []struct{name string;p identity.Principal;httpCode int;grpcCode codes.Code}{{"unauthenticated",identity.Principal{},401,codes.Unauthenticated},{"foreign-tenant",func()identity.Principal{p:=principal();p.TenantID="tenant-b";return p}(),403,codes.PermissionDenied}} {
  t.Run(tc.name,func(t *testing.T){tr:=start(t,tc.p);code,_:=tr.request(t,"GET","tenantId=tenant-a&authenticated=true&capabilityCodes=coffee.read","");_,err:=tr.rpc.Read(context.Background(),&queryv1.QueryRequest{TenantId:"tenant-a",CapabilityCodes:[]string{"coffee.read"}});if code!=tc.httpCode||status.Code(err)!=tc.grpcCode||tr.app.count()!=0{t.Fatalf("authority changed: http=%d grpc=%v application=%d",code,err,tr.app.count())}})
 }
}
`
