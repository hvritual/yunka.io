package intercept

import (
	"yunka.io/gateway/rpc/meta"
)

/**
* @Description: TODO
* @date 2019-01-15
* @version V1.0
 */

type Intercept interface {
	VerifyRoleApiRight(apiUUID, orgUUID string, RoleUUID []string) ([]byte, bool)

	meta.GatewayServiceServer
}
