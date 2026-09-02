package api

import (
	"encoding/json"
	"fmt"
	"github.com/kataras/golog"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
	"github.com/hvritual/yunka.io/pkg/array"
	"github.com/hvritual/yunka.io/pkg/cryptoExt"
	"github.com/hvritual/yunka.io/pkg/httpExt"
)

const (
	routerItemSplit     = `;`
	routerItemInfoSplit = `:`
	packageSplit        = `/`
	ServerPosition      = 3
	ModulePosition      = 2
)

const (
	importUri             = `/v1/system/api/import`
	moduleUuidUri         = `/v1/system/module/uuid`
	moduleCreateButtonUri = `/v1/system/module/createButton`
)

/**
* @Description: TODO
* @date 2019-07-29
* @version V1.0
 */

const (
	AppName = `api`
)

func getRouterFiles(path string) []string {
	var files []string
	err := filepath.Walk(path, func(path string, f os.FileInfo, err error) error {
		if f == nil {
			return err
		}
		if f.IsDir() {
			return nil
		}
		if path[0:1] == `.` {
			return nil
		}

		if strings.Contains(path, `auto/route`) || strings.Contains(path, `auto\route`) {
			if strings.Contains(path, "_def.go") {
				files = append(files, path)
				log.Info(path)
				return nil
			}
		}

		return nil
	})
	if err != nil {
		fmt.Printf("filepath.Walk() returned %v\n", err)
	}
	log.Debug(files)
	return files
}

type DataPoint struct {
	APIItem  *APIItem
	parent   *DataPoint
	children []*DataPoint
}

func ConfigFastModule(fast bool) {
	fastModule = fast
}

type Arg struct {
	FrameName        string
	Host             string
	Path             string
	Info             bool
	Print            bool
	AutoCreateButton bool
	Force            bool
	APIKey           string
}

