package aliLogStore

import (
	"encoding/json"
	"fmt"
	sls "github.com/aliyun/aliyun-log-go-sdk"
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
	now := uint32(time.Now().Unix())
	log := &sls.Log{
		Time:     &now,
		Contents: make([]*sls.LogContent, 0),
	}
	for key, val := range model {
		keyCopy := key
		valueCopy := fmt.Sprintf("%v", val)
		log.Contents = append(log.Contents, &sls.LogContent{
			Key:   &keyCopy,
			Value: &valueCopy,
		})
	}

	topic := w.topic
	source := w.source
	err = w.l.PutLogs(&sls.LogGroup{
		Logs:   []*sls.Log{log},
		Topic:  &topic,
		Source: &source,
	})

	if err != nil {
		return 0, err
	}

	return 0, nil
}
