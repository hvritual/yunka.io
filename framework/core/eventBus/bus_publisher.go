package eventBus

import "time"

/**
 * @BelongProject yunka
 * @BelongPackage eventBus
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/9/18 3:52 下午
 * @Version V1.0
 */

type BusPublisher interface {
	Publish(topic string, param interface{}) error

	DelayPublish(topic string, delay time.Duration, param interface{}) error
}
