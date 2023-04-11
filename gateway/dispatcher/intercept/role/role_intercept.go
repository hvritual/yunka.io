package role

import (
	"context"
	"yunka.io/gateway/dispatcher/intercept"
	"yunka.io/gateway/dispatcher/intercept/role/db"
	"yunka.io/gateway/dispatcher/router"
	"yunka.io/gateway/rpc/meta"
	"yunka.io/gateway/rpc/server"
	"yunka.io/pkg/invoke"
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
	db         *db.Store
}

func (r *RoleIntercept) VerifyRoleApiRight(apiUUID, orgUUID string, RoleUUID []string) ([]byte, bool) {
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
	err := r.db.BatchCreate(btns)
	if err != nil {
		return &meta.OperateRuntimeApiResponse{}, err
	}

	for idx, api := range request.Apis {
		r.apiTree.Insert(api.Uri, request.Apis[idx])
	}

	return &meta.OperateRuntimeApiResponse{}, nil
}

func (r *RoleIntercept) DeleteRuntimeApi(ctx context.Context,
	request *meta.DeleteRuntimeApiRequest) (*meta.OperateRuntimeApiResponse, error) {
	err := r.db.DeleteApi(request.Uuid)
	if err != nil {
		return &meta.OperateRuntimeApiResponse{}, err
	}

	r.apiTree.Delete(request.Uri)
	return &meta.OperateRuntimeApiResponse{}, nil
}

func (r *RoleIntercept) OperateRoleAPI(ctx context.Context,
	btn *meta.RoleModuleBtn) (*meta.OperateRoleResponse, error) {

	return &meta.OperateRoleResponse{}, r.db.OperateRole(btn.OrgUUID, btn.RoleUUID, btn.ModuleBtnUUID, btn.DeleteModuleBtnUUID)
}

func NewRoleIntercept(rt *router.Tree, srv invoke.RpcServer) (intercept.Intercept, error) {
	db, err := db.NewStore("build", "gateway.db")
	if err != nil {
		logExt.Error(err)
	}

	ri := &RoleIntercept{
		apiTree: rt,
		db:      db,
	}
	return ri, srv.RegisterServer(server.GatewayServiceName, (meta.GatewayServiceServer)(ri))
}
