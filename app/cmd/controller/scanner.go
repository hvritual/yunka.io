package controller

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"yunka.io/pkg/di"
	"yunka.io/pkg/stringsExt"
)

const (
	parseFlag    = `@api:[`
	parseEndFlag = `];`
	fieldFlag    = `;`
	kvFlag       = `:`
)

var (
	parseFlagLen = len(parseFlag)
	mustKws      = []KeyWord{kwGroup, kwEngName, kwMethod, kwDescription}
)

type Annotation struct {
	IsParse            bool
	Redirect           string
	Uri                string
	RouterName         string
	ModuleName         string
	ServiceName        string
	ControllerName     string
	Name               string
	EngName            string
	Group              string
	Method             string
	IsCommentStart     bool //当前方法注释是否被解析开始
	AuthBit            string
	Button             string
	Description        string
	RequestParam       string
	RequestBody        string
	Response           string
	RequestParamStruct string
	RequestBodyStruct  string
	ResponseType       ResponseType
}
type fillParam struct {
	Version     string
	ModuleName  string
	ServiceName string
	Value       string
}

type fillAnnotation func(annotation *Annotation, param *fillParam)

type (
	KeyWord      string
	ResponseType string
)

const (
	kwAuth         KeyWord = `auth`
	kwUri          KeyWord = `uri`
	kwButton       KeyWord = `button`
	kwName         KeyWord = `name`
	kwCommon       KeyWord = `common`
	kwRedirect     KeyWord = `redirect`
	kwModule       KeyWord = `module`
	kwService      KeyWord = `service`
	kwMethod       KeyWord = `method`
	kwDescription  KeyWord = `description`
	kwEngName      KeyWord = `engName`
	kwGroup        KeyWord = `group`
	kwRequestParam KeyWord = `request_param`
	kwRequestBody  KeyWord = `request_body`
	kwResponse     KeyWord = `response`

	ResponseTypeString    ResponseType = `string`
	ResponseTypeObject    ResponseType = `object`
	ResponseTypeArrayByte ResponseType = `byte`
	ResponseTypeError     ResponseType = `error`
)

type AuthBit uint32

const (
	AuthBitAuthNot   AuthBit = 0
	AuthBitAuthToken AuthBit = 1 << iota
	AuthBitAuthRole
	AuthBitAuthForce
	AuthBitAuthApi
)

var (
	authBitAuthString = []string{
		`not`,
		`token`,
		`role`,
		`force`,
		`api`,
	}
)

func ParseAuthBitStr(s string) AuthBit {
	arr := strings.Split(s, `|`)
	b := AuthBitAuthNot
	for _, a := range arr {
		for i := 0; i < len(authBitAuthString); i++ {
			if authBitAuthString[i] == a {
				b |= 1 << i
			}
		}
	}
	return b
}

var (
	handlers = map[KeyWord]fillAnnotation{
		kwAuth: func(a *Annotation, p *fillParam) {
			a.AuthBit = p.Value
		},
		kwUri: func(a *Annotation, p *fillParam) {
			a.Uri = p.Value
		},
		kwButton: func(a *Annotation, p *fillParam) {
			a.Button = p.Value
		},
		kwEngName: func(a *Annotation, p *fillParam) {
			if p.Value != `` {
				a.EngName = p.Value
			} else {
				a.EngName = a.Uri[strings.LastIndex(a.Uri, `/`)+1:]
			}
		},
		kwMethod: func(a *Annotation, p *fillParam) {
			if p.Value != `` {
				a.Method = p.Value
			} else {
				a.Method = `get|post|put|delete`
			}
		},
		kwRedirect: func(a *Annotation, p *fillParam) {
			if p.Value != `` {
				a.Redirect = p.Value
			}
		},
		kwName: func(a *Annotation, p *fillParam) {
			a.Name = p.Value
		},
		kwModule: func(a *Annotation, p *fillParam) {
			a.ModuleName = p.Value
		},
		kwService: func(a *Annotation, p *fillParam) {
			a.ServiceName = p.Value
		},
		kwDescription: func(a *Annotation, p *fillParam) {
			if p.Value != `` {
				a.Description = p.Value
			} else {
				a.Description = a.Name
			}
		},
		kwGroup: func(a *Annotation, p *fillParam) {
			if p.Value != `` {
				a.Group = p.Value
			} else {
				a.Group = fmt.Sprintf("%s_%s", p.ModuleName, p.ServiceName)
			}
		},
		kwRequestParam: func(a *Annotation, p *fillParam) {

			params_transfer(a, p, &a.RequestParam, ``)
		},
		kwRequestBody: func(a *Annotation, p *fillParam) {
			params_transfer(a, p, &a.RequestBody, `RequestBody`)
		},
		kwResponse: func(a *Annotation, p *fillParam) {
			params_transfer(a, p, &a.Response, ``)
		},
	}
)

