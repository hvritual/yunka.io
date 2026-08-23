package role

import (
	"errors"
	"strings"

	"yunka.io/framework/core/modulecatalog"
	"yunka.io/gateway/dispatcher/intercept"
	"yunka.io/gateway/dispatcher/router"
)

var ErrBuildContextUnavailable = errors.New("role intercept: module build context is unavailable")

// NewRoleInterceptFromBuildContext is the C7.2.3 module-composition seam. The
// caller declares databaseName in its modulecatalog.Descriptor requirements;
// platform.Provider prepares it once for the App and the restricted BuildContext
// exposes only that declared GORM capability to this vertical slice.
func NewRoleInterceptFromBuildContext(rt *router.Tree, registrar GatewayServiceRegistrar, build modulecatalog.BuildContext, databaseName string) (intercept.Intercept, error) {
	if build == nil || build.Databases() == nil {
		return nil, ErrBuildContextUnavailable
	}
	databaseName = strings.TrimSpace(databaseName)
	if databaseName == "" {
		return nil, errors.New("role intercept: database capability name is required")
	}
	database, err := build.Databases().GORM(databaseName)
	if err != nil {
		return nil, err
	}
	return NewRoleInterceptWithDatabase(rt, registrar, database)
}
