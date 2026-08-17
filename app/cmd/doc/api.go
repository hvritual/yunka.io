package doc

import (
	"encoding/json"
	"github.com/kataras/golog"
	"github.com/pkg/errors"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"
	"yunka.io/pkg/uuid"
)

/**
* @Description: TODO
* @date 2019-07-29
* @version V1.0
 */

var (
	log = golog.New()
)

type (
	RequestType uint8
	APICommon   uint8
)

type APIItem struct {
	Description string      `json:"description"`
	Uuid        string      `json:"uuid"`
	ParentUuid  string      `json:"parentUuid"`
	IsRequired  bool        `json:"isRequired"`
	Name        string      `json:"name"`
	Explain     string      `json:"explain"`
	Type        string      `json:"type"`
	RequestType RequestType `json:"requestType"`
	Index       string      `json:"-"`
	ApiItems    []APIItem   `json:"apiItems,omitempty"`
}

type APIRequest struct {
	Param []APIItem `json:"param"`
	Body  []APIItem `json:"body"`
}

type APIResponse struct {
	Body []APIItem `json:"body"`
}

type APIStructBuilder struct {
	b   *strings.Builder
	api *APIStruct
}

const (
	APICommon_All APICommon = iota
	APICommon_Private
)

const (
	moduleBtnNumberContactFlag = `|`
)

type APIStruct struct {
	callDeep        int
	eleMap          map[string]int    `json:"-"`
	relMap          map[string]int    `json:"-"`
	importMap       map[string]string `json:"-"`
	Uri             string            `json:"uri"`
	SrvUri          string            `json:"srvUri"`
	Name            string            `json:"name"`
	EngName         string            `json:"engName"`
	Description     string            `json:"description,omitempty"`
	AuthBit         uint32            `json:"authBit"`
	Group           string            `json:"group"`
	CallType        uint32            `json:"callType"`
	ModuleName      string            `json:"moduleName"`
	ServiceName     string            `json:"serviceName"`
	Method          string            `json:"method,omitempty"`
	ModuleBtnNumber []string          `json:"moduleBtnNumber"`
	Composes        []*APIStruct      `json:"composes,omitempty"`
	isUnion         bool
	unionMap        map[string]string
	Request         APIRequest  `json:"-"`
	Response        APIResponse `json:"-"`
}

const (
	ReqErr RequestType = iota
	ReqString
	ReqNumber
	ReqBool
	ReqObject
	ReqArray
)

var (
	reqTypeMap = map[RequestType]string{
		ReqString: `String`,
		ReqNumber: `Number`,
		ReqBool:   `Boolean`,
		ReqArray:  `Array`,
		ReqObject: `Object`,
	}
)

type TagInfo struct {
	TagName string
	Locale  string
	Explain string
	Binding bool
}

var (
	errNotFound = errors.New("not found")
)

const (
	T_Uri          = `uri`
	T_UriName      = `name`
	T_NotParse     = `notParse`
	T_Redirect     = `redirect`
	T_EngName      = `engName`
	T_Module       = `module`
	T_Service      = `service`
	T_AuthBit      = `authBit`
	T_Button       = `button`
	T_Description  = `description`
	T_GroupName    = `group`
	T_Method       = `method`
	T_RequestBody  = `request_body`
	T_RequestParam = `request_param`
	T_Response     = `response`
	T_UNION        = `union`
)

const (
	OmitEmpty = `-`
)
const (
	maxCallDeep = 1
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
	authBits = []struct {
		AuthBit
		Name string
	}{{
		AuthBitAuthToken,
		"员工",
	}, {
		AuthBitAuthRole,
		"授权人员",
	}, {
		AuthBitAuthApi,
		"系统",
	}}
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
		for i := 1; i < len(authBitAuthString); i++ {
			if authBitAuthString[i] == a {
				b |= 1 << i
			}
		}
	}
	return b
}

type apiTranslateHandle func(api *APIStruct, tag, value string) bool

