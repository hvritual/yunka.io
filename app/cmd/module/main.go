package module

import (
	"fmt"
	"github.com/kataras/golog"
	"io/ioutil"
	"os"
	"path/filepath"
	"yunka.io/pkg/fileExt"
	"yunka.io/pkg/stringsExt"
)

const (
	AppName = `module`
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

func Generate(moduleName string) error {

	if fileExt.Exists(filepath.Join("modules", moduleName, `main.go`)) {
		return nil
	}

	paths := []string{
		`adapter`,
		`adapter/repository`,
		`auto/controller`,
		`auto/route`,
		`auto/doc`,
		`conf`,
		`domain/aggregate`,
		`domain/rpc`,
		`domain/dependency`,
		`domain/dto/code`,
		`domain/dto/req`,
		`domain/dto/resp`,
		`domain/entity`,
		`domain/po`,
		`domain/services`,
	}

	for _, path := range paths {
		os.MkdirAll(filepath.Join(`modules`, moduleName, path), 0777)
	}
	file, err := os.Create(filepath.Join(`modules`, moduleName, `domain/services/main.go`))
	if err != nil {
		return err
	}
	file.Close()

	err = generateMain(moduleName)
	if err != nil {
		return err
	}

	return generateMainService(moduleName)
}

func generateMainService(moduleName string) error {
	camelModuleName := stringsExt.Lcfirst(stringsExt.CamelName(moduleName))
	stm := `package services

import (
	_ "yunka/modules/%s/adapter/repository"
)
`
	err := ioutil.WriteFile(filepath.Join(`modules`, moduleName, `domain/services/main.go`),
		stringsExt.StringToSlice(fmt.Sprintf(stm, camelModuleName)),
		0777)
	return err
}

func generateMain(moduleName string) error {

	camelModuleName := stringsExt.CamelName(moduleName)
	confTmpl := `package conf

// module conf
type %sConf struct {
	
}`

	err := ioutil.WriteFile(filepath.Join(`modules`, moduleName, `conf/conf.go`),
		stringsExt.StringToSlice(fmt.Sprintf(confTmpl, camelModuleName)),
		0777)
	if err != nil {
		return err
	}

	tmpl := `package %s

import (
	"yunka.io/framework/core"
	"yunka.io/framework/core/module"
	"yunka/modules/%s/conf"
)


const (
	ModuleName = "%s"
)

var (
	mod core.Module
)

func init() {
	core.RegisterConfType(ModuleName, conf.%sConf{})
	mod = module.NewModule(ModuleName, func(mod core.Module) {
		
	})
}

func Init(fn core.ModuleInit) {
	mod.Init(fn)
}
`

	return ioutil.WriteFile(filepath.Join(`modules`, moduleName, `main.go`),
		stringsExt.StringToSlice(fmt.Sprintf(tmpl, moduleName, moduleName, moduleName, camelModuleName)),
		0777)
}
