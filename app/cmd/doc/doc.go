package doc

import (
	"fmt"
	"os"
	"strings"
)

/**
 * @BelongProject yunka
 * @BelongPackage api
 * @Description:
 *
 * @Copyright 2021 - Powered By 云咖
 * @Author: fworld
 * @Date:  2021/3/22 上午9:53
 * @Version V1.0
 */

const (
	docPackeage = `package doc

/**
 *  auto generate by yunka
 */
`
	docNotTokenHeader = `
/**
* @api {%s} %s %s
* @apiDescription %s
* @apiGroup %s
* @apiVersion %s
`
	docBody = `*
* @apiSuccess {String} code    状态码，0：请求成功
* @apiSuccess {String} message   提示信息
`

	docToken = `
* @apiParam {String} token  请求授权
`
	docMustParam = `* @apiParam {%s} %s  %s %s
`

	docCommonParam = `* @apiParam {%s} [%s]  %s %s
`

	docMustResponseBody = `* @apiSuccess {%s} %s  %s %s
`

	docTailTmpl = `*
* @apiSuccessExample {json} 正常返回:
* {"code":"0","msg":"","data":[]}
*
* @apiErrorExample {json} 错误返回:
* {"code":"5001","msg":"接口异常"}
*
*/

`
	split = `/`
)

func makeDir(modulePath, moduleName, apiDocVersion string) error {
	return os.MkdirAll(fmt.Sprintf("%s/%s/auto/doc/%s", modulePath, moduleName, apiDocVersion), 0750)
}

func makeCtlDoc(modulePath, moduleName, ctlName, apiDocVersion string) (*os.File, error) {
	return os.OpenFile(fmt.Sprintf("%s/%s/auto/doc/%s/%s.go", modulePath, moduleName, apiDocVersion, ctlName),
		os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0640)
}

func writerResponse(typeStr, parentName string, f *os.File, items []APIItem) {
	for _, value := range items {
		f.WriteString(fmt.Sprintf(docMustResponseBody, typeStr+value.Type, parentName+value.Name, value.Description, value.Explain))
		if len(value.ApiItems) != 0 {
			writerResponse(value.Type+`.`, parentName+value.Name+`.`, f, value.ApiItems)
		}
	}
}

func writerRequestBody(typeStr, parentName string, f *os.File, items []APIItem) {
	for _, value := range items {
		if value.IsRequired {
			f.WriteString(fmt.Sprintf(docMustParam, typeStr+value.Type, parentName+value.Name, value.Description, value.Explain))
		} else {
			f.WriteString(fmt.Sprintf(docCommonParam, typeStr+value.Type, parentName+value.Name, value.Description, value.Explain))
		}
		if len(value.ApiItems) != 0 {
			writerRequestBody(value.Type+`.`, parentName+value.Name+`.`, f, value.ApiItems)
		}
	}
}

func productDescription(api *APIStruct) string {
	results := []string{}
	results = append(results, checkApiRight(api))
	results = append(results, checkButton(api))
	results = append(results, "接口描述: "+api.Description)
	return strings.Join(results, `<br/>`)
}

func checkButton(apiStruct *APIStruct) string {
	btns := make([]string, len(apiStruct.ModuleBtnNumber))
	for idx, btn := range apiStruct.ModuleBtnNumber {
		btns[idx] = fmt.Sprintf("【%s】", btn)
	}

	return "权限按钮:" + strings.Join(btns, `、`)
}

func checkApiRight(apiStruct *APIStruct) string {
	vals := make([]string, 0, len(authBits))
	for _, bit := range authBits {
		if apiStruct.AuthBit&uint32(bit.AuthBit) != 0 {
			vals = append(vals, fmt.Sprintf("【%s】", bit.Name))
		}
	}
	return "使用对象:" + strings.Join(vals, `、`)
}

func writerFile(apis []*APIStruct, apiDocVersion string, modulePath string) {
	if modulePath == `` {
		modulePath = `modules`
	}
	var fileMap = make(map[string]*os.File)
	for _, value := range apis {
		infos := strings.Split(value.Uri, split)
		if len(infos) < 3 {
			continue
		}

		modName := infos[2]
		srvName := infos[3]
		f, ok := fileMap[modName+srvName]
		if !ok {
			makeDir(modulePath, value.ModuleName, apiDocVersion)
			f, _ = makeCtlDoc(modulePath, modName, srvName, apiDocVersion)
			f.WriteString(docPackeage)
			fileMap[modName+srvName] = f
		}

		f.WriteString(fmt.Sprintf(docNotTokenHeader, value.Method, value.Uri, value.Name,
			productDescription(value), value.Group, apiDocVersion))
		if value.AuthBit > 0 {
			f.WriteString(docToken)
		}

		for _, value := range value.Request.Param {
			if value.IsRequired {
				f.WriteString(fmt.Sprintf(docMustParam, value.Type, value.Name, value.Description, value.Explain))
			} else {
				f.WriteString(fmt.Sprintf(docCommonParam, value.Type, value.Name, value.Description, value.Explain))
			}
		}

		writerRequestBody(``, ``, f, value.Request.Body)

		f.WriteString(docBody)
		writerResponse(``, ``, f, value.Response.Body)

		f.WriteString(docTailTmpl)
	}
	for _, value := range fileMap {
		value.Close()
	}
}