var (
	_map = map[string]apiTranslateHandle{
		T_Uri: func(api *APIStruct, tag, value string) bool {
			api.Uri = strings.TrimFunc(value, unicode.IsSpace)
			return false
		},
		T_UriName: func(api *APIStruct, tag, value string) bool {
			api.Name = strings.TrimFunc(value, unicode.IsSpace)
			return false
		},

		T_Redirect: func(api *APIStruct, tag, value string) bool {
			api.SrvUri = strings.TrimFunc(value, unicode.IsSpace)
			return false
		},
		T_Module: func(api *APIStruct, tag, value string) bool {
			api.ModuleName = strings.TrimFunc(value, unicode.IsSpace)
			return false
		},
		T_Service: func(api *APIStruct, tag, value string) bool {
			api.ServiceName = strings.TrimFunc(value, unicode.IsSpace)
			return false
		},
		T_EngName: func(api *APIStruct, tag, value string) bool {
			api.EngName = strings.TrimFunc(value, unicode.IsSpace)
			return false
		},
		T_AuthBit: func(api *APIStruct, tag, value string) bool {
			api.AuthBit = uint32(ParseAuthBitStr(strings.TrimFunc(value, unicode.IsSpace)))
			return false
		},
		T_Button: func(api *APIStruct, tag, value string) bool {
			api.ModuleBtnNumber = strings.Split(strings.TrimFunc(value, unicode.IsSpace),
				moduleBtnNumberContactFlag)
			return false
		},
		T_Description: func(api *APIStruct, tag, value string) bool {
			api.Description = strings.TrimFunc(value, unicode.IsSpace)
			return false
		},
		T_GroupName: func(api *APIStruct, tag, value string) bool {
			api.Group = strings.TrimFunc(value, unicode.IsSpace)
			return false
		},
		T_Method: func(api *APIStruct, tag, value string) bool {
			api.Method = strings.TrimFunc(value, unicode.IsSpace)
			return false
		},

		T_RequestBody: func(api *APIStruct, tag, value string) bool {

			if strings.TrimSpace(value) == `` {
				return false
			}
			api.Request.Body = parseCommon(api.Uri, value, `json`, `请求体`)
			return false
		},
		T_RequestParam: func(api *APIStruct, tag, value string) bool {
			if strings.TrimSpace(value) == `` {
				return false
			}

			if strings.Contains(value, `[`) {
				// json 解析
				var apiItems []APIItem
				err := json.Unmarshal([]byte(value), &apiItems)
				if err != nil {
					log.Fatal(api.Uri, err)
				}
				api.Request.Param = apiItems
			} else {
				api.Request.Param = parseFile(api.Uri, `form`, strings.TrimFunc(value, unicode.IsSpace))
			}
			return false
		},
		T_Response: func(api *APIStruct, tag, value string) bool {
			if strings.TrimSpace(value) == `` {
				return false
			}
			api.Response.Body = parseCommon(api.Uri, value, `json`, `返回值`)
			return false
		},
		T_NotParse: func(api *APIStruct, tag, value string) bool {
			return true
		},

		T_UNION: func(api *APIStruct, tag, value string) bool {
			m := make(map[string]string)
			err := json.Unmarshal([]byte(value), &m)
			if err != nil {
				log.Error(err, value)
				return true
			}

			unionAfter := strings.Replace(api.Uri, `union`, ``, 1)
			for key, value := range m {
				uri := value
				if value != `` && value[0] == '~' {
					uri = unionAfter + value[1:]
				}
				api.unionMap[key] = uri
			}
			api.isUnion = true
			return false
		},
	}
)

