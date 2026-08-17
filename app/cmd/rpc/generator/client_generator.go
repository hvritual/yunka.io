package generator

import (
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"
	plugin "github.com/golang/protobuf/protoc-gen-go/plugin"
	"strings"
	"yunka-rpc/protobuf/pkg/naming"
)

/**
* @Description: TODO
* @date 2019-07-25
* @version V1.0
 */
func (t *mig) generateClient(resp *plugin.CodeGeneratorResponse) {
	for _, f := range t.GenFiles {
		respFile := t.generateClientForFile(f)
		if respFile != nil {
			resp.File = append(resp.File, respFile)
		}
	}
}

func (t *mig) generateClientForFile(file *descriptor.FileDescriptorProto) *plugin.CodeGeneratorResponse_File {
	resp := new(plugin.CodeGeneratorResponse_File)

	t.generateFileHeader(file, t.GenPkgName)

	t.generateClientImport(file)
	for _, service := range file.Service {
		t.generateServiceClientAndNew(file, service)
		t.generateClientInvoke(file, service)
	}
	resp.Name = proto.String(naming.GoFileName(file, ".xr_client.go"))
	resp.Content = proto.String(t.FormattedOutput())
	t.Output.Reset()

	t.filesHandled++
	return resp
}

func (t *mig) generateClientImport(file *descriptor.FileDescriptorProto) {
	if len(file.Service) == 0 {
		return
	}
	t.P(`import (`)
	t.P(`	"context"`)
	t.P(`	"sync/atomic"`)
	t.P()
	t.P(fmt.Sprintf(`	"%s/rpc/meta"`, t.ProjectName))
	t.P(fmt.Sprintf(`	"%s/rpc/method"`, t.ProjectName))
	t.P(fmt.Sprintf(`	"%s/pkg/invoke"`, t.FrameworkName))

	t.P(`)`)
	// It's legal to import a message and use it as an input or output for a
	// method. Make sure to import the package of any such message. First, dedupe
	// them.
	deps := make(map[string]string) // Map of package name to quoted import path.
	deps = t.DeduceDeps(file)
	for pkg, importPath := range deps {
		for _, service := range file.Service {
			for _, method := range service.Method {
				inputType := t.GoTypeName(method.GetInputType())
				outputType := t.GoTypeName(method.GetOutputType())
				if strings.HasPrefix(pkg, outputType) || strings.HasPrefix(pkg, inputType) {
					t.P(`import `, pkg, ` `, importPath)
				}
			}
		}
	}
	if len(deps) > 0 {
		t.P()
	}
	t.P()
}

func (t *mig) generateServiceClientAndNew(file *descriptor.FileDescriptorProto,
	service *descriptor.ServiceDescriptorProto) {
	servName := naming.ServiceName(service)

	t.P(fmt.Sprintf("// %sClient for invoke %s method ", servName, servName))
	t.P()
	t.P(fmt.Sprintf("type %sClient struct {", servName))
	t.P(fmt.Sprintf("	client invoke.RpcClient"))
	t.P(fmt.Sprintf("	idx    int64"))
	t.P(fmt.Sprintf("}"))
	t.P()
	t.P(fmt.Sprintf("func New%sClient(client invoke.RpcClient) *%sClient {", servName, servName))
	t.P(fmt.Sprintf("	return &%sClient{", servName))
	t.P(`		client: client,`)
	t.P(`		idx:    0,`)
	t.P(`	}`)
	t.P(`}`)
}

func (t *mig) generateClientInvoke(
	file *descriptor.FileDescriptorProto,
	service *descriptor.ServiceDescriptorProto) {
	servName := naming.ServiceName(service)

	for _, method := range service.Method {
		if !t.ShouldGenForMethod(file, service, method) {
			continue
		}

		methName := naming.MethodName(method)
		inputType := t.GoTypeName(method.GetInputType())
		outputType := t.GoTypeName(method.GetOutputType())

		t.P(fmt.Sprintf("func (s *%sClient) %s(cCtx context.Context, request *meta.%s) (*meta.%s, error) {",
			servName, methName, inputType, outputType))
		t.P(fmt.Sprintf("	var reply meta.%s", outputType))
		t.P("	ctx, cancel := context.WithTimeout(cCtx, invoke.RpcTimeOut)")
		t.P(fmt.Sprintf("	err := s.client.Invoke(ctx, method.%s, request, &reply)", methName))
		t.P("	cancel()")
		t.P("	return &reply, err")
		t.P("}")
		t.P()

		t.P(fmt.Sprintf("func (s *%sClient) %sSpecial(cCtx context.Context, request *meta.%s, nodeIp string) (*meta.%s, error) {",
			servName, methName, inputType, outputType))
		t.P(fmt.Sprintf("	var reply meta.%s", outputType))
		t.P("	ctx, cancel := context.WithTimeout(cCtx, invoke.RpcTimeOut)")
		t.P(fmt.Sprintf("	err := s.client.InvokeNode(ctx, nodeIp, method.%s, request, &reply)", methName))
		t.P("	cancel()")
		t.P("	return &reply, err")
		t.P("}")
		t.P()

		t.P(fmt.Sprintf("func (s *%sClient) %sCluster(cCtx context.Context, request *meta.%s, nodeIps []string) (*meta.%s, error) {",
			servName, methName, inputType, outputType))
		t.P(fmt.Sprintf(`
	var reply meta.%s
	var err error
	atomic.AddInt64(&s.idx, 1)
	lenIps := int64(len(nodeIps))
`, outputType),
			`	start := s.idx % lenIps	`,
			fmt.Sprintf(`
	
	for i := start; i < lenIps; i++ {
		ctx, cancel := context.WithTimeout(cCtx, invoke.RpcTimeOut)
		err = s.client.InvokeNode(ctx, nodeIps[i], method.%s, request, &reply)
		cancel()
		if err == nil {
			return &reply, nil
		}
		if err != context.DeadlineExceeded {
			break
		}
	}

	for i := start; i >= 0; i-- {
		ctx, cancel := context.WithTimeout(cCtx, invoke.RpcTimeOut)
		err = s.client.InvokeNode(ctx, nodeIps[i], method.%s, request, &reply)
		cancel()
		if err == nil {
			return &reply, nil
		}
		if err != context.DeadlineExceeded {
			break
		}
	}
	return &reply, err
}
`, methName, methName))
		t.P()
	}

}
