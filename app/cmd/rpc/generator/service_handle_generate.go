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
	"sort"
	"yunka-rpc/protobuf/pkg/naming"
	"yunka-rpc/protobuf/pkg/tag"
)

func (t *mig) generateHandleFile(file *descriptor.FileDescriptorProto) (*plugin.CodeGeneratorResponse_File, []string) {
	resp := new(plugin.CodeGeneratorResponse_File)
	t.generateFileHeader(file, t.GenPkgName)
	t.generateHandleImports(file)
	srvNames := make([]string, 0, 5)
	for _, service := range file.Service {
		t.generateHandleMethodDefine(file, service)
		t.generateHandleRegisterMethod(file, service)
		t.generateHandleHook(file, service)
		servName := naming.ServiceName(service)
		srvNames = append(srvNames, servName)
	}
	resp.Name = proto.String(naming.GoFileName(file, ".xr_srv.go"))
	resp.Content = proto.String(t.FormattedOutput())
	t.Output.Reset()

	t.filesHandled++
	return resp, srvNames
}

func (t *mig) generateHandleImports(file *descriptor.FileDescriptorProto) {
	if len(file.Service) == 0 {
		return
	}
	t.P(`import (`)
	t.P(`	"context"`)
	t.P(`	"errors"`)
	t.P(`	"sync"`)
	t.P(`	"github.com/golang/protobuf/proto"`)
	t.P(fmt.Sprintf(`	"%s/pkg/invoke"`, t.FrameworkName))
	t.P(fmt.Sprintf(`	"%s/rpc/meta"`, t.ProjectName))
	t.P(fmt.Sprintf(`	"%s/rpc/method"`, t.ProjectName))

	t.P(`)`)
	t.P()

}

func (t *mig) generateHandleMethodDefine(file *descriptor.FileDescriptorProto, service *descriptor.ServiceDescriptorProto) int {
	count := 0
	servName := naming.ServiceName(service)
	t.P("// " + servName + "Handler is the Hook  for " + servName + " service register.")

	comments, err := t.Reg.ServiceComments(file, service)
	if err == nil {
		t.PrintComments(comments)
	}
	t.P(`type `, fmt.Sprintf("_%sHook func(srv meta.%sServer) invoke.SrvHandler", servName, servName))
	t.P()
	return count
}

func (t *mig) generateHandleRegisterMethod(file *descriptor.FileDescriptorProto, service *descriptor.ServiceDescriptorProto) int {
	count := 0
	servName := naming.ServiceName(service)
	t.P("// " + servName + "Server is the server API for " + servName + " service.")

	comments, err := t.Reg.ServiceComments(file, service)
	if err == nil {
		t.PrintComments(comments)
	}
	t.P(`func (s *ServiceHandle) `, fmt.Sprintf("register%sServer(server meta.%sServer) error {", servName, servName))
	t.P(`	if server == nil {`)
	t.P("		return errors.New(`interface is nil`)")
	t.P("	}")
	t.P()
	t.P(fmt.Sprintf(`	for key, value := range _%sDescs {`, servName))
	t.P(`		err := s.register(key, value(server))`)
	t.P(`		if err != nil {`)
	t.P(`			return err`)
	t.P(`		}`)
	t.P(`	}`)
	t.P()
	t.P(`	return nil`)
	t.P(`}`)
	return count
}

func (t *mig) generateHandleHook(
	file *descriptor.FileDescriptorProto,
	service *descriptor.ServiceDescriptorProto) {
	servName := naming.ServiceName(service)
	type methodInfo struct {
		methodName string
	}
	var methList []methodInfo
	var allMidwareMap = make(map[string]bool)
	for _, method := range service.Method {
		if !t.ShouldGenForMethod(file, service, method) {
			continue
		}

		comments, _ := t.Reg.MethodComments(file, service, method)
		tags := tag.GetTagsInComment(comments.Leading)
		if tag.GetTagValue("dynamic", tags) == "true" {
			continue
		}

		methName := naming.MethodName(method)
		inputType := t.GoTypeName(method.GetInputType())

		methList = append(methList, methodInfo{
			methodName: methName,
		})

		t.P(fmt.Sprintf("func _%s (srv meta.%sServer) invoke.SrvHandler {", methName, servName))
		t.P(`	return func(ctx context.Context, args proto.Message) (reply proto.Message, err error) {`)
		t.P(fmt.Sprintf(`		reply,err = srv.%s(ctx, args.(*meta.%s))`, methName, inputType))
		t.P(fmt.Sprintf(`		_%sPool.Put(args)`, methName))
		t.P(fmt.Sprintf(`		return reply, err`))
		t.P(`	}`)
		t.P(`}`)
		t.P(fmt.Sprintf(`
var (
	_%sPool = sync.Pool{New: func() interface{} {
		return &meta.%s{}
	}}

	_%sPoolFactory = func() proto.Message {
		return _%sPool.Get().(proto.Message)
	}
)`, methName, inputType, methName, methName))
	}

	// generate route group
	var midList []string
	for m := range allMidwareMap {
		midList = append(midList, m+" mig.HandlerFunc")
	}

	sort.Strings(midList)

	// 服务自动注册
	t.P(`// register invoke server to hooks`)
	t.P(`var (`)
	t.P(fmt.Sprintf(`	 _%sDescs =map[string]_%sHook{`, servName, servName))

	for _, methInfo := range methList {
		t.P(fmt.Sprintf(`		method.%s:_%s,`, methInfo.methodName, methInfo.methodName))
	}

	t.P(`	}`)
	t.P(`)`)
	t.P()
	t.P(`func init() {`)
	for _, methInfo := range methList {
		t.P(fmt.Sprintf(`	factories[method.%s] = _%sPoolFactory`, methInfo.methodName, methInfo.methodName))
	}

	t.P(`}`)

}
