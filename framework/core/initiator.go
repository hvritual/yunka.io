package core

import (
	"yunka.io/pkg/conf"
)

/**
 * @BelongProject yunka
 * @BelongPackage application
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/9/22 3:54 下午
 * @Version V1.0
 */

type Initiator func(app *App)
type Prepare func(cnf conf.Map)

func RegisterPrepare(p Prepare) {
	prepares = append(prepares, p)
}

func RegisterInitiator(initiator Initiator) {
	initiators = append(initiators, initiator)
}
