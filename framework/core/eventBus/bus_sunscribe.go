package eventBus

/**
 * @BelongProject yunka
 * @BelongPackage eventBus
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/9/18 3:51 下午
 * @Version V1.0
 */

type BusSubscribe interface {
	Subscribe(topic string, handle SubscribeHandle) (subIndex uint64, err error)

	UnSubscribe(topic string, subIndex uint64) error
}
