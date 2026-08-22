package selector

import (
	"strings"
	"time"
)

// Pick uses the W5 Picker when available and falls back to the legacy
// Select/Mark lifecycle for third-party Selector implementations. It selects a
// target only; grpc-go remains the sole RPC transport.
func Pick(selected Selector, service string, options ...SelectOption) (*Selection, error) {
	if selected == nil {
		return nil, ErrNoneAvailable
	}
	if picker, ok := selected.(Picker); ok {
		return picker.Pick(service, options...)
	}
	next, err := selected.Select(service, options...)
	if err != nil {
		return nil, err
	}
	node, err := next()
	if err != nil {
		return nil, err
	}
	selection := &Selection{Node: cloneNode(node), started: time.Now()}
	selection.finish = func(_ time.Duration, err error, _ Outcome) { selected.Mark(service, node, err) }
	return selection, nil
}

// ServiceFromFullMethod parses a standard gRPC full method name of the form
// /package.Service/Method.
func ServiceFromFullMethod(method string) (string, error) {
	method = strings.TrimSpace(method)
	if len(method) < 3 || method[0] != '/' {
		return "", ErrInvalidMethod
	}
	rest := method[1:]
	index := strings.IndexByte(rest, '/')
	if index <= 0 || index == len(rest)-1 {
		return "", ErrInvalidMethod
	}
	return rest[:index], nil
}
