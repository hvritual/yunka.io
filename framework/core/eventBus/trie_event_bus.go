package eventBus

import (
	"context"
	"github.com/pkg/errors"
	"sync"
	"time"
	"github.com/hvritual/yunka.io/pkg/trie"
)

/**
* @Description: 前缀树数据总线
* @date 2019-04-23
* @version V1.0
 */

var (
	TopicExistErr    = errors.New(`topic is exist`)
	TopicNotExistErr = errors.New(`topic not exist`)
	TopicNotMatchErr = errors.New(`topic not match`)
)

type trieEventBus struct {
	node   *trie.RuneTrie
	pool   *sync.Pool
	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	closed bool
}

type nodeManager struct {
	indexer      uint64
	mutex        sync.Mutex
	payloadIndex *payload // 转换节点用作节点管理
}

func NewTrieEventBus() *trieEventBus {
	ctx, cancel := context.WithCancel(context.Background())
	return &trieEventBus{
		node:   trie.NewRuneTrie(),
		ctx:    ctx,
		cancel: cancel,
		pool: &sync.Pool{
			New: func() interface{} {
				return &payload{}
			},
		},
	}
}

func (tBus *trieEventBus) loadPayload() *payload {
	payload := tBus.pool.Get().(*payload)
	payload.handle = nil
	payload.next = nil
	payload.pre = nil
	return payload
}

func (tBus *trieEventBus) putPayload(p *payload) {
	tBus.pool.Put(p)
	return
}

func (tBus *trieEventBus) CreateTopic(topic string) error {
	tBus.mu.Lock()
	defer tBus.mu.Unlock()
	if tBus.closed {
		return errors.New("event bus is closed")
	}
	return tBus.createTopic(topic)
}

func (tBus *trieEventBus) createTopic(topic string) error {
	if tBus.node.Get(topic) != nil {
		return TopicExistErr
	}

	// put node manager
	tBus.node.Put(topic, &nodeManager{})
	return nil
}

func (tBus *trieEventBus) Subscribe(topic string, handle SubscribeHandle) (uint64, error) {
	if handle == nil {
		return 0, errors.New("subscribe handle is nil")
	}
	tBus.mu.Lock()
	defer tBus.mu.Unlock()
	if tBus.closed {
		return 0, errors.New("event bus is closed")
	}
	err := tBus.createTopic(topic)
	if err != nil && err != TopicExistErr {
		return 0, err
	}

	nodeManager := tBus.node.Get(topic).(*nodeManager)
	nodeManager.mutex.Lock()
	defer nodeManager.mutex.Unlock()
	nodeManager.indexer++
	subIndex := nodeManager.indexer
	payload := tBus.loadPayload()
	payload.handle = handle
	payload.index = subIndex
	if nodeManager.payloadIndex == nil {
		nodeManager.payloadIndex = payload
	} else {
		nodeManager.payloadIndex.next = payload
		payload.pre = nodeManager.payloadIndex
		nodeManager.payloadIndex = payload
	}
	return subIndex, nil

}

func (tBus *trieEventBus) UnSubscribe(topic string, subIndex uint64) error {
	tBus.mu.RLock()
	defer tBus.mu.RUnlock()
	iNodeManager := tBus.node.Get(topic)
	if iNodeManager == nil {
		return TopicNotExistErr
	}
	nodeManager := iNodeManager.(*nodeManager)
	nodeManager.mutex.Lock()
	defer nodeManager.mutex.Unlock()
	if nodeManager.payloadIndex == nil {
		return nil
	}
	nodePayload := nodeManager.payloadIndex

	for nodePayload != nil {
		if nodePayload.index == subIndex {
			if nodePayload.pre != nil {
				nodePayload.pre.next = nodePayload.next
			}
			if nodePayload.next != nil {
				nodePayload.next.pre = nodePayload.pre
			}
			if nodeManager.payloadIndex == nodePayload {
				nodeManager.payloadIndex = nodePayload.pre
			}
			tBus.putPayload(nodePayload)
			return nil
		}
		nodePayload = nodePayload.pre
	}
	return TopicNotMatchErr
}

func (tBus *trieEventBus) Publish(topic string, param interface{}) error {
	tBus.mu.RLock()
	if tBus.closed {
		tBus.mu.RUnlock()
		return errors.New("event bus is closed")
	}
	iNodeManager := tBus.node.Get(topic)
	if iNodeManager == nil {
		tBus.mu.RUnlock()
		return nil
	}
	nodeManager := iNodeManager.(*nodeManager)
	nodeManager.mutex.Lock()
	handles := make([]SubscribeHandle, 0)
	nodePayload := nodeManager.payloadIndex
	for nodePayload != nil {
		handles = append(handles, nodePayload.handle)
		nodePayload = nodePayload.pre
	}
	nodeManager.mutex.Unlock()
	tBus.mu.RUnlock()
	for _, handle := range handles {
		handle(param)
	}
	return nil
}

func (tBus *trieEventBus) DestroyTopic(topic string) error {
	tBus.mu.Lock()
	defer tBus.mu.Unlock()
	iNodeManager := tBus.node.Get(topic)
	if iNodeManager == nil {
		return TopicNotExistErr
	}
	nodeManager := iNodeManager.(*nodeManager)
	nodeManager.mutex.Lock()
	defer nodeManager.mutex.Unlock()
	if nodeManager.payloadIndex != nil {
		nodePayload := nodeManager.payloadIndex
		for {
			if nodePayload == nil {
				break
			}
			tBus.putPayload(nodePayload)
			nodePayload = nodePayload.pre
		}
	}

	tBus.node.Delete(topic)
	return nil
}

func (tBus *trieEventBus) DelayPublish(topic string, delay time.Duration, param interface{}) error {
	tBus.mu.RLock()
	if tBus.closed {
		tBus.mu.RUnlock()
		return errors.New("event bus is closed")
	}
	tBus.wg.Add(1)
	ctx := tBus.ctx
	tBus.mu.RUnlock()
	go func() {
		defer tBus.wg.Done()
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-timer.C:
			_ = tBus.Publish(topic, param)
		case <-ctx.Done():
		}
	}()
	return nil
}

func (tBus *trieEventBus) Close() error {
	tBus.mu.Lock()
	if tBus.closed {
		tBus.mu.Unlock()
		return nil
	}
	tBus.closed = true
	tBus.cancel()
	tBus.mu.Unlock()
	tBus.wg.Wait()
	return nil
}
