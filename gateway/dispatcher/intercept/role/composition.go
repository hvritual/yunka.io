package role

import (
	"errors"
	"strings"

	"gorm.io/gorm"
	"yunka.io/framework/core/modulecatalog"
	"yunka.io/gateway/authz"
	"yunka.io/gateway/dispatcher/intercept"
	"yunka.io/gateway/dispatcher/intercept/role/db"
	"yunka.io/gateway/dispatcher/router"
)

var ErrBuildContextUnavailable = errors.New("role intercept: module build context is unavailable")

// NewRoleInterceptFromBuildContext is the C7.2.3 module-composition seam. The
// caller declares databaseName in its modulecatalog.Descriptor requirements;
// platform.Provider prepares it once for the App and the restricted BuildContext
// exposes only that declared GORM capability to this vertical slice.
func NewRoleInterceptFromBuildContext(rt *router.Tree, registrar GatewayServiceRegistrar, build modulecatalog.BuildContext, databaseName string) (intercept.Intercept, error) {
	database, err := databaseFromBuildContext(build, databaseName)
	if err != nil {
		return nil, err
	}
	return NewRoleInterceptWithDatabase(rt, registrar, database)
}

// NewAuthorizerFromBuildContext composes the gateway-owned Authorizer from the
// same App-owned database capability as the role control-plane service. Callers
// do not need to know repository schema or permission matching semantics.
func NewAuthorizerFromBuildContext(build modulecatalog.BuildContext, databaseName string) (authz.Authorizer, error) {
	database, err := databaseFromBuildContext(build, databaseName)
	if err != nil {
		return nil, err
	}
	return NewAuthorizerWithDatabase(database)
}

// NewAuthorizerWithDatabase is the C8.3 composition seam. Store implements the
// gateway authz.PermissionChecker directly, so all execution boundaries share
// one typed Authorizer without depending on Button relationships.
func NewAuthorizerWithDatabase(database *gorm.DB) (authz.Authorizer, error) {
	if err := db.Migrate(database); err != nil {
		return nil, err
	}
	store, err := db.NewStoreFromDB(database)
	if err != nil {
		return nil, err
	}
	return authz.NewRBACAuthorizer(store)
}

func databaseFromBuildContext(build modulecatalog.BuildContext, databaseName string) (*gorm.DB, error) {
	if build == nil || build.Databases() == nil {
		return nil, ErrBuildContextUnavailable
	}
	databaseName = strings.TrimSpace(databaseName)
	if databaseName == "" {
		return nil, errors.New("role intercept: database capability name is required")
	}
	return build.Databases().GORM(databaseName)
}
