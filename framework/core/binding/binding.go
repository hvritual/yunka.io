package binding

import (
	"github.com/valyala/fasthttp"
)

/**
* @Description: 业务请求的绑定处理 复制gin
* @date 2019/1/11
* @version V1.0
 */
type Bind interface {
	Name() string
	Bind(*fasthttp.Request, interface{}) error
}

// 请求结构体校验器
type StructValidator interface {
	ValidateStruct(interface{}) error
	Engine() interface{}
}

var (
	JSON  = jsonBinding{}
	Query = queryBinding{}
)

//var Validator StructValidator = &defaultValidator{}
var Validator StructValidator = &translationValidator{}

func validate(obj interface{}) error {
	if Validator == nil {
		return nil
	}
	return Validator.ValidateStruct(obj)
}
