package invoke

import (
	"context"
	"github.com/golang/protobuf/proto"
	"time"
)

/**
* @Description: TODO
* @date 2019-04-23
* @version V1.0
 */

const (
	RpcTimeOut = 500 * time.Millisecond
)

type SrvHandler func(ctx context.Context, args proto.Message) (reply proto.Message, err error)



type RpcClient interface {
	Invoke(ctx context.Context, method string, args, reply proto.Message, param ...interface{}) error

	InvokeNode(ctx context.Context, nodeId, method string, args, reply proto.Message, param ...interface{}) error
}

type RpcServer interface {
	RegisterServer(name string, srv interface{}) error
}


type Rpc interface {
	RpcClient
	RpcServer
}