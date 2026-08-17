package grpc

import (
	"errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"sync"
	"sync/atomic"
	"time"
)

/**
 * @BelongProject hub-client
 * @BelongPackage grpc
 * @Description:
 *
 * @Copyright 2020 5pluscloud - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/3/20 3:16 下午
 * @Version V1.0
 */

var (
	ErrStringSplit    = errors.New("err string split")
	ErrNotFoundClient = errors.New("not found grpc conn")
	ErrConnShutdown   = errors.New("grpc conn shutdown")

	defaultClientPoolCap    = 5
	defaultDialTimeout      = 5 * time.Second
	defaultKeepAlive        = 30 * time.Second
	defaultKeepAliveTimeout = 10 * time.Second
)

type ClientOption struct {
	DialTimeout      time.Duration
	KeepAlive        time.Duration
	KeepAliveTimeout time.Duration
	ClientPoolSize   int
}

func NewDefaultClientOption() *ClientOption {
	return &ClientOption{
		DialTimeout:      defaultDialTimeout,
		KeepAlive:        defaultKeepAlive,
		KeepAliveTimeout: defaultKeepAliveTimeout,
	}
}

type ClientPool struct {
	option   *ClientOption
	capacity int64
	next     int64
	sync.Mutex
	target  string
	connect connect
	conns   []*ClientConn
}

type connect func(target string) (*grpc.ClientConn, error)

func NewClient(target string, connect connect, option *ClientOption) *ClientPool {
	if option.ClientPoolSize <= 0 {
		option.ClientPoolSize = defaultClientPoolCap
	}

	return &ClientPool{
		conns:    make([]*ClientConn, option.ClientPoolSize),
		capacity: int64(option.ClientPoolSize),
		option:   option,
		connect:  connect,
		target:   target,
	}
}

func (cc *ClientPool) init() {
	for idx, _ := range cc.conns {
		conn, _ := cc.connect(cc.target)
		cc.conns[idx] = &ClientConn{conn}
	}
}

func (cc *ClientPool) checkState(conn *grpc.ClientConn) error {
	state := conn.GetState()
	switch state {
	case connectivity.TransientFailure, connectivity.Shutdown:
		return ErrConnShutdown
	}

	return nil
}

func (cc *ClientPool) getConn() (*ClientConn, error) {
	var (
		idx  int64
		next int64

		err error
	)

	next = atomic.AddInt64(&cc.next, 1)
	idx = next % cc.capacity
	conn := cc.conns[idx]
	if conn != nil && cc.checkState(conn.ClientConn) == nil {
		return conn, nil
	}

	// gc old conn
	if conn != nil {
		conn.Close()
	}

	cc.Lock()
	defer cc.Unlock()
	// double check, already inited
	conn = cc.conns[idx]
	if conn != nil && cc.checkState(conn.ClientConn) == nil {
		return conn, nil
	}

	c, err := cc.connect(cc.target)
	if err != nil {
		return nil, err
	}
	conn = &ClientConn{c}
	cc.conns[idx] = conn
	return conn, nil
}

type ClientConn struct {
	*grpc.ClientConn
}

func (c *ClientConn) Close() error {
	return nil
}

func (c *ClientConn) GetConn() *grpc.ClientConn {
	return c.ClientConn
}
