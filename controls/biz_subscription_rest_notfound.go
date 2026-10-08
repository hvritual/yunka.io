//go:build ignore

// Executable, read-only pressure qualifier for the actual original Biz
// GetMySubscription protobuf request/route, not a Biz application deployment.
// All generated and executable code is confined to an isolated temp directory.
package main

import (
 "context"
 "crypto/sha256"
 "encoding/hex"
 "flag"
 "fmt"
 "os"
 "os/exec"
 "path/filepath"
 "strings"
 "time"

 "github.com/hvritual/yunka.io/pkg/contract"
)

func main() {
 var bizRoot,frameworkRoot,googleRoot,protoc,goPlugin,grpcPlugin string
 flag.StringVar(&bizRoot,"biz","","immutable Biz source checkout")
 flag.StringVar(&frameworkRoot,"framework","","immutable framework candidate checkout")
 flag.StringVar(&googleRoot,"google","","pinned google.api.http include root")
 flag.StringVar(&protoc,"protoc","protoc","protoc compiler")
 flag.StringVar(&goPlugin,"go-plugin","","locked protoc-gen-go binary")
 flag.StringVar(&grpcPlugin,"grpc-plugin","","locked protoc-gen-go-grpc binary")
 flag.Parse()
 if err:=run(bizRoot,frameworkRoot,googleRoot,protoc,goPlugin,grpcPlugin);err!=nil{
  fmt.Fprintln(os.Stderr,"BIZ_SUBSCRIPTION_C9_NOTFOUND_QUALIFICATION_FAILED:",err)
  os.Exit(2)
 }
}
func run(bizRoot,frameworkRoot,googleRoot,protoc,goPlugin,grpcPlugin string) error {
 if bizRoot==""||frameworkRoot==""||googleRoot==""||goPlugin==""||grpcPlugin=="" {return fmt.Errorf("required exact checkout/toolchain arguments missing")}
 sourcePath:=filepath.Join(bizRoot,"contracts","proto","commercial","v1","subscription.proto")
 source,err:=os.ReadFile(sourcePath);if err!=nil{return err}
 if !strings.Contains(string(source), `get:"/v1/tenant/subscription"`) {
  return fmt.Errorf("source does not contain actual GetMySubscription GET route")
 }
 digest:=sha256.Sum256(source)
 root,err:=os.MkdirTemp("","biz-subscription-rest-");if err!=nil{return err};defer os.RemoveAll(root)
 protoRoot:=filepath.Join(root,"proto")
 if err=os.MkdirAll(filepath.Join(protoRoot,"commercial","v1"),0755);err!=nil{return err}
 if err=os.WriteFile(filepath.Join(protoRoot,"commercial","v1","subscription.proto"),source,0644);err!=nil{return err}
 args:=[]string{"-I",protoRoot,"-I",googleRoot,"-I",filepath.Join(frameworkRoot,"contracts","proto")}
 if extras:=os.Getenv("PROTOC_INCLUDE");extras!=""{args=append(args,"-I",extras)}
 args=append(args,
  "--plugin=protoc-gen-go="+goPlugin,
  "--plugin=protoc-gen-go-grpc="+grpcPlugin,
  "--go_out="+root,
  "--go_opt=module=github.com/hvritual/biz",
  "--go-grpc_out="+root,
  "--go-grpc_opt=module=github.com/hvritual/biz,require_unimplemented_servers=false",
  "commercial/v1/subscription.proto")
 cmd:=exec.Command(protoc,args...);cmd.Dir=protoRoot
 if output,er:=cmd.CombinedOutput();er!=nil{return fmt.Errorf("real Biz protoc failed: %w: %s",er,output)}
 ctx,cancel:=context.WithTimeout(context.Background(),12*time.Minute);defer cancel()
 compiled,err:=contract.Compile(ctx,contract.CompileOptions{
  Dir:protoRoot,Files:[]string{"commercial/v1/subscription.proto"},
  ProtoPaths:[]string{googleRoot,filepath.Join(frameworkRoot,"contracts","proto")},
  Protoc:protoc,
 })
 if err!=nil{return fmt.Errorf("compile real Biz descriptor: %w",err)}
 manifest:=compiled.Manifest
 var selected contract.Service
 serviceFound:=false
 for _,svc:=range manifest.Services{
  if svc.FullName=="commercial.v1.SubscriptionManagementApplication" {
   selected=svc;serviceFound=true;break
  }
 }
 if !serviceFound||selected.Application==nil{return fmt.Errorf("canonical Biz SubscriptionManagementApplication missing")}
 methodFound:=false
 for _,method:=range selected.Methods {
  if method.Name!="GetMySubscription"{continue}
  if len(method.HTTP)!=1||method.HTTP[0].Method!="GET"||
   method.HTTP[0].Path!="/v1/tenant/subscription" {return fmt.Errorf("Biz canonical GetMySubscription HTTP binding unexpectedly changed")}
  if method.Operation==nil||method.Operation.ID!="commercial.subscription.get_my" {return fmt.Errorf("original Biz Operation semantic identity changed")}
  selected.Methods=[]contract.Method{method};methodFound=true;break
 }
 if !methodFound{return fmt.Errorf("Biz GetMySubscription source method missing")}
 copyApp:=*selected.Application
 copyApp.Requires=nil
 copyApp.Operations=nil
 selected.Application=&copyApp
 manifest.Services=[]contract.Service{selected}
 manifest.Normalize()
 files,err:=contract.RenderC9ApplicationCode(manifest,contract.ApplicationCodeOptions{RootImport:"github.com/hvritual/biz/internal"})
 if err!=nil{return fmt.Errorf("generate scoped actual Biz C9 REST/RPC: %w",err)}
 hasMappedREST:=false
 for _,file:=range files{
  if strings.Contains(file.Path,"/transport/rest/") &&
   strings.Contains(string(file.Content),"httpbinding.WriteOperationError(writer, err)") {hasMappedREST=true}
 }
 if !hasMappedREST {return fmt.Errorf("generated Biz REST adapter does not reuse canonical error mapper")}
 if err=contract.WriteApplicationCode(filepath.Join(root,"internal"),files);err!=nil{return err}
 lines:=[]string{
 "module github.com/hvritual/biz",
 "",
 "go 1.25.0",
 "",
 "require (",
 " github.com/hvritual/yunka.io/framework v0.0.0",
 " github.com/hvritual/yunka.io/gateway v0.0.0",
 " github.com/hvritual/yunka.io/pkg v0.0.0",
 " google.golang.org/grpc v1.83.2",
 " google.golang.org/protobuf v1.36.11",
 ")",
 "replace github.com/hvritual/yunka.io/framework => "+filepath.ToSlash(filepath.Join(frameworkRoot,"framework")),
 "replace github.com/hvritual/yunka.io/gateway => "+filepath.ToSlash(filepath.Join(frameworkRoot,"gateway")),
 "replace github.com/hvritual/yunka.io/pkg => "+filepath.ToSlash(filepath.Join(frameworkRoot,"pkg")),
 }
 if err=os.WriteFile(filepath.Join(root,"go.mod"),[]byte(strings.Join(lines,"\n")+"\n"),0644);err!=nil{return err}
 if err=os.WriteFile(filepath.Join(root,"subscription_runtime_test.go"),[]byte(fixtureTest),0644);err!=nil{return err}
 cmd=exec.CommandContext(ctx,"go","test","-mod=mod","-race","-count=1","-timeout=6m","-v","./...")
 cmd.Dir=root;cmd.Env=append(os.Environ(),"GOWORK=off")
 output,err:=cmd.CombinedOutput()
 fmt.Print(string(output))
 if err!=nil{return fmt.Errorf("scoped actual Biz generated HTTP/RPC execution failed: %w",err)}
 if !strings.Contains(string(output),"--- PASS: TestOriginalBizGetMySubscriptionREST404AndRPCNotFound"){return fmt.Errorf("real Biz HTTP/RPC test was skipped")}
 fmt.Printf("BIZ_SUBSCRIPTION_C9_NOTFOUND_QUALIFIED framework=%s original_biz_sha256=%s descriptor_sha256=%s scope=scoped_real_Biz_PB_generated_HTTP_and_grpc\n",
  frameworkRoot,hex.EncodeToString(digest[:]),compiled.DescriptorSHA)
 return nil
}

