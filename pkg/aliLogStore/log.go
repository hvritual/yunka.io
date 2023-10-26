package aliLogStore

import (
	sls "github.com/aliyun/aliyun-log-go-sdk"
	"github.com/golang/protobuf/proto"
	"time"
)

type Log struct {
	Conf *Config
}

type Content interface {
	Body() map[string]string
	Index() sls.Index
}

func NewAliLogStore(conf *Config) *Log {
	return &Log{Conf: conf}
}

func (l *Log) Put(data Content) error {
	return l.client().PutLogs(l.Conf.Project, l.Conf.LogStore, l.buildLogGroupFromMap(data.Body()))
}

func (l *Log) client() sls.ClientInterface {
	return sls.CreateNormalInterfaceV2(l.Conf.Endpoint, sls.NewStaticCredentialsProvider(l.Conf.AccessKeyId, l.Conf.AccessKeySecret, l.Conf.SecurityToken))
}

func (l *Log) buildLogGroupFromMap(data map[string]string) *sls.LogGroup {
	var contents = make([]*sls.LogContent, 0)

	for key, val := range data {
		contents = append(contents, &sls.LogContent{
			Key:   proto.String(key),
			Value: proto.String(val),
		})
	}

	var now = uint32(time.Now().Unix())

	var log = &sls.Log{
		Time:     &now,
		Contents: contents,
	}

	return &sls.LogGroup{
		Logs:                 []*sls.Log{log},
		Category:             nil,
		Topic:                &l.Conf.Topic,
		Source:               &l.Conf.Source,
		MachineUUID:          nil,
		LogTags:              nil,
		XXX_NoUnkeyedLiteral: struct{}{},
		XXX_unrecognized:     nil,
		XXX_sizecache:        0,
	}
}

func (l *Log) GetLogs(fromInNs int64, toInNs int64, queryExp string,
	maxLineNum int64, offset int64, reverse bool) (*sls.GetLogsResponse, error) {

	resp, err := l.client().GetLogs(l.Conf.Project, l.Conf.LogStore, l.Conf.Topic, fromInNs, toInNs, queryExp, maxLineNum, offset, reverse)

	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (l *Log) BuildIndex(data Content) error {
	return l.client().CreateIndex(l.Conf.Project, l.Conf.LogStore, data.Index())
}
