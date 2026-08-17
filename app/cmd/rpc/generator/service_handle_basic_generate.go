package generator

/**
 * @BelongProject yunka-rpc
 * @BelongPackage generator
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/4/13 2:06 下午
 * @Version V1.0
 */

import (
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"
	plugin "github.com/golang/protobuf/protoc-gen-go/plugin"
	"yunka-rpc/protobuf/pkg/naming"
)

func (t *mig) generateHandleBaseImport(file *descriptor.FileDescriptorProto) {
	t.P(fmt.Sprintf(`
import (
	"errors"
	"fmt"
	"github.com/golang/protobuf/proto"
	"%s/pkg/invoke"
	"%s/pkg/trie"
	"%s/rpc/meta"
	"%s/rpc/server"
)`, t.FrameworkName, t.FrameworkName, t.ProjectName, t.ProjectName))

}

func (t *mig) generateServerHandle(resp *plugin.CodeGeneratorResponse) {
	var srvNames = make([]string, 0)
	for _, f := range t.GenFiles {
		respFile, _srvName := t.generateHandleFile(f)
		if respFile != nil {
			resp.File = append(resp.File, respFile)
			srvNames = append(srvNames, _srvName...)
		}
	}
	resp.File = append(resp.File, t.generateHandleBaseFile(srvNames))

}

func (t *mig) generateHandleBaseFile(servNames []string) *plugin.CodeGeneratorResponse_File {
	resp := new(plugin.CodeGeneratorResponse_File)
	file := new(descriptor.FileDescriptorProto)
	file.Name = &baseFileName

	resp.Name = proto.String(naming.GoFileName(file, ".xr_srv_handle.go"))
	t.generateFileHeader(file, t.GenPkgName)
	t.generateHandleBaseImport(file)

	t.P(`var (
	MethodExistErr    = errors.New("method has exist")
	MethodNotExistErr = errors.New("method not exist")
	factories         =  make(map[string]ProtoFactory)
	_ invoke.RpcServer = (*ServiceHandle)(nil)
)

type ProtoFactory func()proto.Message

func init() {
	factories = make(map[string]ProtoFactory)
}

type ServiceHandle struct {
	node    trie.Trier
}

func NewServiceHandle(t trie.Trier) *ServiceHandle{
	return &ServiceHandle{
		node: t,
	}
}
`)
	t.P()

	t.P(`func (s *ServiceHandle) RegisterServer(name string, srv interface{}) error {`)
	t.P(`	switch name {`)
	for _, value := range servNames {
		t.P(fmt.Sprintf(`	case server.%sName:`, value))
		t.P(fmt.Sprintf(`		return s.register%sServer(srv.(meta.%sServer))`, value, value))
	}
	t.P(`	}`)
	t.P(`	return errors.New(fmt.Sprintf("%s handler has not register", name))`)
	t.P(`}`)

	t.P(`
func (s *ServiceHandle) Get(method string) (invoke.SrvHandler, error){
	ihandle := s.node.Get(method)
	if ihandle == nil {
		return nil, MethodNotExistErr
	}
	return ihandle.(invoke.SrvHandler), nil
}
`)

	t.P(`func (s *ServiceHandle) register(method string, handler invoke.SrvHandler) error {
	if s.node.Get(method) != nil {
		return MethodExistErr
	}

	s.node.Put(method, handler)
	return nil
}

func (s *ServiceHandle) ModelMessage(method string) proto.Message {
	f := factories[method]
	if f == nil {
		return nil
	}
	return f()
}
`)

	resp.Content = proto.String(t.FormattedOutput())
	t.Output.Reset()

	t.filesHandled++
	return resp
}
