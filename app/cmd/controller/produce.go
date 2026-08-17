package controller

import (
	"fmt"
	"os"
	"strings"
	"yunka.io/pkg/stringsExt"
)

/**
 * @BelongProject namei
 * @BelongPackage router
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/7/22 3:32 下午
 * @Version V1.0
 */

const (
	routerFileTmpl = `package route

/**
 *  auto generate by yunka
 */

const (
%s
)
`
	routerTmpl = `
	// Sys%s
	// %s
	// uri: %s;
	// name: %s;
	// group: %s;
	// method: %s;
	// authBit: %s;
	// button: %s;
	// engName: %s;
	// description: %s;
	// request_param: %s;
	// request_body: %s;
	// response: %s;
	Sys%s = "%s"
`

	routerRedirectTmpl = `
	// %s
	// uri: %s;
	// redirect: %s;
	// name: %s;
	// group: %s;
	// method: %s;
	// module: %s;
	// service: %s;
	// authBit: %s;
	// button: %s;
	// engName: %s;
	// description: %s;
	// request_param: %s;
	// request_body: %s;
	// response: %s;
	SysRe%s = "%s"
`
)

const (
	controllerRegisterTmpl = `package route

import (
	"%s/core"
	"%s/modules/%s/auto/controller"
)

 
func init(){
	core.RegisterInitiator(func(app *core.App) {
		%s
	})
}`
	bindParamTmpl = `
	var param %s
	if err := srv.Runtime.GetParam(&param); err != nil {
		return nil, response.IllegalParamError(err)
	}`

	bindBodyTmpl = `
	var body %s
	
	if err := srv.Runtime.GetBody(&body); err != nil {
		return nil, response.IllegalParamError(err)
	}`
	ctxParamBodyTmpl = `(param, body)`
	ctxBodyTmpl      = `(body)`
	ctxParamTmpl     = `(param)`
	ctxTmpl          = `()`

	resultTmpl = `
// %s
func %s(_srv core.Service) ([]byte, error) {
	srv := _srv.(*services.%s)
	%s
	%s
}
`
	emptyTml = `srv.%s%s
	return nil, nil`
	emptyErrResponseTml = `err := srv.%s%s
	if err != nil {
		srv.Runtime.Logger().Error("%s:", err)
		return nil, err
	}
	return response.SysSuccess, nil`

	errResponseTml = `err := srv.%s%s
	if err != nil {
		srv.Runtime.Logger().Error("%s:", err)
		return nil, err
	}
	return response.SysSuccess, nil`

	stringResponseTml = `str, err := srv.%s%s
	if err != nil {
		srv.Runtime.Logger().Error("%s:", err)
		return nil, err
	}
	return srv.ResponseString(str)`

	objectResponseTml = `obj, err := srv.%s%s
	if err != nil {
		srv.Runtime.Logger().Error("%s:", err)
		return nil, err
	}
	return srv.ResponseObject(obj)`

	arrayByteResponseTml = `
	return srv.%s%s`
)

func product(moduleName, corePkgName, flagName, serviceName, pkgName string, ans []*Annotation) {
	produceRouterFile(moduleName, flagName, serviceName, ans)
	produceController(corePkgName, moduleName, flagName, serviceName, pkgName, ans)
	produceApiRegister(corePkgName, moduleName, flagName, serviceName, ans)
}

func produceRouterFile(moduleName, flagName, serviceName string, ans []*Annotation) {

	buf := strings.Builder{}
	for _, a := range ans {
		if !a.IsParse {
			continue
		}

		if a.Redirect != `` {
			if a.ModuleName == `` || a.ServiceName == `` {
				log.Fatalf("uri:%s need module:%s & service config:%s", a.Uri, a.ModuleName, a.ServiceName)
			}
			buf.WriteString(fmt.Sprintf(routerRedirectTmpl,
				flagName,
				a.Uri,
				a.Redirect,
				a.Name,
				a.Group,
				a.Method,
				a.ModuleName,
				a.ServiceName,
				a.AuthBit,
				a.Button,
				a.EngName,
				a.Description,
				a.RequestParam,
				a.RequestBody,
				a.Response,
				a.RouterName,
				a.Uri,
			))
		} else {
			buf.WriteString(fmt.Sprintf(routerTmpl,
				a.RouterName,
				flagName,
				a.Uri,
				a.Name,
				a.Group,
				a.Method,
				a.AuthBit,
				a.Button,
				a.EngName,
				a.Description,
				a.RequestParam,
				a.RequestBody,
				a.Response,
				a.RouterName,
				a.Uri,
			))
		}

	}

	if buf.Len() == 0 {
		return
	}

	routerFile, err := os.OpenFile(
		fmt.Sprintf("modules/%s/auto/route/%s_def.go", moduleName, stringsExt.UnderscoreName(serviceName)),
		os.O_RDWR|os.O_CREATE, os.ModePerm)
	if err != nil {
		fmt.Println(err)
		return
	}
	routerFile.Truncate(0)
	defer routerFile.Close()

	routerFile.WriteString(fmt.Sprintf(routerFileTmpl, buf.String()))
}

