package generator

import (
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"
	plugin "github.com/golang/protobuf/protoc-gen-go/plugin"
	"yunka-rpc/protobuf/pkg/naming"
)

/**
* @Description: TODO
* @date 2019-07-25
* @version V1.0
 */
func (t *mig) generateMethod(resp *plugin.CodeGeneratorResponse) {
	for _, f := range t.GenFiles {
		respFile := t.generateMethodForFile(f)
		if respFile != nil {
			resp.File = append(resp.File, respFile)
		}
	}
}

func (t *mig) generateMethodForFile(file *descriptor.FileDescriptorProto) *plugin.CodeGeneratorResponse_File {
	resp := new(plugin.CodeGeneratorResponse_File)

	t.generateFileHeader(file, t.GenPkgName)
	for _, service := range file.Service {
		t.generateMethodUri(file, service)
	}
	resp.Name = proto.String(naming.GoFileName(file, ".xr_define.go"))
	resp.Content = proto.String(t.FormattedOutput())
	t.Output.Reset()

	t.filesHandled++
	return resp
}

func (t *mig) generateMethodUri(file *descriptor.FileDescriptorProto,
	service *descriptor.ServiceDescriptorProto) {
	servName := naming.ServiceName(service)

	t.P("const (")
	for _, method := range service.Method {
		if !t.ShouldGenForMethod(file, service, method) {
			continue
		}
		methName := naming.MethodName(method)

		if file.Package != nil {
			t.P(fmt.Sprintf("	%s = `/%s.%s/%s`", methName, *file.Package, servName, methName))
		} else {
			t.P(fmt.Sprintf("	%s = `/%s/%s`", methName, servName, methName))
		}

	}
	t.P(")")
}
