package request

import (
	"context"
	"yunka.io/framework/infras/transaction"
	"yunka.io/pkg/logExt"
	"yunka.io/pkg/memstore"
)

/**
* @Description: TODO
* @date 2019/1/11
* @version V1.0
 */

const (
	StoreKey = `store`
)

type FinishHook func(err error) error

type (
	Runtime interface {
		GetRequestCtx() *RequestCtx

		Logger() logExt.Logger

		SetLogger(logExt.Logger)

		Store() *memstore.Store

		GetParam(param interface{}) error

		GetBody(body interface{}) error

		Write(bys []byte) error

		ResponseObject(obj interface{}) ([]byte, error)

		ResponseString(str string) ([]byte, error)

		ResponseByte(bys []byte) ([]byte, error)

		ResponseError(err error) ([]byte, error)

		GetServiceName() string

		context.Context

		TransactionPrepare(hook transaction.Transaction)

		Transaction(interface{}, func() error) error

		BindFinishHook(hook FinishHook)

		Status() string
	}

	BaseRuntime interface {
		SetRuntime(Runtime)

		GetRuntime() Runtime
	}
)
