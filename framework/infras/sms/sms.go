package sms

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/go-redis/redis"
	"math/rand"
	"sync"
	"time"
	"yunka.io/pkg/util"
)

/**
* @Description: TODO
* @date 2019-07-18
* @version V1.0
 */

const (
	maxPutCacheTime = 5
)

var (
	ErrPhoneHasSendCode = errors.New("phone has code")
	ErrEmptyEnvironment = errors.New("empty environment")
	ErrPhoneNotExist    = errors.New("phone not exit")
)

type (
	Sms struct {
		cache      redis.Cmdable
		smsSenders []Sender
		pool       *sync.Pool
		sendLen    int
		lock       sync.Locker
	}
)

type Sender interface {
	SendSms(phone, smsCode string) error
	GetID() string
}

func init() {
	rand.Seed(time.Now().Unix())
}

func NewSms(cache redis.Cmdable, senders ...Sender) (*Sms, error) {
	smsLen := len(senders)
	if smsLen == 0 {
		return nil, errors.New("not allow empty send")
	}
	return &Sms{
		cache:      cache,
		smsSenders: senders,
		sendLen:    smsLen,
		lock:       &sync.Mutex{},
		pool: &sync.Pool{
			New: func() interface{} {
				return &Environment{}
			},
		},
	}, nil
}

func (s *Sms) random() []Sender {
	s.lock.Lock()
	defer s.lock.Unlock()
	for i := s.sendLen - 1; i > 0; i-- {
		num := rand.Intn(i + 1)
		s.smsSenders[i], s.smsSenders[num] = s.smsSenders[num], s.smsSenders[i]
	}
	return s.smsSenders
}

func (s *Sms) getKey(phone string, keys []string) string {
	var bys bytes.Buffer
	for _, key := range keys {
		bys.WriteString(key)
	}
	bys.WriteString(phone)
	return bys.String()
}

func (*Sms) getCheckCode() string {
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	return fmt.Sprintf("%06v", rnd.Int31n(1000000))
}

func (s *Sms) GetEnvironment() *Environment {
	return s.pool.Get().(*Environment)
}

func (s *Sms) SendTTL(e *Environment, ttl int, phone string, keys ...string) error {
	if e == nil {
		return ErrEmptyEnvironment
	}
	defer s.pool.Put(e)
	_, err := s.GetCacheSms(phone, keys...)
	if err == nil {
		return ErrPhoneHasSendCode
	}
	code := s.getCheckCode()
	e.Code = code
	bys, err := util.EncodeMsg(e)
	if err != nil {
		return err
	}
	s.pool.Put(e)
	err = (s.random()[0]).SendSms(phone, code)
	if err != nil {
		return err
	}
	key := s.getKey(phone, keys)
	return s.cache.Set(key, bys, time.Duration(ttl)*time.Second).Err()
}

func (s *Sms) SendTTLTryBest(e *Environment, ttl int, phone string, keys ...string) error {
	if e == nil {
		return ErrEmptyEnvironment
	}
	defer s.pool.Put(e)
	_, err := s.GetCacheSms(phone, keys...)
	if err == nil {
		return ErrPhoneHasSendCode
	}
	code := s.getCheckCode()
	e.Code = code
	bys, err := util.EncodeMsg(e)
	if err != nil {
		return err
	}
	s.pool.Put(e)

	sends := s.random()
	for i := 0; i < s.sendLen; i++ {
		err = (sends[i]).SendSms(phone, code)
		if err == nil {
			break
		}
	}
	if err != nil {
		return err
	}

	key := s.getKey(phone, keys)
	for i := 0; i < maxPutCacheTime; i++ {
		statusCmd := s.cache.Set(key, bys, time.Duration(ttl)*time.Second)
		err = statusCmd.Err()
		if err == nil {
			break
		}

	}
	return err
}

func (s *Sms) getPhoneEnvironment(phone string, keys ...string) (*Environment, bool, error) {
	bys, err := s.GetCacheSms(phone, keys...)
	if err != nil {
		return nil, false, ErrPhoneNotExist
	}

	e := s.GetEnvironment()
	defer s.pool.Put(e)

	err = util.DecodeMsg(bys, e)
	if err != nil {
		return nil, false, err
	}
	return e, false, nil
}

func (s *Sms) GetCacheSms(phone string, keys ...string) ([]byte, error) {
	key := s.getKey(phone, keys)
	return s.cache.Get(key).Bytes()
}

func (s *Sms) CheckCode(phone, code string, keys ...string) (bool, error) {
	e, b, err := s.getPhoneEnvironment(phone, keys...)
	if err != nil {
		return b, err
	}
	return e.Code == code, nil
}

func (s *Sms) CheckCodeForce(checkE *Environment, phone, code string, keys ...string) (bool, error) {
	e, b, err := s.getPhoneEnvironment(phone, keys...)
	if err != nil {
		return b, err
	}

	if e.Code != code {
		return false, nil
	}
	if e.ClientIP != checkE.ClientIP {
		return false, nil
	}

	if e.Browser != checkE.Browser {
		return false, nil
	}

	return true, nil
}

func (s *Sms) ForgetCode(phone string, keys ...string) error {
	return s.cache.Del(s.getKey(phone, keys)).Err()
}
