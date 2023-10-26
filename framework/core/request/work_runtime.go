package request

import (
	"encoding/json"
	"github.com/buger/jsonparser"
	"github.com/pkg/errors"
	"github.com/valyala/fasthttp"
	"runtime/debug"
	"sync"
	"time"
	"yunka.io/framework/infras/transaction"
	"yunka.io/pkg/define"
	"yunka.io/pkg/logExt"
	"yunka.io/pkg/memstore"
	"yunka.io/pkg/response"
	"yunka.io/pkg/stringsExt"
)

/**
 * @BelongProject yunka
 * @BelongPackage application
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/9/18 4:22 下午
 * @Version V1.0
 */

const (
	DataKey = `data`
	codeKey = `code`
)

var (
	_ Runtime = (*WorkRuntime)(nil)
)

type WorkRuntime struct {
	ctx   *RequestCtx
	store *memstore.Store
	lock  sync.Locker
	// 运行handle 对应的服务module名称
	srvName      string
	IsDirect     bool
	logger       logExt.Logger
	hooks        []FinishHook
	transactions []transaction.Transaction
	statue       string
}

func (wrt *WorkRuntime) Status() string {
	return wrt.statue
}

func (wrt *WorkRuntime) TransactionPrepare(hook transaction.Transaction) {
	wrt.transactions = append(wrt.transactions, hook)
}

func (wrt *WorkRuntime) Transaction(param interface{}, f func() error) (err error) {
	for _, t := range wrt.transactions {
		if err := t.Begin(param); err != nil {
			return err
		}
	}
	defer func() {
		v := recover()
		if v != nil {
			for _, t := range wrt.transactions {
				if err := t.Rollback(); err != nil {
					wrt.Logger().Error(err)
				}
			}
			wrt.Logger().Error(string(debug.Stack()))
			err = errors.New("system error")
		}
		if err != nil {
			for _, t := range wrt.transactions {
				if err := t.Rollback(); err != nil {
					wrt.Logger().Error(err)
				}
			}
		} else {
			for _, t := range wrt.transactions {
				if err := t.Commit(); err != nil {
					wrt.Logger().Error(err)
				}
			}
		}
	}()

	return f()
}

func (wrt *WorkRuntime) BindFinishHook(hook FinishHook) {
	wrt.hooks = append(wrt.hooks, hook)
}

func (wrt *WorkRuntime) FinishRequest(err error) error {
	for _, hook := range wrt.hooks {
		if _err := hook(err); err != nil && _err != nil {
			wrt.Logger().Error(_err)
		}
	}
	wrt.hooks = nil
	wrt.transactions = nil
	return err
}

func (wrt *WorkRuntime) GetParam(param interface{}) error {
	return wrt.ctx.ShouldBindQuery(param)
}

func (wrt *WorkRuntime) GetBody(body interface{}) error {
	return wrt.ctx.ShouldBindJSON(body)
}

func (wrt *WorkRuntime) GetServiceName() string {
	return wrt.srvName
}

func (wrt *WorkRuntime) WriteBys(respBys, setBys []byte) ([]byte, error) {
	bys, err := jsonparser.Set(respBys, setBys, DataKey)
	if err != nil {
		return respBys, response.ErrSysError
	}
	return bys, nil
}

func (wrt *WorkRuntime) ResponseObject(obj interface{}) ([]byte, error) {
	bys, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	return wrt.ResponseByte(bys)
}

func (wrt *WorkRuntime) ResponseString(str string) ([]byte, error) {
	return wrt.ResponseByte(stringsExt.StringToSlice("\"" + str + "\""))
}

func (wrt *WorkRuntime) ResponseError(err error) ([]byte, error) {
	return wrt.WriteBys(response.ErrSysError, stringsExt.StringToSlice("\""+err.Error()+"\""))
}

func (wrt *WorkRuntime) Write(bys []byte) error {
	_, err := wrt.GetRequestCtx().Response.BodyWriter().Write(bys)
	v, _ := jsonparser.GetString(bys, codeKey)
	wrt.statue = v
	return err
}

func (wrt *WorkRuntime) ResponseByte(bys []byte) ([]byte, error) {
	return wrt.WriteBys(response.SysSuccess, bys)
}

func (wrt *WorkRuntime) SetSrvName(srvName string) {
	wrt.srvName = srvName
}

func (wrt *WorkRuntime) Logger() logExt.Logger {
	var traceId = wrt.GetRequestCtx().UserValue(define.TraceId)
	if traceId, ok := traceId.(string); ok {
		lg := logExt.Copy(wrt.logger)
		switch lg.(type) {
		case logExt.TraceLogger:
			lg.(logExt.TraceLogger).Set(traceId)
			return lg
		}
	}
	return wrt.logger
}

func (wrt *WorkRuntime) SetLogger(logger logExt.Logger) {
	wrt.logger = logger
}

func (wrt *WorkRuntime) Store() *memstore.Store {
	return wrt.store
}

func (wrt *WorkRuntime) CopyFrom(dst *RequestCtx) {
	wrt.lock.Lock()
	defer wrt.lock.Unlock()
	wrt.ctx.RequestCtx = &fasthttp.RequestCtx{}
	dst.Request.CopyTo(&wrt.ctx.Request)

}

func (wrt *WorkRuntime) GetRequestCtx() *RequestCtx {
	return wrt.ctx
}

func (wrt *WorkRuntime) JSONWrite(data interface{}) (int, error) {
	bys, _ := json.Marshal(data)
	return wrt.ctx.Response.BodyWriter().Write(bys)
}

func (wrt *WorkRuntime) SetRequestCtx(ctx *fasthttp.RequestCtx) {
	wrt.ctx.RequestCtx = ctx
}

// implement context.Context interface
func (wrt *WorkRuntime) Deadline() (deadline time.Time, ok bool) {
	return
}

func (wrt *WorkRuntime) Done() <-chan struct{} {
	return nil
}

func (wrt *WorkRuntime) Err() error {
	return nil
}

func (wrt *WorkRuntime) Value(key interface{}) interface{} {
	if key == 0 {
		return &(wrt.ctx.Request).Header
	}
	if keyAsString, ok := key.(string); ok {
		return stringsExt.SliceToString(wrt.ctx.Request.Header.Peek(keyAsString))
	}
	return nil
}

func (wrt *WorkRuntime) Set(key, val string) {
	(wrt.ctx.Request).Header.Set(key, val)
}

func (wrt *WorkRuntime) ForeachKey(handler func(key, val string) error) error {
	var err error
	if wrt.ctx == nil {
		return errors.New("not context request ")
	}

	wrt.ctx.Request.Header.VisitAll(func(key, value []byte) {
		if err = handler(stringsExt.SliceToString(key), stringsExt.SliceToString(value)); err != nil {
			return
		}
	})
	return err
}

func NewWorkRuntime() *WorkRuntime {
	return &WorkRuntime{
		IsDirect: true,
		store:    &memstore.Store{},
		ctx:      &RequestCtx{},
		lock:     &sync.Mutex{},
	}
}