func parseCommon(uri, value, tag, dataName string) []APIItem {
	if strings.Contains(value, `[`) {
		// json 解析
		var apiItems []APIItem
		err := json.Unmarshal([]byte(value), &apiItems)
		if err != nil {
			log.Fatal(err)
		}
		return apiItems

	} else {
		if strings.TrimSpace(value) == `` {
			return nil
		}
		if index := strings.Index(value, `@`); index != -1 {
			if len(value) > index+2 {
				var (
					eleType RequestType
					_type   string
				)

				splitIdx := strings.Index(value[index:], `_`)

				typeIndex := value[index : index+splitIdx+1]
				switch typeIndex {
				case `@a_`:
					eleType = ReqArray<<4 | ReqObject
					_type = `Array.Object`

				case `@ra_`:
					fallthrough
				case `@rpa_`:
					eleType = ReqArray<<4 | ReqObject
					_type = `Array.Object`

				case `@s_`:
					eleType = ReqString
					_type = reqTypeMap[eleType]
				case `@o_`:
					eleType = ReqObject
					_type = reqTypeMap[eleType]
				case `@n_`:
					eleType = ReqObject
					_type = reqTypeMap[eleType]
				case `@rb_`:
					eleType = ReqObject
					_type = reqTypeMap[eleType]
				case `@rp_`:
					eleType = ReqObject
					_type = reqTypeMap[eleType]
				case `@rps_`:
					eleType = ReqString
					_type = reqTypeMap[eleType]
				}
				if eleType != ReqErr {

					if typeIndex != `@n_` {
						apis := []APIItem(nil)
						value = strings.Replace(value, typeIndex, ``, -1)
						if eleType&ReqObject != 0 {
							log.Debug("prepare parse file ", uri, value)
							apis = parseFile(uri, tag, strings.TrimFunc(value, unicode.IsSpace))
						}

						dataUuid := uuid.Next()
						item := []APIItem{
							{Name: "data", Description: dataName, Uuid: dataUuid,
								IsRequired:  true,
								RequestType: eleType, ApiItems: apis, Type: _type},
						}
						for key, value := range apis {
							if value.ParentUuid == `` {
								apis[key].ParentUuid = dataUuid
							}
						}
						return item
					}
				}

			}
		}

		exitError(value)
		return nil
	}
}

func exitError(val string) {
	log.Fatal("response body data format error, it's  \n"+
		"@a_ mean data is array \n"+
		"@s_ mean data is string \n"+
		"@o_ mean data is object \n"+
		"@n_ mean data is null \n"+
		"@rb_ mean data is request body \n"+
		"@rp_ mean data is request param \n"+
		"@rps_int mean data is string \n"+
		"@rps_string mean data is string \n"+
		"As engine parse rule, eg: response:@a_filepath/bean config, "+
		"will find filepath file and lookup bean, but value is :", val)

}

func (api *APIStruct) Transfer(tag, value string) bool {
	tag = strings.TrimFunc(tag, unicode.IsSpace)
	h := _map[tag]
	if h != nil {
		return h(api, tag, value)
	}
	return false
}

func (api *APIStruct) TransferBuild(content string) bool {
	api.eleMap = make(map[string]int)
	api.relMap = make(map[string]int)
	api.importMap = make(map[string]string)
	rtItems := strings.Split(content, routerItemSplit)

	for key := range rtItems {
		index := strings.Index(rtItems[key], routerItemInfoSplit)
		if index <= 1 {
			continue
		}
		if api.Transfer(rtItems[key][0:index], rtItems[key][index+1:]) {
			return false
		}
	}
	return false
}

func judoType(uri, tagName string, parent *APIItem, object types.Type) []APIItem {
	var items []APIItem
	switch x := object.(type) {
	case *types.Array:

	case *types.Basic:
		k := x.Kind()
		if k == types.Bool {
			parent.RequestType = ReqBool

		} else if k == types.String {
			parent.RequestType = ReqString
		} else {
			parent.RequestType = ReqNumber
		}

		parent.Type = reqTypeMap[parent.RequestType]
	case *types.Named:
		switch k := x.Underlying().(type) {
		case *types.Struct:
			items = append(items, parseParam(uri, tagName, parent, x.String(), k)...)
		case *types.Basic:
			//fmt.Println(k.String())
			kKind := k.Kind()
			if kKind == types.Bool {
				parent.RequestType = ReqBool

			} else if kKind == types.String {
				parent.RequestType = ReqString
			} else {
				parent.RequestType = ReqNumber
			}
			parent.Type = reqTypeMap[parent.RequestType]
			//fmt.Printf("basic %s kind %d info %v \n", name, k.Kind(), k.Info())
		}
	case *types.Pointer:
		items = judoType(uri, tagName, parent, x.Elem())
	case *types.Slice:
		items = judoType(uri, tagName, parent, x.Elem())
		parent.RequestType = ReqArray<<4 | parent.RequestType
		parent.Type = reqTypeMap[ReqArray] + "." + parent.Type

	case *types.Struct:
		items = append(items, parseParam(uri, tagName, parent, x.String(), x)...)
	}
	return items

}

