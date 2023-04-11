package memory

import (
	"yunka.io/framework/core"
)

/**
 * @BelongProject quanxingaopin
 * @BelongPackage memory
 * @Description:
 *
 * @Copyright 2020 5pluscloud - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/3/11 2:07 下午
 * @Version V1.0
 */

type Memory struct {
	ID string
}

func (m *Memory) GetID() string {
	return m.ID
}

func (m *Memory) SendSms(phone, smsCode string) error {
	core.Log().Debugf("memory: id:%s send sms--- phone:%s code:%s\n", m.ID, phone, smsCode)
	return nil
}