func params_transfer(a *Annotation, p *fillParam, save *string, objType string) {
	p.Value = strings.Replace(p.Value, "request/", "", -1)
	p.Value = strings.Replace(p.Value, "bean/", "", -1)
	//switch p.Value {
	//case `@a_`, `@o_`:
	//	controllerName := a.Uri[:strings.Index(a.Uri, `/`)]
	//	paramName := stringsExt.CamelNameFlag(a.Uri, `/`)
	//	*save = fmt.Sprintf(`%s%s/%s/request/%s/%s%s`,
	//		p.Value, p.ModuleDirName, p.ModuleName, controllerName, paramName, objType)
	//case `@c_`:
	//	a.Response = `[{"description":"合计", "name":"data", "type":"String"}]`
	//
	//default:
	//	if strings.Index(p.Value, `@s_`) != -1 {
	//		a.Response = fmt.Sprintf(`[{"description":"数据", "name":"data", "type":"%s"}]`,
	//			strings.Replace(p.Value, `@s_`, ``, -1))
	//		return
	//	}
	//	if strings.Index(p.Value, `@ab_`) != -1 {
	//		*save = fmt.Sprintf(`@a_%s/%s/bean/%s`,
	//			p.ModuleDirName, p.ModuleName, strings.Replace(p.Value, `@ab_`, ``, -1))
	//		return
	//
	//	}
	//	if strings.Index(p.Value, `@ob_`) != -1 {
	//		*save = fmt.Sprintf(`@o_%s/%s/bean/%s`,
	//			p.ModuleDirName, p.ModuleName, strings.Replace(p.Value, `@ob_`, ``, -1))
	//		return
	//	}
	//
	//	if strings.Index(p.Value, `@ar_`) != -1 {
	//		controllerName := a.Uri[:strings.Index(a.Uri, `/`)]
	//		*save = fmt.Sprintf(`@a_%s/%s/request/%s/%s`,
	//			p.ModuleDirName, p.ModuleName, controllerName,
	//			strings.Replace(p.Value, `@ar_`, ``, -1))
	//		return
	//	}
	//
	//	if strings.Index(p.Value, `@or_`) != -1 {
	//		controllerName := a.Uri[:strings.Index(a.Uri, `/`)]
	//		*save = fmt.Sprintf(`@o_%s/%s/request/%s/%s`,
	//			p.ModuleDirName, p.ModuleName, controllerName,
	//			strings.Replace(p.Value, `@or_`, ``, -1))
	//		return
	//	}
	//
	//	if p.Value == `` {
	//		return
	//	}
	//	paramName := stringsExt.CamelNameFlag(a.Uri, `/`) + `Request`
	//	if p.Value != `true` {
	//		paramName = p.Value
	//	}
	//
	//	controllerName := a.Uri[:strings.Index(a.Uri, `/`)]
	//
	//	*save = fmt.Sprintf(`%s/%s/request/%s/%s`,
	//		p.ModuleDirName, p.ModuleName, controllerName, paramName)
	//}
}

func (p *fillParam) parserAnnotation(a *Annotation, s string) {
	a.IsParse = true

	if s[len(s)-1:] != fieldFlag {
		s += fieldFlag
	}
	log.Debug(s)

	for _, kw := range mustKws {
		if strings.Index(s, string(kw)) == -1 {
			s += fmt.Sprintf("%s%s%s", kw, kvFlag, fieldFlag)
		}
	}
	fields := strings.Split(s, fieldFlag)
	for _, field := range fields {
		kv := strings.Split(field, kvFlag)
		switch len(kv) {
		case 1:
			h := handlers[KeyWord(strings.TrimSpace(kv[0]))]
			if h != nil {
				p.Value = ``
				h(a, p)
			}
		case 2:
			h := handlers[KeyWord(strings.TrimSpace(kv[0]))]
			if h != nil {
				p.Value = strings.TrimSpace(kv[1])
				h(a, p)
			}
		}
	}
	log.Debug("redirect:", a.Redirect)
}

