package po

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/build"
	"go/format"
	"go/parser"
	"go/token"
	"io/ioutil"
	"strings"
	"text/template"
	"yunka.io/pkg/stringsExt"
)

const (
	AppName = `po`
)

var (
	pkgInfo *build.Package
)

func Main(typeNames string) error {

	fields, pkgName, err := getConsts(typeNames)
	if err != nil {
		return err
	}

	return parseField(typeNames, pkgName, fields)

}

func parseField(typeNames, pkgName string, fields map[string]string) error {

	const strTmp = `
	package {{.pkg}}
	
	`

	fnTmp := `
	func (obj *%s) Set%s(%s %s) {
		obj.%s = %s
		obj.ConfField("%s", %s)
	}
`
	data := map[string]interface{}{
		"pkg":        pkgName,
		"structName": typeNames,
	}
	//利用模板库，生成代码文件
	t, err := template.New("").Parse(strTmp)
	if err != nil {
		fmt.Println("Parse err")
	}
	buff := bytes.NewBufferString("")
	err = t.Execute(buff, data)
	if err != nil {
		fmt.Println("Parse Execute err")
		return err
	}

	for name, typ := range fields {
		orgName := name
		if strings.Contains(name, `UUID`) {
			name = strings.Replace(name, `UUID`, `Uuid`, -1)
		} else {
			if strings.Contains(name, `ID`) {
				name = strings.Replace(name, `ID`, `Id`, -1)
			}

		}
		fieldName := name
		if name != `Type` {
			fieldName = stringsExt.Lcfirst(stringsExt.CamelName(name))
		} else {
			name = `Type`
			fieldName = `Type`
		}

		buff.WriteString(fmt.Sprintf(fnTmp, typeNames, orgName, fieldName, typ, orgName, fieldName,
			stringsExt.UnderscoreName(name), fieldName))
	}

	//格式化
	src, err := format.Source(buff.Bytes())
	if err != nil {

		fmt.Println("Source error", buff.String())
		return err
	}

	return ioutil.WriteFile(stringsExt.UnderscoreName(typeNames)+"_gen.go", src, 0644)
}
func getConsts(typeNames string) (map[string]string, string, error) {
	//解析当前目录下包信息

	fileName := stringsExt.UnderscoreName(typeNames)
	var err error
	pkgInfo, err = build.ImportDir(".", 0)
	if err != nil {
		return nil, ``, err
	}
	fset := token.NewFileSet()
	fieldMap := make(map[string]string)
	for _, file := range pkgInfo.GoFiles {
		if strings.Replace(file, ".go", ``, -1) != fileName {
			continue
		}
		f, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			return nil, ``, err
		}
		tmplFieldMap := make(map[string]string)
		ast.Inspect(f, func(n ast.Node) bool {
			decl, ok := n.(*ast.StructType)
			if !ok {
				return true
			}

			isPo := false

			if decl.Fields != nil {
				for _, field := range decl.Fields.List {
					t, ok := field.Type.(*ast.Ident)
					if ok {
						if len(field.Names) > 0 {
							name := field.Names[0].Name
							tmplFieldMap[name] = t.Name
						}
					} else {
						if strings.Index(fmt.Sprintf("%v", field.Type), "PersistObject") != 0 {
							isPo = true
						}
					}
				}
			}

			if isPo {
				for k, v := range tmplFieldMap {
					fieldMap[k] = v
				}
			}

			return true
		})
	}
	return fieldMap, pkgInfo.Name, nil
}