func searchTag(content, tag string) (string, error) {
	index := strings.Index(content, tag)
	if index == -1 {
		return "", errNotFound
	}

	content = content[index+len(tag)+3:]

	return content[:strings.Index(content, "\\")], nil

}

func parseTag(content, tag string) map[string]TagInfo {

	var tagInfo = make(map[string]TagInfo)
	content = strings.ReplaceAll(strings.ReplaceAll(content, `struct{`, ``), `}`, ``)

	fieldInfos := strings.Split(content, `; `)
	for key := range fieldInfos {
		infos := strings.Split(fieldInfos[key], ` `)
		infoLen := len(infos)
		switch infoLen {
		case 0, 1:
			continue
		default:
			tagContent := strings.ReplaceAll(strings.ReplaceAll(fieldInfos[key], infos[0], ``), infos[1], ``)
			tagContentInfo, err := searchTag(tagContent, tag)
			if err != nil {
				tagContentInfo = infos[0]
			}
			if tagContentInfo == `-` {
				continue
			}

			splitIndex := strings.Index(tagContentInfo, `,`)
			if splitIndex != -1 {
				tagContentInfo = tagContentInfo[:splitIndex]
			}

			localeInfo, _ := searchTag(tagContent, `locale`)
			binding, _ := searchTag(tagContent, `binding`)
			explain, _ := searchTag(tagContent, `explain`)

			tagInfo[infos[0]] = TagInfo{
				Locale:  localeInfo,
				TagName: tagContentInfo,
				Binding: binding != ``,
				Explain: explain,
			}
		}
	}
	return tagInfo
}

const (
	maxDeep = 1
)

var (
	structMap = sync.Map{}
	cache     = make(map[string]*types.Package)
	lock      sync.Mutex
)

func parseParam(uri, tagName string, parent *APIItem, filedName string, objField *types.Struct) []APIItem {
	var params []APIItem
	//objType := obj.Type()
	parentName := ``
	if parent != nil {
		parentName = parent.Index
	}
	tagInfos := parseTag(objField.String(), tagName)

	key := uri + parentName + filedName

	v ,_:=structMap.Load(key)
	hasCnt := 0
	if v != nil {
		hasCnt = v.(int)
	}
	if hasCnt+1 > maxDeep {
		log.Error(key, uri + parentName + filedName)
		return params
	}

	structMap.Store(key, hasCnt + 1)

	//k := objField.Underlying().(*types.Struct)
	k := objField
	if parent != nil && parent.Type == `` {
		parent.Type = reqTypeMap[ReqObject]
		parent.RequestType = ReqObject
	}
	for i := 0; i < k.NumFields(); i++ {
		field := k.Field(i)
		if field.Embedded() {
			items := judoType(uri, tagName, parent, field.Type())
			if len(items) != 0 {
				params = append(params, items...)
			}

		} else {
			apiUuid := uuid.Next()
			tag, ok := tagInfos[field.Name()]
			if !ok {
				continue
			}
			if strings.Contains(parentName, tag.TagName) {
				continue
			}

			parentUuid := ``
			if parent != nil {
				parentUuid = parent.Uuid
			}
			index := tag.TagName
			if parentName != `` {
				index = parentName + `.` + index
			}
			item := APIItem{
				Uuid:        apiUuid,
				ParentUuid:  parentUuid,
				IsRequired:  tag.Binding,
				Index:       index,
				Name:        tag.TagName,
				Description: tag.Locale,
				Explain:     tag.Explain,
			}
			item.ApiItems = judoType(uri, tagName, &item, field.Type())
			params = append(params, item)
		}

	}
	return params
}