func (p *fillParam) toUri(name string) string {
	//return strings.Replace(stringsExt.UnderscoreName(name), `_`, `/`, -1)
	return stringsExt.Lcfirst(stringsExt.CamelName(name))
}

func (p *fillParam) checkFuncIsController(k *ast.FuncDecl, annotation *Annotation) bool {
	if k.Type.Params == nil {
		return false
	}

	getPkgStructName := func(s *ast.Object) (bool, string, string, string) {
		switch dk := s.Decl.(type) {
		case *ast.Field:
			switch dTy := dk.Type.(type) {
			case *ast.SelectorExpr:
				structName := dTy.Sel.Name
				pkgName := dTy.X.(*ast.Ident).Name
				prefix := `@rp_`
				if strings.Contains(structName, `Body`) {
					prefix = `@rb_`
				}

				return pkgName == `req`, prefix, pkgName, structName
			case *ast.StarExpr:
				switch dxt := dTy.X.(type) {
				case *ast.SelectorExpr:
					structName := dxt.Sel.Name
					pkgName := dxt.X.(*ast.Ident).Name
					prefix := `@rp_`
					if strings.Contains(structName, `Body`) {
						prefix = `@rb_`
					}
					return pkgName == `req`, prefix, pkgName, structName
				}
			case *ast.ArrayType:
				switch dxt := dTy.Elt.(type) {
				case *ast.SelectorExpr:
					structName := dxt.Sel.Name
					pkgName := dxt.X.(*ast.Ident).Name
					return pkgName == `req`, `@ra_`, pkgName, structName
				}

			default:
				return false, ``, ``, ``
			}

		}
		return false, ``, ``, ``

	}

	var (
		ok bool
	)

	if len(k.Type.Params.List) != 0 {
		for i := 0; i < len(k.Type.Params.List); i++ {
			if len(k.Type.Params.List[i].Names) == 0 {
				log.Debug("============", annotation.Uri)
				return false
			}
			_ok, prefix, pkgName, structName := getPkgStructName(k.Type.Params.List[i].Names[0].Obj)
			if _ok {
				ok = _ok
				switch prefix {
				case `@rp_`:
					annotation.RequestParamStruct = pkgName + "." + structName
					annotation.RequestParam = prefix +
						fmt.Sprintf("modules/%s/domain/dto/%s/%s/%s",
							p.ModuleName,
							pkgName,
							p.ServiceName,
							structName)
				case `@rb_`:
					annotation.RequestBodyStruct = pkgName + "." + structName
					annotation.RequestBody = prefix +
						fmt.Sprintf("modules/%s/domain/dto/%s/%s/%s",
							p.ModuleName,
							pkgName,
							p.ServiceName,
							structName)
				case `@ra_`:
					annotation.RequestBodyStruct = `[]` + pkgName + "." + structName
					annotation.RequestBody = prefix +
						fmt.Sprintf("modules/%s/domain/dto/%s/%s/%s",
							p.ModuleName,
							pkgName,
							p.ServiceName,
							structName)
				}
			}
		}
	} else {
		ok = true
	}

	if k.Type.Results != nil && len(k.Type.Results.List) != 0 && ok {

		switch dx := k.Type.Results.List[0].Type.(type) {
		case *ast.Ident:
			if dx.Name == `error` {
				annotation.ResponseType = ResponseTypeError
			} else {
				annotation.ResponseType = ResponseTypeString
				annotation.Response = `@rps_` + dx.Name
			}

		case *ast.ArrayType:
			switch dk := dx.Elt.(type) {
			case *ast.SelectorExpr:
				structName := dk.Sel.Name
				pkgName := dk.X.(*ast.Ident).Name
				annotation.Response = `@rpa_` + fmt.Sprintf("modules/%s/domain/dto/%s/%s/%s",
					p.ModuleName,
					pkgName,
					p.ServiceName,
					structName)
				annotation.ResponseType = ResponseTypeObject
			case *ast.StarExpr:
				switch x := dk.X.(type) {
				case *ast.SelectorExpr:
					structName := x.Sel.Name
					pkgName := x.X.(*ast.Ident).Name
					annotation.Response = `@rpa_` + fmt.Sprintf("modules/%s/domain/dto/%s/%s/%s",
						p.ModuleName,
						pkgName,
						p.ServiceName,
						structName)
					annotation.ResponseType = ResponseTypeObject
				}
			case *ast.Ident:
				if dk.Name == `byte` {
					annotation.ResponseType = ResponseTypeArrayByte
				} else {
					annotation.ResponseType = ResponseTypeObject
				}

			}

		case *ast.SelectorExpr:
			structName := dx.Sel.Name
			pkgName := dx.X.(*ast.Ident).Name
			annotation.Response = `@rpa_` + fmt.Sprintf("modules/%s/domain/dto/%s/%s/%s",
				p.ModuleName,
				pkgName,
				p.ServiceName,
				structName)
			annotation.ResponseType = ResponseTypeObject
		case *ast.StarExpr:
			switch dxt := dx.X.(type) {
			case *ast.SelectorExpr:
				structName := dxt.Sel.Name
				pkgName := dxt.X.(*ast.Ident).Name
				annotation.Response = `@rp_` + fmt.Sprintf("modules/%s/domain/dto/%s/%s/%s",
					p.ModuleName,
					pkgName,
					p.ServiceName,
					structName)
				annotation.ResponseType = ResponseTypeObject
			}
		default:
			annotation.ResponseType = ``
			log.Debug(k.Type.Results.List[0].Type.(*ast.Ident))
		}

	}
	return ok
}

