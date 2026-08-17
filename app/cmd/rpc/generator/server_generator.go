package generator

import (
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"
	plugin "github.com/golang/protobuf/protoc-gen-go/plugin"
	"yunka-rpc/protobuf/pkg/naming"
)

var (
	serverName = `server`
)

func (t *mig) generateServer(resps *plugin.CodeGeneratorResponse) {
	file := new(descriptor.FileDescriptorProto)
	file.Name = &serverName
	resp := new(plugin.CodeGeneratorResponse_File)
	resp.Name = proto.String(naming.GoFileName(file, ".xr_srv_name.go"))
	t.generateFileHeader(file, t.GenPkgName)

	t.P("const (")
	for _, f := range t.GenFiles {
		for _, value := range f.Service {
			servName := naming.ServiceName(value)
			t.P(fmt.Sprintf("	%sName        = `%s`", servName, servName))
		}
	}
	t.P(")")
	resp.Content = proto.String(t.FormattedOutput())
	t.Output.Reset()
	resps.File = append(resps.File, resp)
}
