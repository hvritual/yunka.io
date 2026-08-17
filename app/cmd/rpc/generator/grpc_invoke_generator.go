package generator

import (
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"
	plugin "github.com/golang/protobuf/protoc-gen-go/plugin"
	"yunka-rpc/protobuf/pkg/naming"
)

var (
	baseFileName   = `base`
	clientFileName = `grpc_client`
)

func (t *mig) generateGrpcInvokeServer(resp *plugin.CodeGeneratorResponse) {

	var servNames = make([]string, 0)
	for _, f := range t.GenFiles {
		respFile := t.generateForRPCFile(f, &servNames)
		if respFile != nil {
			resp.File = append(resp.File, respFile)
		}
	}
	resp.File = append(resp.File, t.generateGRPCBaseFile(servNames))
	resp.File = append(resp.File, t.generateGRPCClientFile())
}

func (t *mig) generateGRPCClientFile() *plugin.CodeGeneratorResponse_File {
	resp := new(plugin.CodeGeneratorResponse_File)
	file := new(descriptor.FileDescriptorProto)
	file.Name = &clientFileName

	resp.Name = proto.String(naming.GoFileName(file, ".xr_g_srv.go"))
	t.generateFileHeader(file, t.GenPkgName)
	t.generateGrpcClientImport(file)

	t.P(`var (
	methodNameErr = errors.New("method not fill")
)

type ClientConn interface{
	GetConn() *grpc.ClientConn
	Close() error
}

type GetConn interface {
	Get(nodeID string, ctx context.Context) (ClientConn, error)
}

type GrpcClient struct {
	selector    selector.Selector
	getFn       GetConn
}

func NewGrpcClient(getFn GetConn, selector selector.Selector) *GrpcClient {
	if  getFn == nil {
		panic("grpc transport don't allow getFn empty")
	}
	c := &GrpcClient{
		selector:    selector,
		getFn:       getFn,
	}

	return c
}

func (m *GrpcClient) Invoke(ctx context.Context, method string, args, reply proto.Message,
	param ...interface{}) error {

	i := strings.Index(method[1:], "/")
	if i <= 0 {
		return methodNameErr
	}
	servName := method[1 : i+1]
	nxt, err := m.selector.Select(servName)
	if err != nil {
		return err
	}
	node, err := nxt()
	if err != nil {
		return err
	}

	return m.InvokeNode(ctx, node.Address, method, args, reply)
}

func (m *GrpcClient) InvokeNode(ctx context.Context, nodeId, method string, args, reply proto.Message,
	param ...interface{}) error {
	conn, err := m.getFn.Get(nodeId, ctx)
	if err != nil {
		return err
	}
	cc := conn.GetConn()
	defer conn.Close()
	return cc.Invoke(ctx, method, args, reply)
}`)

	resp.Content = proto.String(t.FormattedOutput())
	t.Output.Reset()

	t.filesHandled++
	return resp
}

func (t *mig) generateGRPCBaseFile(servNames []string) *plugin.CodeGeneratorResponse_File {
	resp := new(plugin.CodeGeneratorResponse_File)
	file := new(descriptor.FileDescriptorProto)
	file.Name = &baseFileName

	resp.Name = proto.String(naming.GoFileName(file, ".xr_g_srv.go"))
	t.generateFileHeader(file, t.GenPkgName)
	t.generateGrpcBaseImport(file)

	t.P(`type IGrpcServer interface {
	GetGrpcServer() *grpc.Server
}

type _RegisterHandler func(s IGrpcServer, srv interface{}) error

type GrpcServer struct {
	grpcSrv   *grpc.Server
	handleMap map[string]_RegisterHandler
}

func (g *GrpcServer) GetGrpcServer() *grpc.Server {
	return g.grpcSrv
}

func (g *GrpcServer) RegisterServer(name string,  srv interface{}) error {
	if srv == nil {
		return errors.New("interface is nil")
	}
	h, ok := g.handleMap[name]
	if !ok {
		return errors.New(fmt.Sprintf("%s handler has not register", name))
	}
	return h(g, srv)
}`)

	t.generateNewGrpcServer(servNames)
	resp.Content = proto.String(t.FormattedOutput())
	t.Output.Reset()

	t.filesHandled++
	return resp
}

func (t *mig) generateForRPCFile(file *descriptor.FileDescriptorProto, srvName *[]string) *plugin.CodeGeneratorResponse_File {
	resp := new(plugin.CodeGeneratorResponse_File)
	if len(file.Service) == 0 {
		return nil
	}

	t.generateFileHeader(file, t.GenPkgName)
	t.generateGrpcSrvImports(file)
	for _, service := range file.Service {
		servName := naming.ServiceName(service)
		t.generateGrpcRegister(file, servName)
		*srvName = append(*srvName, servName)
	}

	resp.Name = proto.String(naming.GoFileName(file, ".xr_g_srv.go"))
	resp.Content = proto.String(t.FormattedOutput())
	t.Output.Reset()

	t.filesHandled++
	return resp
}

func (t *mig) generateGrpcSrvImports(p *descriptor.FileDescriptorProto) {
	t.P(fmt.Sprintf(`import (
	"errors"
	"%s/rpc/meta"
)`, t.ProjectName))
}

func (t *mig) generateGrpcBaseImport(file *descriptor.FileDescriptorProto) {
	t.P(fmt.Sprintf(`import (
	"errors"
	"fmt"
	"google.golang.org/grpc"
	"%s/rpc/server"
	"%s/pkg/invoke"
)`, t.ProjectName, t.FrameworkName))
}

func (t *mig) generateGrpcClientImport(file *descriptor.FileDescriptorProto) {
	t.P(fmt.Sprintf(`import (
	"context"
	"errors"
	"google.golang.org/grpc"
	"github.com/golang/protobuf/proto"
	"strings"
	"%s/pkg/selector"
)`, t.FrameworkName))
}

func (t *mig) generateGrpcRegister(file *descriptor.FileDescriptorProto, servName string) {

	t.P()
	t.P(fmt.Sprintf(`func _Register%s(s IGrpcServer, srv interface{}) error {
	if srv == nil {
		return errors.New("interface is nil")
	}
	meta.Register%sServer(s.GetGrpcServer(), srv.(meta.%sServer))
	return nil
}`, servName, servName, servName))
	t.P()
}

func (t *mig) generateNewGrpcServer(srvName []string) {
	t.P(`func NewGrpcServer(grpcSrv *grpc.Server) invoke.RpcServer {
	return &GrpcServer{
		grpcSrv: grpcSrv,
		handleMap: map[string]_RegisterHandler{
`)
	for _, value := range srvName {
		t.P(fmt.Sprintf(`server.%sName: _Register%s,`, value, value))
	}
	t.P(`		},
	}
}`)
}