func produceController(corePkgName, moduleName, flagName, serviceName, pkgName string, ans []*Annotation) {
	buf := strings.Builder{}

	isReq := false

	for _, a := range ans {
		if !a.IsParse {
			continue
		}

		if a.Redirect != `` {
			continue
		}

		methodName := fmt.Sprintf("%s%s", stringsExt.CamelName(serviceName),
			stringsExt.CamelName(a.ControllerName))

		if a.RequestBodyStruct == `` {
			var (
				returnTmpl    = errResponseTml
				methodTmpl    = resultTmpl
				callParamTmpl = ``
				callTmpl      = ctxTmpl
			)
			if a.RequestParamStruct != `` {
				callParamTmpl = fmt.Sprintf(bindParamTmpl,
					a.RequestParamStruct)
				callTmpl = ctxParamTmpl
				isReq = true
			} else {
				if a.ResponseType == ResponseTypeError {
					returnTmpl = emptyErrResponseTml
				} else {
					returnTmpl = emptyTml
				}
			}

			switch a.ResponseType {
			case ResponseTypeObject:
				returnTmpl = objectResponseTml
			case ResponseTypeArrayByte:
				returnTmpl = arrayByteResponseTml
				buf.WriteString(fmt.Sprintf(methodTmpl,
					a.Name, methodName,
					stringsExt.CamelName(serviceName),
					callParamTmpl,
					fmt.Sprintf(returnTmpl,
						a.ControllerName,
						callTmpl)))
				continue
			case ResponseTypeString:
				returnTmpl = stringResponseTml
			case ResponseTypeError:
			default:
				buf.WriteString(fmt.Sprintf(methodTmpl,
					a.Name, methodName,
					stringsExt.CamelName(serviceName),
					callParamTmpl,
					fmt.Sprintf(returnTmpl,
						a.ControllerName,
						callTmpl)))
				continue
			}
			buf.WriteString(fmt.Sprintf(methodTmpl,
				a.Name, methodName,
				stringsExt.CamelName(serviceName),
				callParamTmpl,
				fmt.Sprintf(returnTmpl,
					a.ControllerName,
					callTmpl, a.ControllerName)))

		} else {
			isReq = true
			var (
				returnTmpl    = errResponseTml
				methodTmpl    = resultTmpl
				callParamTmpl = fmt.Sprintf(bindBodyTmpl, a.RequestBodyStruct)
				callTmpl      = ctxBodyTmpl
			)

			if a.RequestParamStruct != `` {
				callParamTmpl = fmt.Sprintf(bindParamTmpl,
					a.RequestParamStruct) + callParamTmpl
				callTmpl = ctxParamBodyTmpl
			}

			switch a.ResponseType {
			case ResponseTypeObject:
				returnTmpl = objectResponseTml

			case ResponseTypeString:
				returnTmpl = stringResponseTml
			case ResponseTypeError:
			case ResponseTypeArrayByte:
				returnTmpl = arrayByteResponseTml
				buf.WriteString(fmt.Sprintf(methodTmpl,
					a.Name, methodName,
					stringsExt.CamelName(serviceName),
					callParamTmpl,
					fmt.Sprintf(returnTmpl,
						a.ControllerName,
						callTmpl)))
				continue
			default:
				returnTmpl = emptyTml
				buf.WriteString(fmt.Sprintf(methodTmpl,
					a.Name, methodName,
					stringsExt.CamelName(serviceName),
					callParamTmpl,
					fmt.Sprintf(returnTmpl,
						a.ControllerName,
						callTmpl)))
				continue
			}
			buf.WriteString(fmt.Sprintf(methodTmpl,
				a.Name, methodName,
				stringsExt.CamelName(serviceName),
				callParamTmpl,
				fmt.Sprintf(returnTmpl,
					a.ControllerName,
					callTmpl, a.ControllerName)))

		}

	}

	if buf.Len() == 0 {
		return
	}

	routerFile, err := os.OpenFile(
		fmt.Sprintf("modules/%s/auto/controller/%s.go", moduleName, stringsExt.UnderscoreName(serviceName)),
		os.O_RDWR|os.O_CREATE, os.ModePerm)
	if err != nil {
		fmt.Println(err)
		return
	}
	routerFile.Truncate(0)
	defer routerFile.Close()
	importStr := ``
	if isReq {
		importStr += fmt.Sprintf(`
	"%s/modules/%s/domain/dto/req"
`, flagName, moduleName)
	}

	strExtImport := "\n" + fmt.Sprintf(`	"%s/pkg/stringsExt"`, pkgName)
	responseImport := "\n" + fmt.Sprintf(`	"%s/pkg/response"`, pkgName)

	content := buf.String()

	if strings.Contains(content, "response") {
		importStr += responseImport
	}

	if strings.Contains(content, "stringsExt") {
		importStr += strExtImport
	}

	routerFile.WriteString(fmt.Sprintf(`package controller
import(
	"%s/core"%s
	"%s/modules/%s/domain/services"
)

%s
`, corePkgName, importStr, flagName,
		moduleName, content))

}

func produceApiRegister(corePkgName, moduleName, flagName, serviceName string, ans []*Annotation) {
	inserts := []string(nil)
	if len(ans) == 0 {
		return
	}
	for _, a := range ans {
		if !a.IsParse {
			continue
		}
		if a.Redirect != `` {
			continue
		}
		inserts = append(inserts, fmt.Sprintf(
			`
		// %s
		app.GetHandleTree().Insert(Sys%s, controller.%s%s)`,
			a.Name,
			stringsExt.CamelName(a.RouterName),
			stringsExt.CamelName(serviceName),
			stringsExt.CamelName(a.ControllerName)))
	}

	routerFile, err := os.OpenFile(
		fmt.Sprintf("modules/%s/auto/route/%s_register.go", moduleName, stringsExt.UnderscoreName(serviceName)),
		os.O_RDWR|os.O_CREATE, os.ModePerm)
	if err != nil {
		log.Error(err)
		return
	}
	routerFile.Truncate(0)
	routerFile.WriteString(fmt.Sprintf(controllerRegisterTmpl, corePkgName, flagName, moduleName,
		strings.Join(inserts, "\n")))
	defer routerFile.Close()

}
