package role

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"yunka.io/framework/requestscope"
	"yunka.io/gateway/dispatcher/intercept"
	"yunka.io/gateway/dispatcher/intercept/role/db"
	"yunka.io/gateway/dispatcher/router"
	"yunka.io/gateway/internal/resp"
	"yunka.io/gateway/rpc/meta"
	"yunka.io/pkg/logExt"
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

func (r *RoleIntercept) VerifyRoleApiRight(ctx context.Context, apiUUID, orgUUID string, roleUUID []string) ([]byte, bool) {
	ok, err := requestscope.ExecuteValue(ctx, r.scopes, func(scope *requestscope.Scope[*db.Store]) (bool, error) {
		return scope.Repositories().VerifyRoleAPIRight(apiUUID, orgUUID, roleUUID)
	})
	if err != nil {
		logExt.Error(err)
		return resp.SysNotRightBys, false
	}
	if !ok {
		return resp.SysRoleNotMatchBys, false
	}
	return nil, true
}

func (r *RoleIntercept) BatchAddRuntimeApi(ctx context.Context,
	request *meta.BatchRuntimeApiRequest) (*meta.OperateRuntimeApiResponse, error) {

	btns := make([]db.ApiModuleButton, len(request.Buttons))
	for idx, button := range request.Buttons {
		btns[idx] = db.ApiModuleButton{
			ApiUUID:          button.ApiUUID,
			ModuleButtonUUID: button.ModuleBtnUUID,
		}
	}
	if err := requestscope.Execute(ctx, r.scopes, func(scope *requestscope.Scope[*db.Store]) error {
		return scope.Repositories().BatchCreate(btns)
	}); err != nil {
		return &meta.OperateRuntimeApiResponse{}, err
	}

	for idx, api := range request.Apis {
		r.apiTree.Insert(api.Uri, request.Apis[idx])
	}

	return &meta.OperateRuntimeApiResponse{}, nil
}

func (r *RoleIntercept) DeleteRuntimeApi(ctx context.Context,
	request *meta.DeleteRuntimeApiRequest) (*meta.OperateRuntimeApiResponse, error) {
	if err := requestscope.Execute(ctx, r.scopes, func(scope *requestscope.Scope[*db.Store]) error {
		return scope.Repositories().DeleteApi(request.Uuid)
	}); err != nil {
		return &meta.OperateRuntimeApiResponse{}, err
	}

	r.apiTree.Delete(request.Uri)
	return &meta.OperateRuntimeApiResponse{}, nil
}

func (r *RoleIntercept) OperateRoleAPI(ctx context.Context,
	btn *meta.RoleModuleBtn) (*meta.OperateRoleResponse, error) {
	return &meta.OperateRoleResponse{}, requestscope.Execute(ctx, r.scopes, func(scope *requestscope.Scope[*db.Store]) error {
		return scope.Repositories().OperateRole(btn.OrgUUID, btn.RoleUUID, btn.ModuleBtnUUID, btn.DeleteModuleBtnUUID)
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
