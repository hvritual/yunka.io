package doc

import (
	"fmt"
	"github.com/kataras/golog"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"
)

const (
	routerItemSplit     = `;`
	routerItemInfoSplit = `:`
	packageSplit        = `/`
	ServerPosition      = 3
	ModulePosition      = 2
)

/**
* @Description: TODO
* @date 2019-07-29
* @version V1.0
 */

const (
	AppName = `doc`
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
	return files
}

type DataPoint struct {
	APIItem  *APIItem
	parent   *DataPoint
	children []*DataPoint
}

func Main(frameName, modulesDir, path, apiDocVersion string, info bool) {
	if info {
		log.SetLevel(golog.InfoLevel.String())
	} else {
		log.SetLevel(golog.DebugLevel.String())
	}
	//
	apis := api(path, frameName)
	//for _, api := range apis {
	//	ApiPrint(api)
	//}

	writerFile(apis, apiDocVersion, modulesDir)

}

func api(path, frameName string) []*APIStruct {

	var apis []*APIStruct

	rs := getRouterFiles(path)
	var abs []*APIStructBuilder

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

	var (
		m     = make(map[string]*APIStruct)
		union []*APIStruct
	)
	wg := sync.WaitGroup{}
	lock  := sync.Mutex{}
	for _, v := range abs {
		wg.Add(1)
		go func(value *APIStructBuilder) {
			defer wg.Done()
			if value.api.TransferBuild(value.b.String()) {
				return
			}
			if value.api.ModuleName == `` {
				uri := value.api.Uri
				uriIndex := strings.Index(uri, `v1/`)
				uri = uri[len(`v1/`) + uriIndex:]

				i := strings.Split(value.api.Uri, packageSplit)
				if len(i) < ServerPosition+1 {
					log.Fatal(value.api.Uri, "not api uri")
				}
				value.api.ServiceName = i[ServerPosition]
				value.api.ModuleName = i[ModulePosition]
			}
			lock.Lock()
			m[value.api.Uri] = value.api
			if value.api.isUnion {
				union = append(union, value.api)
			}

			apis = append(apis, value.api)
			lock.Unlock()
		}(v)
	}

	wg.Wait()

	if len(union) != 0 {
		for i, u := range union {
			compose := false
			if len(union[i].Response.Body) == 0 {
				union[i].Response.Body = append(union[i].Response.Body, APIItem{
					Name:        "data",
					Description: "聚合数据返回",
					RequestType: ReqObject,
					ApiItems:    nil,
					Type:        reqTypeMap[ReqObject]})
				compose = true
			}
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

					if compose {
						apiBody := api.Response.Body
						for _, body := range apiBody {
							if body.Name == "data" {
								union[i].Response.Body[0].ApiItems = append(union[i].Response.Body[0].ApiItems,
									APIItem{
										Name:     key,
										Type:     body.Type,
										Explain:  body.Explain,
										ApiItems: body.ApiItems,
									})
							}

						}
					}
				}
			}
		}
	}

	if len(apis) == 0 {
		log.Fatal("not found api config")
	}
	return apis

}

func ApiPrint(value *APIStruct) {
	log.Debug("******************************************************************************************")
	log.Debug("path:", value.Uri)
	log.Debug("description: ", value.Description, "\tname: ", value.Name,
		"\tauthBit: ", value.AuthBit, "\tgroup: ", value.Group, "\t serverName: ",
		value.ServiceName)
	log.Debug("-------------- request Param -----------------")

	PrintAPIItem(value.Request.Param)
	fmt.Println()
	log.Debug("-------------- request body -----------------")
	PrintAPIItem(value.Request.Body)
	fmt.Println()
	log.Debug("------------- response body -----------------")
	PrintAPIItem(value.Response.Body)
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
