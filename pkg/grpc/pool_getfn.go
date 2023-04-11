package grpc

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"sync"
	"time"
)

/**
 * @BelongProject hub-client
 * @BelongPackage grpc
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/4/10 4:30 下午
 * @Version V1.0
 */

type ClientConnLoader interface {
	GetConn() *grpc.ClientConn
	Close() error
}

type GetConn interface {
	Get(nodeID string, ctx context.Context) (ClientConnLoader, error)
}

type poolFn struct {
	lock   sync.Locker
	pools  map[string]*ClientPool
	option *ClientOption
}

func NewPoolFn(lock sync.Locker, init, capacity int, idleTimeout time.Duration,
	maxLifeDuration time.Duration) *poolFn {
	opts := NewDefaultClientOption()
	opts.KeepAlive = maxLifeDuration
	opts.ClientPoolSize = capacity
	return &poolFn{
		lock:   lock,
		pools:  make(map[string]*ClientPool),
		option: opts,
	}
}

func (pf *poolFn) factory(target string) (*grpc.ClientConn, error) {
	return grpc.Dial(target,
		grpc.WithInsecure(),
		grpc.WithBlock(),
		grpc.WithTimeout(3*time.Second),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:    pf.option.KeepAlive,
			Timeout: pf.option.KeepAliveTimeout},
		),
	)
}

func (pf *poolFn) Get(nodeID string, ctx context.Context) (ClientConnLoader, error) {
	p := pf.pools[nodeID]
	if p == nil {
		pf.lock.Lock()
		p = pf.pools[nodeID]
		if p == nil {
			p = NewClient(nodeID, pf.factory, pf.option)
			pf.pools[nodeID] = p
		}
		pf.lock.Unlock()
	}
	cc, err := p.getConn()
	if err != nil {
		return nil, err
	}

	return cc, nil
}
