package intercept

import (
	"context"

	"yunka.io/gateway/rpc/meta"
)

/**
* @Description: TODO
* @date 2019-01-15
* @version V1.0
 */

type Intercept interface {
	VerifyRoleApiRight(context.Context, string, string, []string) ([]byte, bool)

	meta.GatewayServiceServer
}
