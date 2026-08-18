package middleware

import (
	"context"

	"github.com/golang/protobuf/proto"
	"yunka.io/framework/core/runtimecontext"
	"yunka.io/pkg/invoke"
)

type rpcClient struct {
	next  invoke.RpcClient
	chain Chain
}

// WrapRPCClient applies the same transport-neutral middleware chain around an
// existing RPC client without changing generated RPC code.
func WrapRPCClient(next invoke.RpcClient, middlewares ...Middleware) invoke.RpcClient {
	if next == nil {
		return nil
	}
	return &rpcClient{next: next, chain: New(middlewares...)}
}

func (client *rpcClient) Invoke(ctx context.Context, method string, args, reply proto.Message, param ...interface{}) error {
	return client.invoke(ctx, method, func(child context.Context) error {
		return client.next.Invoke(child, method, args, reply, param...)
	})
}

func (client *rpcClient) InvokeNode(ctx context.Context, nodeID, method string, args, reply proto.Message, param ...interface{}) error {
	return client.invoke(ctx, method, func(child context.Context) error {
		return client.next.InvokeNode(child, nodeID, method, args, reply, param...)
	})
}

func (client *rpcClient) invoke(ctx context.Context, method string, final Handler) error {
	if ctx == nil {
		ctx = context.Background()
	}
	metadata, _ := runtimecontext.MetadataFrom(ctx)
	metadata.Transport = "rpc"
	metadata.Protocol = "rpc"
	metadata.Operation = method
	metadata.Method = method
	if metadata.Attributes == nil {
		metadata.Attributes = make(map[string]string)
	}
	metadata.Attributes["rpc.direction"] = "client"
	ctx = runtimecontext.WithMetadata(ctx, metadata)
	return client.chain.Handle(ctx, final)
}
