package selector

import (
	"context"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	"yunka.io/pkg/invoke"
)

type RPCOption func(*rpcOptions)

type rpcOptions struct {
	serviceResolver func(string) (string, error)
	bypass          func(string) bool
	selectOptions   []SelectOption
}

// WithRPCServiceResolver overrides the default /service/method parser.
func WithRPCServiceResolver(resolver func(string) (string, error)) RPCOption {
	return func(options *rpcOptions) { options.serviceResolver = resolver }
}

// WithRPCBypass leaves matching methods on the wrapped client's normal Invoke
// path. This is useful when the wrapped client multiplexes local and remote RPC.
func WithRPCBypass(bypass func(string) bool) RPCOption {
	return func(options *rpcOptions) { options.bypass = bypass }
}

func WithRPCSelectOptions(options ...SelectOption) RPCOption {
	return func(config *rpcOptions) { config.selectOptions = append(config.selectOptions, options...) }
}

type selectorRPCClient struct {
	next     invoke.RpcClient
	selector Selector
	options  rpcOptions
}

// WrapRPCClient closes the selector feedback loop without changing generated
// RPC code. The wrapped client's InvokeNode must be the final remote transport.
func WrapRPCClient(next invoke.RpcClient, selector Selector, opts ...RPCOption) invoke.RpcClient {
	if next == nil || selector == nil {
		return next
	}
	options := rpcOptions{serviceResolver: serviceFromRPCMethod}
	for _, option := range opts {
		option(&options)
	}
	return &selectorRPCClient{next: next, selector: selector, options: options}
}

func (client *selectorRPCClient) Invoke(ctx context.Context, method string, args, reply proto.Message, params ...interface{}) error {
	if client.options.bypass != nil && client.options.bypass(method) {
		return client.next.Invoke(ctx, method, args, reply, params...)
	}
	service, err := client.options.serviceResolver(method)
	if err != nil {
		return err
	}
	selection, err := Pick(client.selector, service, client.options.selectOptions...)
	if err != nil {
		return err
	}
	err = client.next.InvokeNode(ctx, selection.Node.Address, method, args, reply, params...)
	selection.Done(err)
	return err
}

func (client *selectorRPCClient) InvokeNode(ctx context.Context, nodeID, method string, args, reply proto.Message, params ...interface{}) error {
	return client.next.InvokeNode(ctx, nodeID, method, args, reply, params...)
}

// Pick uses W5 Picker when available and falls back to the legacy Select/Mark
// lifecycle for third-party Selector implementations.
func Pick(selector Selector, service string, opts ...SelectOption) (*Selection, error) {
	if picker, ok := selector.(Picker); ok {
		return picker.Pick(service, opts...)
	}
	next, err := selector.Select(service, opts...)
	if err != nil {
		return nil, err
	}
	node, err := next()
	if err != nil {
		return nil, err
	}
	selection := &Selection{Node: cloneNode(node), started: time.Now()}
	selection.finish = func(_ time.Duration, err error, _ Outcome) { selector.Mark(service, node, err) }
	return selection, nil
}

func serviceFromRPCMethod(method string) (string, error) {
	method = strings.TrimSpace(method)
	if len(method) < 3 || method[0] != '/' {
		return "", ErrInvalidMethod
	}
	rest := method[1:]
	index := strings.IndexByte(rest, '/')
	if index <= 0 {
		return "", ErrInvalidMethod
	}
	return rest[:index], nil
}
