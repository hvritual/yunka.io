// Behavioral tests are compiled inside the real generated HTTP/gRPC fixture.
// No second transport runtime or cross-project business-specific fixture.
package contract

const httpTransportConformanceFixture = `package bindingfixture

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	queryv1 "example.com/bindingfixture/contracts"
	"github.com/hvritual/yunka.io/framework/core/identity"
	"github.com/hvritual/yunka.io/framework/execution"
	"github.com/hvritual/yunka.io/framework/operation"
	"github.com/hvritual/yunka.io/pkg/operationplan"
	"google.golang.org/grpc/codes"
	grpcmetadata "google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// This is a test-only fault injector that still exercises the *real generated*
// REST and gRPC adapters and their canonical ExecuteTyped entry points.
// It cannot grant authorization or synthesize successful Application results.
type conformanceExecutor struct {
	wrapped operation.Executor
	unavailable atomic.Bool
}
func (runtime *conformanceExecutor) Execute(ctx context.Context, plan operationplan.Plan, input any, invoke operation.Invoker) (any, error) {
	if runtime.unavailable.Load() {
		return nil, operation.ErrExecutorUnavailable
	}
	return runtime.wrapped.Execute(ctx, plan, input, invoke)
}

func recordedConformance(t *testing.T, name string, req, resp *bool, httpStatus int, grpcStatus, diagnostic string) {
	t.Helper()
	row:=map[string]any{
		"case":name, "supported":true, "request-equivalent":req,
		"response-equivalent":resp, "httpStatus":httpStatus,
		"grpcStatus":grpcStatus, "diagnostic":diagnostic,
	}
	encoded,err:=json.Marshal(row);if err!=nil{t.Fatal(err)}
	t.Logf("CONFORMANCE_CASE:%s",encoded)
}

func equalProof() *bool { value:=true;return &value }

func requestHTTP(t *testing.T,tr transports,method,path,rawQuery,body,key string)(int,[]byte){
	t.Helper()
	req,err:=http.NewRequest(method,tr.http.URL+path,strings.NewReader(body))
	if err!=nil{t.Fatal(err)}
	req.URL.RawQuery=rawQuery
	if key!=""{req.Header.Set("Idempotency-Key",key)}
	response,err:=tr.http.Client().Do(req);if err!=nil{t.Fatal(err)}
	data,err:=io.ReadAll(response.Body);_ = response.Body.Close()
	if err!=nil{t.Fatal(err)}
	return response.StatusCode,data
}

func receivedRequests(a *application) []*queryv1.QueryRequest {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]*queryv1.QueryRequest(nil),a.received...)
}

func TestTransportConformanceSuccessfulRequestAndResponse(t *testing.T) {
	path:="/v1/tenants/tenant-a/query"
	cases:=[]struct{
		name,method,rawQuery,body string
		want *queryv1.QueryRequest
	}{
		{"get/camel-repeated-duplicate-empty","GET",
			url.Values{"capabilityCodes":{"coffee.read","coffee.read","","device.read"}}.Encode(),"",
			&queryv1.QueryRequest{TenantId:"tenant-a",CapabilityCodes:[]string{"coffee.read","coffee.read","","device.read"}}},
		{"get/snake-repeated-two","GET",
			url.Values{"capability_codes":{"one","two"}}.Encode(),"",
			&queryv1.QueryRequest{TenantId:"tenant-a",CapabilityCodes:[]string{"one","two"}}},
		{"get/omitted-values","GET","","",&queryv1.QueryRequest{TenantId:"tenant-a"}},
		{"get/scalars-and-base64","GET",
			url.Values{"signedValues":{"-2147483648","2147483647"},"counts":{"0","18446744073709551615"},
				"flags":{"true","false"},"ratings":{"1.25"},"payloads":{"YQ==","Yg"},
				"pageSize":{"17"},"label":{"visible"},"versions":{"-1","1"}}.Encode(),"",
			&queryv1.QueryRequest{TenantId:"tenant-a",SignedValues:[]int32{-2147483648,2147483647},
				Counts:[]uint64{0,^uint64(0)},Flags:[]bool{true,false},Ratings:[]float32{1.25},
				Payloads:[][]byte{[]byte("a"),[]byte("b")},PageSize:17,DisplayName:"visible",
				Versions:[]int64{-1,1}}},
		{"get/path-owns-resource","GET",
			"tenant_id=other&tenantId=other2&capabilityCodes=coffee.read","",
			&queryv1.QueryRequest{TenantId:"tenant-a",CapabilityCodes:[]string{"coffee.read"}}},
		{"get/empty-singular","GET","label=","",&queryv1.QueryRequest{TenantId:"tenant-a"}},
		{"post/body-with-path-override","POST","tenantId=other&capabilityCodes=ignored",
			"{\"tenantId\":\"body-other\",\"capabilityCodes\":[\"body-one\",\"body-two\"],\"label\":\"body\"}",
			&queryv1.QueryRequest{TenantId:"tenant-a",CapabilityCodes:[]string{"body-one","body-two"},DisplayName:"body"}},
	}
	for _,tc:=range cases{
		t.Run(tc.name,func(t *testing.T){
			tr:=start(t,principal())
			restStatus,body:=requestHTTP(t,tr,tc.method,path,tc.rawQuery,tc.body,"")
			if restStatus!=200{t.Fatalf("generated HTTP success status=%d body=%s",restStatus,body)}
			httpReply:=&queryv1.QueryResponse{}
			if err:=protojson.Unmarshal(body,httpReply);err!=nil{t.Fatal(err)}
			var grpcReply *queryv1.QueryResponse
			var err error
			if tc.method=="POST"{grpcReply,err=tr.rpc.Submit(context.Background(),tc.want)}else{grpcReply,err=tr.rpc.Read(context.Background(),tc.want)}
			if err!=nil{t.Fatal(err)}
			inputs:=receivedRequests(tr.app)
			if len(inputs)!=2 || !proto.Equal(inputs[0],tc.want) || !proto.Equal(inputs[1],tc.want){
				t.Fatalf("Application input divergence: HTTP=%v RPC=%v want=%v",inputs,inputs,tc.want)
			}
			if !proto.Equal(httpReply.Received,tc.want)||!proto.Equal(grpcReply.Received,tc.want)||
				!proto.Equal(httpReply,grpcReply){
				t.Fatalf("response semantic mismatch HTTP=%v RPC=%v",httpReply,grpcReply)
			}
			recordedConformance(t,tc.name,equalProof(),equalProof(),restStatus,codes.OK.String(),
				"real HTTP and bufconn gRPC transmitted identical protobuf input and Application response")
		})
	}
}

func TestTransportConformanceRejectsMalformedHTTPBeforeApplication(t *testing.T){
	cases:=[]struct{name,raw string}{
		{"invalid/mixed-alias","capabilityCodes=a&capability_codes=b"},
		{"invalid/singular-duplicate","pageSize=1&pageSize=2"},
		{"invalid/overflow","signedValues=2147483648"},
		{"invalid/illegal-encoding","capabilityCodes=%zz"},
		{"invalid/empty-numeric","pageSize="},
		{"invalid/invalid-bool","flags=invalid"},
	}
	for _,tc:=range cases{
		t.Run(tc.name,func(t *testing.T){
			tr:=start(t,principal())
			code,_:=requestHTTP(t,tr,http.MethodGet,"/v1/tenants/tenant-a/query",tc.raw,"","")
			if code!=http.StatusBadRequest||tr.app.count()!=0{
				t.Fatalf("malformed URL was not rejected before Application, status=%d calls=%d",code,tr.app.count())
			}
			recordedConformance(t,tc.name,nil,nil,code,"not-applicable",
				"malformed HTTP encoding cannot be expressed as a typed gRPC request; rejected without Application execution")
		})
	}
}

func TestTransportConformanceApplicationRejectionsPreserveRequestAndClassifyResponse(t *testing.T){
	cases:=[]struct{
		name,raw string
		want *queryv1.QueryRequest
		httpCode int
		grpcCode codes.Code
	}{
		{"validation/unknown-code","capabilityCodes=unknown.capability",
			&queryv1.QueryRequest{TenantId:"tenant-a",CapabilityCodes:[]string{"unknown.capability"}},400,codes.InvalidArgument},
		{"validation/129-values",url.Values{"capabilityCodes":strings.Split(strings.Repeat("coffee.read,",128)+"coffee.read",",")}.Encode(),
			&queryv1.QueryRequest{TenantId:"tenant-a",CapabilityCodes:strings.Split(strings.Repeat("coffee.read,",128)+"coffee.read",",")},400,codes.InvalidArgument},
		{"status/not-found","label=missing",&queryv1.QueryRequest{TenantId:"tenant-a",DisplayName:"missing"},404,codes.NotFound},
		{"status/aborted","label=aborted",&queryv1.QueryRequest{TenantId:"tenant-a",DisplayName:"aborted"},409,codes.Aborted},
		{"status/already-exists","label=existing",&queryv1.QueryRequest{TenantId:"tenant-a",DisplayName:"existing"},409,codes.AlreadyExists},
		{"status/internal","label=application-internal",&queryv1.QueryRequest{TenantId:"tenant-a",DisplayName:"application-internal"},400,codes.Internal},
		{"status/permission-denied","label=application-denied",&queryv1.QueryRequest{TenantId:"tenant-a",DisplayName:"application-denied"},400,codes.PermissionDenied},
		{"status/ordinary-error","label=ordinary-error",&queryv1.QueryRequest{TenantId:"tenant-a",DisplayName:"ordinary-error"},400,codes.Unknown},
	}
	for _,tc:=range cases{
		t.Run(tc.name,func(t *testing.T){
			tr:=start(t,principal())
			restStatus,restBody:=requestHTTP(t,tr,"GET","/v1/tenants/tenant-a/query",tc.raw,"","")
			_,rpcErr:=tr.rpc.Read(context.Background(),tc.want)
			if restStatus!=tc.httpCode||status.Code(rpcErr)!=tc.grpcCode {
				t.Fatalf("transport error classification diverged REST=%d RPC=%v expected %d/%v",
					restStatus,status.Code(rpcErr),tc.httpCode,tc.grpcCode)
			}
			inputs:=receivedRequests(tr.app)
			if len(inputs)!=2||!proto.Equal(inputs[0],tc.want)||!proto.Equal(inputs[1],tc.want){
				t.Fatalf("rejected Operation input differs over transports: %#v",inputs)
			}
			if strings.Contains(string(restBody),"private-")||strings.Contains(string(restBody),"resource tenant mismatch")||
				strings.Contains(string(restBody),"unknown.capability"){
				t.Fatalf("REST leaked an Application error payload: %s",restBody)
			}
			if tc.name=="status/not-found"&&string(restBody)!="application not found\n"{
				t.Fatalf("NotFound public envelope mismatch: %q",restBody)
			}
			recordedConformance(t,tc.name,equalProof(),equalProof(),restStatus,tc.grpcCode.String(),
				"Application received equivalent protobuf values; REST and gRPC preserve their explicitly mapped error categories")
		})
	}
}

func TestTransportConformanceAuthorizationDenialsDoNotInvokeApplication(t *testing.T){
	cases:=[]struct{name string;p identity.Principal;rest int;grpc codes.Code}{
		{"auth/unauthenticated",identity.Principal{},401,codes.Unauthenticated},
		{"auth/foreign-tenant",func()identity.Principal{p:=principal();p.TenantID="tenant-other";return p}(),403,codes.PermissionDenied},
	}
	for _,tc:=range cases{
		t.Run(tc.name,func(t *testing.T){
			tr:=start(t,tc.p)
			code,_:=requestHTTP(t,tr,"GET","/v1/tenants/tenant-a/query","capabilityCodes=a","","")
			_,grpcErr:=tr.rpc.Read(context.Background(),&queryv1.QueryRequest{TenantId:"tenant-a",CapabilityCodes:[]string{"a"}})
			if code!=tc.rest||status.Code(grpcErr)!=tc.grpc||tr.app.count()!=0 {
				t.Fatalf("authorization diverged: HTTP=%d RPC=%v ApplicationCalls=%d",code,status.Code(grpcErr),tr.app.count())
			}
			recordedConformance(t,tc.name,nil,equalProof(),code,tc.grpc.String(),
				"same canonical Executor denied access before Application; denial codes are transport-specific")
		})
	}
}

func idempotencyContext(key string) context.Context{
	return grpcmetadata.NewOutgoingContext(context.Background(),grpcmetadata.Pairs("idempotency-key",key))
}

func TestTransportConformanceCanonicalIdempotencyAcrossTransports(t *testing.T){
	path:="/v1/tenants/tenant-a/change"
	request:=&queryv1.QueryRequest{TenantId:"tenant-a"}
	t.Run("idempotency/missing-key",func(t *testing.T){
		tr:=start(t,principal())
		restStatus,_:=requestHTTP(t,tr,"POST",path,"","{}","")
		_,rpcErr:=tr.rpc.Change(context.Background(),request)
		if restStatus!=400||status.Code(rpcErr)!=codes.InvalidArgument||tr.app.count()!=0{
			t.Fatalf("missing key not rejected before Application HTTP=%d RPC=%v calls=%d",restStatus,status.Code(rpcErr),tr.app.count())
		}
		recordedConformance(t,"idempotency/missing-key",nil,equalProof(),restStatus,codes.InvalidArgument.String(),
			"real Executor rejects both transport attempts without idempotency key before Application")
	})
	t.Run("idempotency/in-progress",func(t *testing.T){
		tr:=start(t,principal())
		err:=tr.store.Claim(context.Background(),execution.IdempotencyIdentity{
			TenantID:"tenant-a",OperationID:"query.change",Key:"already-running",Attempt:"controlled-claim",
		})
		if err!=nil{t.Fatal(err)}
		restStatus,_:=requestHTTP(t,tr,"POST",path,"","{}","already-running")
		_,rpcErr:=tr.rpc.Change(idempotencyContext("already-running"),request)
		if restStatus!=409||status.Code(rpcErr)!=codes.Aborted||tr.app.count()!=0{
			t.Fatalf("running claim wrong HTTP=%d RPC=%v calls=%d",restStatus,status.Code(rpcErr),tr.app.count())
		}
		recordedConformance(t,"idempotency/in-progress",equalProof(),equalProof(),restStatus,codes.Aborted.String(),
			"same canonical store blocks both in-progress claims; no Application executed")
	})
	t.Run("idempotency/completed",func(t *testing.T){
		tr:=start(t,principal())
		http1,body:=requestHTTP(t,tr,"POST",path,"","{}","http-only-key")
		if http1!=200{t.Fatalf("initial HTTP operation failed %d %s",http1,body)}
		httpReply:=&queryv1.QueryResponse{}
		if err:=protojson.Unmarshal(body,httpReply);err!=nil{t.Fatal(err)}
		rpcReply,err:=tr.rpc.Change(idempotencyContext("grpc-only-key"),request)
		if err!=nil{t.Fatal(err)}
		if !proto.Equal(httpReply.Received,request)||!proto.Equal(httpReply,rpcReply) {
			t.Fatalf("idempotent successful request output diverged HTTP=%v RPC=%v",httpReply,rpcReply)
		}
		before:=tr.app.count()
		httpDuplicate,_:=requestHTTP(t,tr,"POST",path,"","{}","http-only-key")
		_,rpcErr:=tr.rpc.Change(idempotencyContext("grpc-only-key"),request)
		if before!=2||tr.app.count()!=before||httpDuplicate!=409||status.Code(rpcErr)!=codes.AlreadyExists {
			t.Fatalf("duplicate was not suppressed: before=%d after=%d HTTP=%d RPC=%v",before,tr.app.count(),httpDuplicate,status.Code(rpcErr))
		}
		recordedConformance(t,"idempotency/completed",equalProof(),equalProof(),httpDuplicate,codes.AlreadyExists.String(),
			"two successful first claims and both duplicate claims were suppressed by canonical idempotency coordinator")
	})
}

func TestTransportConformanceFrameworkUnavailableDoesNotExecuteApplication(t *testing.T){
	tr:=start(t,principal())
	tr.executor.unavailable.Store(true)
	code,body:=requestHTTP(t,tr,"GET","/v1/tenants/tenant-a/query","","","")
	_,rpcErr:=tr.rpc.Read(context.Background(),&queryv1.QueryRequest{TenantId:"tenant-a"})
	if code!=500||status.Code(rpcErr)!=codes.Internal||tr.app.count()!=0{
		t.Fatalf("Executor unavailability mismatch: HTTP=%d RPC=%v calls=%d",code,status.Code(rpcErr),tr.app.count())
	}
	if string(body)!="operation execution unavailable\n"{
		t.Fatalf("safe framework unavailable HTTP response changed: %q",body)
	}
	recordedConformance(t,"framework/security-unavailable",nil,equalProof(),code,codes.Internal.String(),
		"test-only fault injection returns real Executor sentinel; both generated adapters fail safely before Application")
}
`
