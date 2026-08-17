package generator

import (
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"
	plugin "github.com/golang/protobuf/protoc-gen-go/plugin"
	"yunka-rpc/protobuf/pkg/naming"
)

func (t *mig) generateMemoryInvokeServer(resp *plugin.CodeGeneratorResponse) {
	resp.File = append(resp.File, t.generateMemoryBaseFile())
	resp.File = append(resp.File, t.generateMemoryClientFile())
}

var (
	memoryInvokeName = `memory`
)

func (t *mig) generateMemoryBaseImport(*descriptor.FileDescriptorProto) {
	t.P(fmt.Sprintf(`
import (
	"%s/pkg/invoke"
	"%s/pkg/trie"
	"%s/rpc/handle"
)
`, t.FrameworkName, t.FrameworkName, t.ProjectName))
}

func (t *mig) generateMemoryBaseFile() *plugin.CodeGeneratorResponse_File {
	resp := new(plugin.CodeGeneratorResponse_File)
	file := new(descriptor.FileDescriptorProto)
	file.Name = &baseFileName

	resp.Name = proto.String("memory.xr.go")
	t.generateFileHeader(file, t.GenPkgName)
	t.generateMemoryBaseImport(file)

	t.P(`
type memoryInvoke struct {
	*handle.ServiceHandle
}

func NewMemoryInvoke(t trie.Trier) invoke.Rpc {
	return &memoryInvoke{
		ServiceHandle: handle.NewServiceHandle(t),
	}
}
`)
	t.P()

	resp.Content = proto.String(t.FormattedOutput())
	t.Output.Reset()

	t.filesHandled++
	return resp
}

func (t *mig) generateMemoryClientFile() *plugin.CodeGeneratorResponse_File {
	resp := new(plugin.CodeGeneratorResponse_File)
	file := new(descriptor.FileDescriptorProto)
	file.Name = &memoryInvokeName

	resp.Name = proto.String(naming.GoFileName(file, ".xr_clt_invoke.go"))
	t.generateFileHeader(file, t.GenPkgName)

	t.P(`import (
	"context"
	"github.com/golang/protobuf/proto"
)`)
	t.P()

	t.P(`
func (m *memoryInvoke) Invoke(ctx context.Context, method string, args, reply proto.Message,
	param ...interface{}) error {

	handler, err := m.ServiceHandle.Get(method)
	if err != nil {
		return err
	}
	result, err := handler(ctx, args)
	if err != nil {
		return err
	}
	buf := proto.Buffer{}
	err = buf.EncodeMessage(result)
	if err != nil {
		return err
	}
	return buf.DecodeMessage(reply)
}

func (m *memoryInvoke) InvokeNode(ctx context.Context, nodeId, method string, args, reply proto.Message,
	param ...interface{}) error {
	return m.Invoke(ctx, method, args, reply, param...)
}`)

	resp.Content = proto.String(t.FormattedOutput())
	t.Output.Reset()

	t.filesHandled++
	return resp
}
