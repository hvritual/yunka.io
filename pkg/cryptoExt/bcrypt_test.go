package cryptoExt

import (
	"testing"
)

/**
 * @BelongProject fpluscloud
 * @BelongPackage cryptoExt
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/9/8 4:36 下午
 * @Version V1.0
 */

func TestModelBcrypt(t *testing.T) {
	bys, _ := ProduceBcrypt(MD5(`130001`))
	//bys,_ := ModelBcrypt(`f60db355eb18d5e15e899b153e7763a1`)
	t.Log(string(bys))
}
