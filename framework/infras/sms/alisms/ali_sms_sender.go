package alisms

import (
	"fmt"
	"github.com/pkg/errors"
	"sync"
)

var (
	SendSmsError = errors.New("send message error")
)

type AliSmsSender struct {
	AccessID  string `json:"accessID"`
	AccessKEY string `json:"accessKEY"`
	TmplSign  string `json:"tmplSign"`
	TmplCode  string `json:"tmplCode"`
	Param     string `json:"param"`
	pool      *sync.Pool
}

func (aliSmsConfig *AliSmsSender) Init() *AliSmsSender {
	aliSmsConfig.pool = &sync.Pool{
		New: func() interface{} {
			return &UserParams{}
		},
	}
	return aliSmsConfig
}

func (aliSmsConfig *AliSmsSender) GetID() string {
	return aliSmsConfig.TmplCode
}

func (aliSmsConfig *AliSmsSender) GetUserParam() *UserParams {
	param := aliSmsConfig.pool.Get().(*UserParams)
	param.AccessKeyId = aliSmsConfig.AccessID
	param.AppSecret = aliSmsConfig.AccessKEY
	param.SignName = aliSmsConfig.TmplSign
	param.TemplateCode = aliSmsConfig.TmplCode
	return param
}

func (aliSmsConfig *AliSmsSender) PutUserParam(param *UserParams) {
	aliSmsConfig.pool.Put(param)
	return
}

func (aliSmsConfig *AliSmsSender) SendSms(phone, smsCode string) error {

	param := aliSmsConfig.GetUserParam()
	param.PhoneNumbers = phone
	param.TemplateParam = fmt.Sprintf(aliSmsConfig.Param, smsCode)
	ok, reasons, err := SendMessage(param)
	if ok && err == nil {
		return nil
	} else {
		if err != nil {
			return err
		}
		return errors.New(reasons)
	}
}