func Main(arg Arg) {
	if arg.Info {
		log.SetLevel(golog.InfoLevel.String())
	} else {
		log.SetLevel(golog.DebugLevel.String())
	}
	//
	var apis = api(arg.Path, arg.FrameName, ``, arg.Print)

	if len(arg.APIKey) != 32 {
		log.Fatal("api key must be supplied as a 32-byte value")
	}
	code, err := cryptoExt.BaseAesEncrypt([]byte(fmt.Sprintf(`{"authBit": 16, ts:%d}`, time.Now().Unix())),
		[]byte(arg.APIKey))
	if err != nil {
		log.Fatal(err)
	}

	if arg.AutoCreateButton {

		btnIdxMap := make(map[string]string)
		keys := []string(nil)
		for _, api := range apis {
			for _, btn := range api.ModuleBtnNumber {
				if btn == `` {
					continue
				}
				btnIdxMap[btn] = ""
				keys = append(keys, btn)
			}
		}

		keys = array.StringRemoveRepeated(keys)
		sort.Slice(keys, func(i, j int) bool {
			return keys[i] > keys[j]
		})
		bys, err := ioutil.ReadFile("./script/button.json")
		if err != nil {
			log.Fatal(err)
		}

		hasRemarkButtonIdx := make(map[string]string)
		err = json.Unmarshal(bys, &hasRemarkButtonIdx)
		if err != nil {
			log.Fatal(" Unmarshal ", err)
		}

		for _, key := range keys {
			v, ok := hasRemarkButtonIdx[key]
			if ok {
				btnIdxMap[key] = v
			}
		}

		bys, err = json.Marshal(btnIdxMap)
		if err != nil {
			log.Fatal(" Marshal ", err)
		}

		err = ioutil.WriteFile(fmt.Sprintf("./script/button_%d.json",
			time.Now().Unix()/(24*3600)), bys, 0600)
		if err != nil {
			log.Fatal(" WriteFile ", err)
		}

		moduleUuidIdx := make(map[string]string)
		for _, api := range apis {
			btns := api.ModuleBtnNumber
			for _, btn := range btns {
				if btn == `` {
					continue
				}
				moduleNameIdx := strings.LastIndex(btn, `.`)
				if moduleNameIdx == -1 {
					log.Fatal("btn:", btn, " not match")
				}
				moduleName := btn[:moduleNameIdx]
				//btn = btn[moduleNameIdx+1:]
				id, ok := moduleUuidIdx[moduleName]
				if !ok {
					_id, err := httpExt.GetID(arg.Host+moduleUuidUri,
						map[string]string{
							`routerName`: moduleName,
						}, map[string]string{
							`version`: `1.0.1`,
							`X-Code`:  code,
						})
					if err != nil {
						log.Fatal("moduleName:", moduleName, " btn:", btn, err)
					}
					if _id == `` {
						log.Fatal("not found module ", moduleName)
					}
					moduleUuidIdx[moduleName] = _id
					id = _id
				}

				chName, ok := btnIdxMap[btn]
				if !ok || chName == `` {
					log.Fatal("not found module button chines name ", btn)
				}

				bys, err = httpExt.Post(arg.Host+moduleCreateButtonUri, map[string]string{
					`version`: `1.0.1`,
					`X-Code`:  code,
				}, map[string]interface{}{
					`label`:      chName,
					`moduleUuid`: id,
					`name`:       btn,
				})
				log.Debug(string(bys))

			}
		}
	}

	bys, err := httpExt.Post(arg.Host+importUri, map[string]string{
		`version`: `1.0.1`,
		`X-Code`:  code,
	}, map[string]interface{}{
		`api`:   apis,
		`force`: arg.Force,
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Debug(string(bys))

}

func api(path, frameName, fileName string, isPrint bool) []*APIStruct {

	var apis []*APIStruct

	rs := getRouterFiles(path)
	var abs []*APIStructBuilder
	if fileName != `` {
		routerContent, err := ioutil.ReadFile(fileName)
		if err != nil {
			panic(err)
		}

		lines := strings.Split(string(routerContent), "\n")
		routerStart := false
		var apiBuilder *APIStructBuilder
		for key := range lines {

			line := strings.TrimLeftFunc(lines[key], unicode.IsSpace)
			if strings.HasPrefix(line, `const`) {
				routerStart = true
				continue
			}

			if routerStart {
				if strings.HasPrefix(line, `//`) {
					line = line[len(`//`):]
					if strings.Contains(line, frameName) && strings.Replace(line, ` `, ``, -1) == frameName {
						if apiBuilder != nil {
							log.Debug("search content:", apiBuilder.b.String())
							abs = append(abs, apiBuilder)
						}
						apiBuilder = &APIStructBuilder{b: &strings.Builder{}, api: &APIStruct{
							unionMap: make(map[string]string),
						}}
						continue
					}
					if apiBuilder != nil {
						apiBuilder.b.WriteString(line)
					}
				}
			}

		}

		if apiBuilder != nil {
			abs = append(abs, apiBuilder)
		}
	} else {
		for _, r := range rs {
			routerContent, err := ioutil.ReadFile(r)
			if err != nil {
				panic(err)
			}

			lines := strings.Split(string(routerContent), "\n")
			routerStart := false
			var apiBuilder *APIStructBuilder
			for key := range lines {
				line := strings.TrimLeftFunc(lines[key], unicode.IsSpace)
				if strings.HasPrefix(line, `const`) {
					routerStart = true
					continue
				}

				if routerStart {
					if strings.HasPrefix(line, `//`) {
						line = line[len(`//`):]
						if strings.Contains(line, frameName) && strings.Replace(line, ` `, ``, -1) == frameName {
							if apiBuilder != nil {
								log.Debug("search content:", apiBuilder.b.String())
								abs = append(abs, apiBuilder)
							}
							apiBuilder = &APIStructBuilder{b: &strings.Builder{}, api: &APIStruct{
								unionMap: make(map[string]string),
							}}
							continue
						}
						if apiBuilder != nil {
							apiBuilder.b.WriteString(line)
						}
					}
				}

			}

			if apiBuilder != nil {
				abs = append(abs, apiBuilder)
			}
		}
	}

	var (
		m     = make(map[string]*APIStruct)
		union []*APIStruct
	)
	if isPrint {

		for _, value := range abs {
			log.Debug("start parse")
			if value.api.TransferBuild(value.b.String()) {
				continue
			}
			if value.api.ServiceName == `` {
				i := strings.Split(value.api.Uri, packageSplit)
				if len(i) < ServerPosition+1 {
					log.Fatal(value.api.Uri, "not api uri")
				}
				value.api.ServiceName = i[ServerPosition]
				value.api.ModuleName = i[ModulePosition]
			}
			m[value.api.Uri] = value.api
			if value.api.isUnion {
				union = append(union, value.api)
			}
			apis = append(apis, value.api)

			ApiPrint(value.api)

			log.Debug("end parse")
		}
	} else {
		for _, value := range abs {
			if value.api.TransferBuild(value.b.String()) {
				continue
			}
			if value.api.ModuleName == `` {
				i := strings.Split(value.api.Uri, packageSplit)
				if len(i) < ServerPosition+1 {
					log.Fatal(value.api.Uri, "not api uri")
				}
				value.api.ServiceName = i[ServerPosition]
				value.api.ModuleName = i[ModulePosition]
			}
			m[value.api.Uri] = value.api
			if value.api.isUnion {
				union = append(union, value.api)
			}

			apis = append(apis, value.api)
		}
	}

	if len(union) != 0 {
		for i, u := range union {
			for key, uri := range u.unionMap {
				api, ok := m[uri]
				if ok {
					union[i].Composes = append(union[i].Composes, &APIStruct{
						Uri:         api.Uri,
						Group:       api.Group,
						Name:        api.Name,
						EngName:     key,
						ServiceName: api.ServiceName,
						ModuleName:  api.ModuleName,
					})
				}
			}
		}
	}

	if len(apis) == 0 {
		log.Fatal("not found api config")
	}
	return apis

}

func APIDoc(frameName, path string, info, print bool, fileName, module string) {
	//if info {
	//	log.SetLevel(log.LogInfo)
	//}
	//
	//writerFile(_api(path, frameName, fileName, print), module)

}

func ApiPrint(value *APIStruct) {
	log.Debug("******************************************************************************************")
	log.Debug("path:", value.Uri)
	log.Debug("description: ", value.Description, "\tname: ", value.Name,
		"\tauthBit: ", value.AuthBit, "\tgroup: ", value.Group, "\t serverName: ",
		value.ServiceName)
	log.Debug("-------------- request Param -----------------")

	//PrintAPIItem(value.Request.Param)
	fmt.Println()
	log.Debug("-------------- request body -----------------")
	//PrintAPIItem(value.Request.Body)
	fmt.Println()
	log.Debug("------------- response body -----------------")
	//PrintAPIItem(value.Response.Body)
	fmt.Println()
	log.Debug("******************************************************************************************")
}

func Print(d *DataPoint, tabCnt int) {
	if d.APIItem != nil {

		for i := 1; i < tabCnt; i++ {
			fmt.Print("\t")
		}
		fmt.Println(d.APIItem.Name, d.APIItem.Description, reqTypeMap[d.APIItem.RequestType])
	}
	if len(d.children) != 0 {
		for _, value := range d.children {
			Print(value, tabCnt+1)
		}
	}
}

func PrintAPIItem(apis []APIItem) {

	for _, body := range apis {
		PrintAPI(body, 0)
	}

}

func PrintAPI(apis APIItem, tabCnt int) {
	for i := 1; i < tabCnt; i++ {
		fmt.Print("\t")
	}
	fmt.Println(apis.Name, apis.Description, reqTypeMap[apis.RequestType])
	if len(apis.ApiItems) != 0 {
		for _, value := range apis.ApiItems {
			PrintAPI(value, tabCnt+3)
		}
	}
}
