package proxy

/**
 * @BelongProject yunka
 * @BelongPackage proxy
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/9/18 1:44 下午
 * @Version V1.0
 */

func (p *Proxy) Use(h MiddleWare) MiddleWare {
	return p.middles.Use(h)
}
