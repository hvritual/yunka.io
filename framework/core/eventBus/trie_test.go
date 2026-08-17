package eventBus

import "testing"

/**
* @Description: TODO
* @date 2019-04-23
* @version V1.0
 */

func TestNewTrieDataBus(t *testing.T) {
	tbus := NewTrieEventBus()
	params := []struct {
		index  uint64
		topic  string
		handle SubscribeHandle
	}{
		{
			0,
			`hello`,
			func(param interface{}) {
				t.Log(param)
			},
		},
		{
			0,
			`hello`,
			func(param interface{}) {
				t.Log(param)
			},
		},
		{
			0,
			`hello`,
			func(param interface{}) {
				t.Log(param)
			},
		},
		{
			0,
			`world`,
			func(param interface{}) {
				t.Log(param)
			},
		},
		{
			0,
			`world`,
			func(param interface{}) {
				t.Log(`4`, param)
			},
		},
	}

	for key, value := range params {
		params[key].index, _ = tbus.Subscribe(value.topic, params[key].handle)
	}
	t.Log(tbus.Publish(`hello`, `hello world`))
	t.Log(tbus.Publish(`world`, ` world hello`))
	tbus.UnSubscribe(`world`, params[3].index)
	t.Log(tbus.Publish(`world`, ` world hello`))

	tbus.DestroyTopic(`hello`)
	tbus.DestroyTopic(`world`)
	t.Log(tbus.Publish(`world`, `world`))

}
