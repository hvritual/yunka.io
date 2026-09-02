package router

import (
	"testing"
	"github.com/hvritual/yunka.io/gateway/rpc/meta"
)

/**
 * @BelongProject yunka
 * @BelongPackage router
 * @Description:
 *
 * @Copyright 2021 - Powered By 云咖
 * @Author: fworld
 * @Date:  2021/5/1 下午9:54
 * @Version V1.0
 */

func TestTree_Get(t *testing.T) {
	tree := New()

	tree.Insert(`/v1/mkt/wx/devicePayCb/*`, &meta.RuntimeApi{
		Uri: `/v1/mkt/wx/devicePayCb/*`,
	})
	tree.Insert(`/v1/mkt/wx/device/*`, &meta.RuntimeApi{
		Uri: `/v1/mkt/wx/device/*`,
	})

	h, ok := tree.Get(`/v1/mkt/wx/devicePayCb/11111`)
	t.Log(ok)
	if h != nil {
		t.Log(h.Uri)
	}

}
