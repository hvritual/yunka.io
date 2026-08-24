package memory

import "yunka.io/pkg/logExt"

// Memory is an explicit in-memory SMS sender for development and tests. It has
// no dependency on a package-global application logger.
type Memory struct {
	ID     string
	Logger logExt.Logger
}

func New(id string, logger logExt.Logger) *Memory {
	return &Memory{ID: id, Logger: logger}
}

func (sender *Memory) GetID() string { return sender.ID }

func (sender *Memory) SendSms(phone, smsCode string) error {
	if sender != nil && sender.Logger != nil {
		sender.Logger.Debugf("memory: id:%s send sms--- phone:%s code:%s\n", sender.ID, phone, smsCode)
	}
	return nil
}
