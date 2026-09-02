package intercept

import "github.com/hvritual/yunka.io/gateway/rpc/meta"

// Intercept is the typed Gateway control-plane service. Authorization is not a
// business-service method: every execution boundary delegates to authz.Authorizer.
type Intercept interface {
	meta.GatewayServiceServer
}
