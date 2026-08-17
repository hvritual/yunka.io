package uuid

/**
 * @BelongProject yunka
 * @BelongPackage uuid
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/12/9 10:53 下午
 * @Version V1.0
 */
import "github.com/rs/xid"

func UUID() string {
	return xid.New().String()
}
