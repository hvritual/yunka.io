package util

import "github.com/golang/protobuf/proto"

/**
 * @BelongProject sqliteRaft
 * @BelongPackage pbutil
 * @Description:
 *
 * @Copyright 2020 5pluscloud - Powered By 云咖
 * @Author: fworld
 * @Date:  2020-01-15 13:36
 * @Version V1.0
 */

func EncodeMsg(msg proto.Message) ([]byte, error) {
	return proto.Marshal(msg)
}

func DecodeMsg(bys []byte, msg proto.Message) error {
	return proto.Unmarshal(bys, msg)
}
