package controller

import (
	"fmt"
	"github.com/kataras/golog"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"yunka.io/pkg/stringsExt"
)

const (
	AppName = `controller`
)

var (
	log = golog.New()
)

/**
 * @BelongProject namei
 * @BelongPackage router
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/7/22 10:44 上午
 * @Version V1.0
 */

func init() {
	log.Level = golog.DebugLevel
}

func getControllerFile(path string) []string {
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

		if strings.Contains(path, filepath.Join(`domain`, `services`)) {
			log.Debug(path)
			files = append(files, path)
			return nil
		}

		return nil
	})
	if err != nil {
		fmt.Printf("filepath.Walk() returned %v\n", err)
	}

	return files
}

func APIParse(path, project, core, pkgName, version string) {
	files := getControllerFile(path)
	idx := -1

	if len(files) != 0 {
		// +1 去除 /
		idx = strings.Index(files[0], `modules`)
	}
	for _, file := range files {
		f := file[idx:]
		infos := strings.Split(f, string(filepath.Separator))
		if len(infos) == 5 {
			moduleName := infos[1]
			serviceName := strings.Replace(infos[4], `.go`, ``, -1)

			bys, err := ioutil.ReadFile(file)
			if err != nil {
				log.Debug(err)
				break
			}

			serviceName = stringsExt.Lcfirst(stringsExt.CamelName(serviceName))

			product(moduleName, core, project, serviceName, pkgName, (&fillParam{
				Version:     version,
				ModuleName:  moduleName,
				ServiceName: serviceName,
			}).productAST(string(bys)))
		}

	}
}