func parseFile(uri, tagName string, fileAndField string) []APIItem {

	formatErrIdx := strings.Index(fileAndField, `_`)
	if formatErrIdx < 5 {
		// 5 mean less than len(`@rps_`)
		if formatErrIdx > 0 {
			fileAndField = fileAndField[formatErrIdx+1:]
		}
	}

	index := strings.LastIndex(fileAndField, packageSplit)
	if index == -1 {
		log.Debugf("index is -1, can't find file :%s\n", fileAndField)
		return nil
	}
	// 用ParseFile把文件解析成*ast.File节点

	fs := make([]*ast.File, 0, 32)

	pkgIdx := strings.LastIndex(fileAndField[:index], packageSplit)
	if pkgIdx == -1 {
		log.Debugf("index is -1, can't find file :%s\n", fileAndField)
		return nil
	}

	pkg, ok := cache[fileAndField[:pkgIdx]]
	if !ok {
		lock.Lock()
		defer lock.Unlock()
		// 二次确认
		pkg, ok = cache[fileAndField[:pkgIdx]]
	}
	if !ok {
		fset := token.NewFileSet() // 位置是相对于节点
		filepath.Walk(fileAndField[:pkgIdx], func(path string, info os.FileInfo, err error) error {
			if info == nil || info.IsDir() {
				return nil
			}

			if strings.LastIndex(path, ".go") != len(path)-3 {
				return nil
			}

			if strings.LastIndex(path, "_test.go") != -1 {
				return nil
			}

			f, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				log.Fatal("path:" + path + "   " + err.Error())
			}
			fs = append(fs, f)
			return nil
		})

		// 使用types check
		// 构造config
		config := types.Config{
			// 加载包的方式，可以通过源码或编译好的包，其中编译好的包分为gc和gccgo,前者应该是
			Importer: importer.For("source", nil),
			// 表示允许包里面加载c库 import "c"
			FakeImportC: false,
		}

		info := &types.Info{
			// 表达式对应的类型
			Types: make(map[ast.Expr]types.TypeAndValue),
			// 被定义的标示符
			Defs: make(map[*ast.Ident]types.Object),
			// 被使用的标示符
			Uses: make(map[*ast.Ident]types.Object),
			// 隐藏节点，匿名import包，type-specific时的case对应的当前类型，声明函数的匿名参数如var func(int)
			Implicits: make(map[ast.Node]types.Object),
			// 选择器,只能针对类型/对象.字段/method的选择，package.API这种不会记录在这里
			Selections: make(map[*ast.SelectorExpr]*types.Selection),
			// scope 记录当前库scope下的所有域，*ast.File/*ast.FuncType/... 都属于scope，详情看Scopes说明
			// scope关系: 最外层Universe scope,之后Package scope，其他子scope
			Scopes: make(map[ast.Node]*types.Scope),
			// 记录所有package级的初始化值
			InitOrder: make([]*types.Initializer, 0, 0),
		}
		// 这里Check的第一个path参数是当前pkg前缀，和FileSet的文件路径是无关的
		_pkg, err := config.Check("", fset, fs, info)
		if err != nil {

			panic(errors.Wrap(err, uri+"   "+tagName))
		}
		cache[fileAndField[:pkgIdx]] = _pkg
		pkg = _pkg
	}
	// use Identical to compare type
	_type := pkg.Scope().Lookup(fileAndField[index+1:])
	if _type == nil {
		log.Fatal("can't find ", fileAndField, fileAndField[index+1:])
	}
	typeName := _type.Type().(*types.Named)
	return parseParam(uri, tagName, nil, typeName.String(), typeName.Underlying().(*types.Struct))
}