const fixtureTest = "package subscriptionfixture\n\nimport (\n \"context\"\n \"fmt\"\n \"io\"\n \"net\"\n \"net/http\"\n \"net/http/httptest\"\n \"strings\"\n \"sync/atomic\"\n \"testing\"\n \"time\"\n\n commercialv1 \"github.com/hvritual/biz/contracts/gen/commercial/v1\"\n rest \"github.com/hvritual/biz/internal/commercial/transport/rest\"\n rpc \"github.com/hvritual/biz/internal/commercial/transport/rpc\"\n \"github.com/hvritual/yunka.io/framework/core/identity\"\n \"github.com/hvritual/yunka.io/framework/operation\"\n \"github.com/hvritual/yunka.io/framework/execution\"\n \"github.com/hvritual/yunka.io/gateway/authz\"\n \"google.golang.org/grpc\"\n \"google.golang.org/grpc/codes\"\n \"google.golang.org/grpc/credentials/insecure\"\n \"google.golang.org/grpc/status\"\n \"google.golang.org/grpc/test/bufconn\"\n)\n\ntype grants struct{}\nfunc (grants) HasPermissions(_ context.Context, tenant string, _ []string, _ []authz.PermissionKey, _ authz.PermissionMode) (bool,error) {return tenant==\"tenant-a\",nil}\n// A read_only Operation still requests a root UnitOfWork. This deterministic\n// test-only factory preserves that canonical pre-Application requirement.\ntype unitOfWork struct {}\nfunc (unitOfWork) Commit(context.Context) error { return nil }\nfunc (unitOfWork) Rollback(context.Context) error { return nil }\nfunc (unitOfWork) Close() error { return nil }\ntype transactionFactory struct {begins atomic.Int32}\nfunc (factory *transactionFactory) Begin(_ context.Context, mode execution.TransactionMode) (execution.UnitOfWork,error) {\n if mode!=execution.TransactionReadOnly {return nil,fmt.Errorf(\"expected read_only root transaction, got %q\",mode)}\n factory.begins.Add(1)\n return unitOfWork{},nil\n}\ntype application struct {calls atomic.Int32}\nfunc (a *application) GetMySubscription(context.Context,*commercialv1.GetMySubscriptionRequest) (*commercialv1.TenantSubscriptionDTO,error) {\n a.calls.Add(1)\n return nil,status.Error(codes.NotFound,\"private-subscription-secret-do-not-leak\")\n}\nfunc TestOriginalBizGetMySubscriptionREST404AndRPCNotFound(t *testing.T) {\n authorizer,err:=authz.NewRBACAuthorizer(grants{});if err!=nil{t.Fatal(err)}\n security,err:=authz.NewExecutionSecurity(authorizer,nil);if err!=nil{t.Fatal(err)}\n factory:=&transactionFactory{}\n executor:=operation.NewExecutorWithOptions(security,operation.ExecutorOptions{Transactions:factory});app:=&application{}\n mux:=http.NewServeMux()\n if err:=rest.RegisterOperationExecutor(mux,app,executor);err!=nil{t.Fatal(err)}\n principal:=identity.Principal{Subject:\"user\",TenantID:\"tenant-a\",UserID:\"user\",Roles:[]string{\"reader\"},AuthMethod:identity.AuthMethodAPIKey,Authenticated:true}\n server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request) {\n  if r.Header.Get(\"X-Fixture-Unauthenticated\")==\"1\" {mux.ServeHTTP(w,r);return}\n  mux.ServeHTTP(w,r.WithContext(identity.WithPrincipal(r.Context(),principal)))\n }));defer server.Close()\n req,err:=http.NewRequest(http.MethodGet,server.URL+\"/v1/tenant/subscription\",nil);if err!=nil{t.Fatal(err)}\n resp,err:=server.Client().Do(req);if err!=nil{t.Fatal(err)}\n body,err:=io.ReadAll(resp.Body);_ = resp.Body.Close();if err!=nil{t.Fatal(err)}\n t.Logf(\"Biz real C9 HTTP status=%d, application.calls=%d, response=%q\",resp.StatusCode,app.calls.Load(),body)\n if resp.StatusCode!=404||string(body)!=\"application not found\\n\"{t.Fatalf(\"Biz original GET NotFound failed: http=%d body=%q\",resp.StatusCode,body)}\n if strings.Contains(string(body),\"private-\"){t.Fatalf(\"Application detail leaked: %q\",body)}\n listener:=bufconn.Listen(1024*1024)\n grpcServer:=grpc.NewServer(grpc.UnaryInterceptor(func(ctx context.Context,req any,_ *grpc.UnaryServerInfo,next grpc.UnaryHandler)(any,error){\n  return next(identity.WithPrincipal(ctx,principal),req)\n }))\n if err:=rpc.RegisterOperationExecutor(grpcServer,app,executor);err!=nil{t.Fatal(err)}\n go func(){_ = grpcServer.Serve(listener)}();defer grpcServer.Stop()\n ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second);defer cancel()\n conn,err:=grpc.DialContext(ctx,\"bufnet\",grpc.WithContextDialer(func(ctx context.Context,_ string)(net.Conn,error){return listener.DialContext(ctx)}),grpc.WithTransportCredentials(insecure.NewCredentials()),grpc.WithBlock())\n if err!=nil{t.Fatal(err)};defer conn.Close()\n _,rpcErr:=commercialv1.NewSubscriptionManagementApplicationClient(conn).GetMySubscription(ctx,&commercialv1.GetMySubscriptionRequest{})\n if status.Code(rpcErr)!=codes.NotFound{t.Fatalf(\"actual Biz RPC status=%v, expected NotFound\",status.Code(rpcErr))}\n if app.calls.Load()!=2 || factory.begins.Load()!=2 {t.Fatalf(\"real Biz HTTP/gRPC must enter Application behind two read_only root transactions: calls=%d transactions=%d\",app.calls.Load(),factory.begins.Load())}\n before:=app.calls.Load()\n unauth,err:=http.NewRequest(http.MethodGet,server.URL+\"/v1/tenant/subscription\",nil);if err!=nil{t.Fatal(err)}\n unauth.Header.Set(\"X-Fixture-Unauthenticated\",\"1\")\n denied,err:=server.Client().Do(unauth);if err!=nil{t.Fatal(err)}\n _,_ = io.Copy(io.Discard,denied.Body);_ = denied.Body.Close()\n if denied.StatusCode!=401||app.calls.Load()!=before||factory.begins.Load()!=2{t.Fatalf(\"unauthenticated GET must be rejected before Application/transaction, status=%d\",denied.StatusCode)}\n}\n"
