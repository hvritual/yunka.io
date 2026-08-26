package role

import (
	"context"
	"errors"
	"sort"
	"strings"

	"gorm.io/gorm"
	"yunka.io/framework/requestscope"
	"yunka.io/gateway/dispatcher/intercept"
	"yunka.io/gateway/dispatcher/intercept/role/db"
	"yunka.io/gateway/dispatcher/router"
	"yunka.io/gateway/rpc/meta"
)

/**
 * @BelongProject yunka
 * @BelongPackage verify
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/9/18 5:15 下午
 * @Version V1.0
 */

type RoleIntercept struct {
	bucketName []byte
	apiTree    *router.Tree
	scopes     requestscope.ScopeFactory[*db.Store]
}

func (r *RoleIntercept) BatchAddRuntimeApi(ctx context.Context,
	request *meta.BatchRuntimeApiRequest) (*meta.OperateRuntimeApiResponse, error) {
	if request == nil {
		return &meta.OperateRuntimeApiResponse{}, nil
	}

	apiPermissions := make(map[string][]string, len(request.Apis))
	for _, api := range request.Apis {
		if api == nil || api.Authorization == nil {
			continue
		}
		apiPermissions[api.Uuid] = mergePermissions(apiPermissions[api.Uuid], api.Authorization.Permissions)
	}
	bindings := make([]db.ButtonPermissionBinding, 0, len(request.Buttons))
	buttonUUIDs := make([]string, 0, len(request.Buttons))
	for _, button := range request.Buttons {
		if button == nil || strings.TrimSpace(button.ModuleBtnUUID) == "" {
			continue
		}
		bindings = append(bindings, db.ButtonPermissionBinding{
			ModuleButtonUUID: button.ModuleBtnUUID,
			Permissions:      mergePermissions(apiPermissions[button.ApiUUID], button.Permissions),
		})
		buttonUUIDs = append(buttonUUIDs, button.ModuleBtnUUID)
	}
	if err := requestscope.Execute(ctx, r.scopes, func(scope *requestscope.Scope[*db.Store]) error {
		repository := scope.Repositories()
		if err := repository.BindButtonPermissions(bindings); err != nil {
			return err
		}
		return repository.BackfillLegacyRolePermissionsForButtons(buttonUUIDs)
	}); err != nil {
		return &meta.OperateRuntimeApiResponse{}, err
	}

	if r.apiTree != nil {
		for idx, api := range request.Apis {
			if api != nil {
				r.apiTree.Insert(api.Uri, request.Apis[idx])
			}
		}
	}
	return &meta.OperateRuntimeApiResponse{}, nil
}

func (r *RoleIntercept) DeleteRuntimeApi(_ context.Context,
	request *meta.DeleteRuntimeApiRequest) (*meta.OperateRuntimeApiResponse, error) {
	// API lifecycle is deliberately independent from grants. Deleting an API
	// must never revoke Role->Permission or Button->Permission relationships.
	if request != nil && r.apiTree != nil {
		r.apiTree.Delete(request.Uri)
	}
	return &meta.OperateRuntimeApiResponse{}, nil
}

func (r *RoleIntercept) OperateRoleAPI(ctx context.Context,
	request *meta.RoleModuleBtn) (*meta.OperateRoleResponse, error) {
	if request == nil {
		return &meta.OperateRoleResponse{}, nil
	}
	return &meta.OperateRoleResponse{}, requestscope.Execute(ctx, r.scopes, func(scope *requestscope.Scope[*db.Store]) error {
		repository := scope.Repositories()
		add := mergePermissions(nil, request.Permissions)
		remove := mergePermissions(nil, request.DeletePermissions)

		// Fields 3 and 4 are preserved as a wire-compatibility adapter only. They
		// resolve Button->Permission and never create Role->Button grants.
		legacyAdd, err := repository.PermissionsForButtons(request.ModuleBtnUUID)
		if err != nil {
			return err
		}
		legacyRemove, err := repository.PermissionsForButtons(request.DeleteModuleBtnUUID)
		if err != nil {
			return err
		}
		add = mergePermissions(add, legacyAdd)
		remove = mergePermissions(remove, legacyRemove)
		return repository.GrantRolePermissions(request.OrgUUID, request.RoleUUID, add, remove)
	})
}

type GatewayServiceRegistrar interface {
	RegisterGatewayService(meta.GatewayServiceServer) error
}

var (
	ErrGatewayServiceRegistrarUnavailable = errors.New("role intercept: typed GatewayService registrar is unavailable")
	ErrRoleScopeFactoryUnavailable        = errors.New("role intercept: request scope factory is unavailable")
)

func NewRoleInterceptWithScope(rt *router.Tree, registrar GatewayServiceRegistrar, scopes requestscope.ScopeFactory[*db.Store]) (intercept.Intercept, error) {
	if registrar == nil {
		return nil, ErrGatewayServiceRegistrarUnavailable
	}
	if scopes == nil {
		return nil, ErrRoleScopeFactoryUnavailable
	}
	ri := &RoleIntercept{apiTree: rt, scopes: scopes}
	return ri, registrar.RegisterGatewayService(ri)
}

// NewRoleInterceptWithDatabase is the typed C7.2.3 composition entry. The
// database is App-owned (typically supplied by platform.Provider); each call
// receives a fresh request transaction and a transaction-bound role Store.
func NewRoleInterceptWithDatabase(rt *router.Tree, registrar GatewayServiceRegistrar, database *gorm.DB) (intercept.Intercept, error) {
	if err := db.Migrate(database); err != nil {
		return nil, err
	}
	units, err := requestscope.NewGORMFactory(database, nil)
	if err != nil {
		return nil, err
	}
	scopes, err := requestscope.NewFactory(requestscope.FactoryOptions[*db.Store]{
		UnitOfWork: units,
		Repositories: requestscope.GORMRepositories(func(_ context.Context, transaction *gorm.DB) (*db.Store, error) {
			return db.NewStoreFromDB(transaction)
		}),
	})
	if err != nil {
		return nil, err
	}
	return NewRoleInterceptWithScope(rt, registrar, scopes)
}

// NewRoleIntercept preserves the legacy filesystem constructor while routing
// execution through requestscope. New composition code should use
// NewRoleInterceptWithDatabase and obtain the GORM database from platform.Provider.
func NewRoleIntercept(rt *router.Tree, registrar GatewayServiceRegistrar) (intercept.Intercept, error) {
	store, err := db.NewStore("build", "gateway.db")
	if err != nil {
		return nil, err
	}
	return NewRoleInterceptWithDatabase(rt, registrar, store.DB)
}

func mergePermissions(left, right []string) []string {
	seen := make(map[string]struct{}, len(left)+len(right))
	result := make([]string, 0, len(left)+len(right))
	for _, values := range [][]string{left, right} {
		for _, permission := range values {
			permission = strings.TrimSpace(permission)
			if permission == "" {
				continue
			}
			if _, exists := seen[permission]; exists {
				continue
			}
			seen[permission] = struct{}{}
			result = append(result, permission)
		}
	}
	sort.Strings(result)
	return result
}
