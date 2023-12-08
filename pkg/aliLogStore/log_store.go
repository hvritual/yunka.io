package aliLogStore

import (
	"encoding/json"
	"fmt"
	sls "github.com/aliyun/aliyun-log-go-sdk"
	"github.com/gogo/protobuf/proto"
	"io"
	"time"
)

type logStoreWrite struct {
	l             *sls.LogStore
	topic, source string
}

type WithWriteOption func(l *logStoreWrite)

func WithTopicOption(topic string) WithWriteOption {
	return func(l *logStoreWrite) {
		l.topic = topic
	}
}
func WithSourceOption(source string) WithWriteOption {
	return func(l *logStoreWrite) {
		l.source = source
	}
}

func NewLogStoreWrite(store *sls.LogStore, ops ...WithWriteOption) io.Writer {
	var w = &logStoreWrite{
		l:      store,
		topic:  "default",
		source: "default",
	}

	for i := 0; i < len(ops); i++ {
		ops[i](w)
	}

	return w
}

func (w *logStoreWrite) Write(body []byte) (int, error) {
	var model = make(map[string]interface{})

	err := json.Unmarshal(body, &model)
	if err != nil {
		return 0, err
	}
	var log = &sls.Log{
		Time:     proto.Uint32(uint32(time.Now().Unix())),
		Contents: make([]*sls.LogContent, 0),
	}
	for key, val := range model {
		log.Contents = append(log.Contents, &sls.LogContent{
			Key:   proto.String(key),
			Value: proto.String(fmt.Sprintf("%v", val)),
		})
	}

	err = w.l.PutLogs(&sls.LogGroup{
		Logs:   []*sls.Log{log},
		Topic:  proto.String(w.topic),
		Source: proto.String(w.source),
	})

	if err != nil {
		return 0, err
	}

	return 0, nil
}
