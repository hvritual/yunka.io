package eventBus

/**
* @Description: TODO
* @date 2019-04-23
* @version V1.0
 */
type SubscribeHandle func(param interface{})

type payload struct {
	index  uint64
	pre    *payload
	next   *payload
	handle SubscribeHandle
}

type EventBus interface {
	BusSubscribe
	BusPublisher

	CreateTopic(topic string) error

	DestroyTopic(topic string) error

	Close() error
}