func (p *fillParam) productAST(content string) (an []*Annotation) {
	fset := token.NewFileSet() // 位置是相对于节点
	f, err := parser.ParseFile(fset, ``, content, parser.ParseComments)
	if err != nil {
		panic(err)
	}

	buf := strings.Builder{}
	ast.Inspect(f, func(n ast.Node) bool {
		if n == nil {
			return true
		}
		switch k := n.(type) {
		case *ast.Comment:
			anLen := len(an)
			if anLen == 0 {
				return true
			}

			idx := strings.Index(k.Text, parseFlag)

			if idx != -1 {
				if !an[anLen-1].IsCommentStart {
					buf.Reset()
					an[anLen-1].IsCommentStart = true
					buf.WriteString(k.Text[idx+parseFlagLen:])
				}

				// 注解在一行情形
				if lstIdx := strings.Index(k.Text, parseEndFlag); lstIdx != -1 {
					buf.Reset()
					an[anLen-1].IsCommentStart = false
					p.parserAnnotation(an[anLen-1], strings.Replace(k.Text[idx+parseFlagLen:lstIdx], `//`, ``, -1))
					if an[anLen-1].Redirect != `` {
						a := an[anLen-1]
						newAn := Annotation{}
						di.FillValue(*a, &newAn)
						newAn.Uri = a.Redirect
						newAn.Redirect = a.Uri
						an = append(an, &newAn)
						a.Redirect = ``
					}
				}
			} else {
				if an[anLen-1].IsCommentStart {
					if idx := strings.Index(k.Text, parseEndFlag); idx != -1 {
						buf.WriteString(k.Text[:idx])
						an[anLen-1].IsCommentStart = false
						p.parserAnnotation(an[anLen-1], strings.Replace(buf.String(), `//`, ``, -1))
						if an[anLen-1].Redirect != `` {
							a := an[anLen-1]
							newAn := Annotation{}
							di.FillValue(*a, &newAn)
							newAn.Uri = a.Redirect
							newAn.Redirect = a.Uri
							an = append(an, &newAn)
							a.Redirect = ``
						}
					} else {
						buf.WriteString(k.Text)
					}
				}
				return true
			}
		case *ast.FuncDecl:
			for _, n := range []string{
				`init`,
				`GetName`,
			} {
				if k.Name.Name == n {
					return true
				}
			}
			if k.Recv != nil && len(k.Recv.List) > 0 {
				annotation := Annotation{
					Uri: fmt.Sprintf("/%s/%s/%s/%s",
						p.Version,
						p.ModuleName,
						p.ServiceName,
						p.toUri(k.Name.String())),
					ControllerName: k.Name.String(),
					RouterName:     stringsExt.CamelName(p.ServiceName + k.Name.String()),
				}
				ok := p.checkFuncIsController(k, &annotation)
				if ok {
					log.Debug(annotation.Uri)
					if annotation.Group == `` {
						p.Value = ``
						handlers[kwGroup](&annotation, p)
					}
					an = append(an, &annotation)
				}
				return true
			}
		default:
		}
		return true
	})
	return
}
