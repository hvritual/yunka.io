package eventBus

import (
	"github.com/pkg/errors"
	"sync"
	"time"
	"yunka.io/pkg/trie"
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
	node *trie.RuneTrie
	pool *sync.Pool
}

type nodeManager struct {
	indexer      uint64
	mutex        sync.Mutex
	payloadIndex *payload // 转换节点用作节点管理
}

func NewTrieEventBus() *trieEventBus {
	return &trieEventBus{
		node: trie.NewRuneTrie(),
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
	if tBus.node.Get(topic) != nil {
		return TopicExistErr
	}

	// put node manager
	tBus.node.Put(topic, &nodeManager{})
	return nil
}

func (tBus *trieEventBus) Subscribe(topic string, handle SubscribeHandle) (uint64, error) {
	err := tBus.CreateTopic(topic)
	if err != nil && err != TopicExistErr {
		return 0, err
	}

	nodeManager := tBus.node.Get(topic).(*nodeManager)
	nodeManager.mutex.Lock()
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
	nodeManager.mutex.Unlock()
	return subIndex, nil

}

func (tBus *trieEventBus) UnSubscribe(topic string, subIndex uint64) error {
	iNodeManager := tBus.node.Get(topic)
	if iNodeManager == nil {
		return TopicNotExistErr
	}
	nodeManager := iNodeManager.(*nodeManager)
	var indexCnt uint64 = 0
	if nodeManager.payloadIndex == nil {
		return nil
	}
	nodePayload := nodeManager.payloadIndex

	for {
		if indexCnt > nodeManager.indexer {
			return TopicNotMatchErr
		}
		if nodePayload == nil {
			return TopicNotMatchErr
		}
		if nodePayload.index == subIndex {
			// look at here
			nodeManager.mutex.Lock()
			// not first  subscribe
			if nodePayload.pre != nil {
				// remove node
				nodePayload.pre.next = nodePayload.next
				if nodePayload.next != nil {
					nodePayload.next.pre = nodePayload.pre
				}
			}
			// nodePayLoad is payloadIndex point ?
			if nodeManager.payloadIndex == nodePayload {
				nodeManager.payloadIndex = nil
			}
			nodeManager.mutex.Unlock()
			tBus.putPayload(nodePayload)
		}
		indexCnt++
	}
}

func (tBus *trieEventBus) Publish(topic string, param interface{}) error {
	iNodeManager := tBus.node.Get(topic)
	if iNodeManager == nil {
		return nil
	}
	nodeManager := iNodeManager.(*nodeManager)
	if nodeManager.payloadIndex == nil {
		return nil
	}
	nodePayload := nodeManager.payloadIndex
	for {
		if nodePayload == nil {
			break
		}
		nodePayload.handle(param)
		nodePayload = nodePayload.pre
	}
	return nil
}

func (tBus *trieEventBus) DestroyTopic(topic string) error {
	iNodeManager := tBus.node.Get(topic)
	if iNodeManager == nil {
		return TopicNotExistErr
	}
	nodeManager := iNodeManager.(*nodeManager)
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

func (*trieEventBus) DelayPublish(topic string, delay time.Duration, param interface{}) error {
	panic(`implement me`)
}

func (*trieEventBus) Close() error {
	return nil
}
