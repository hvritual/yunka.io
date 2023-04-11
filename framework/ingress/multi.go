package ingress

import (
	"sync"
	"yunka.io/pkg/stringsExt"

	"github.com/buger/jsonparser"
)

var (
	emptyObject = []byte(`{}`)
)

type multiContext struct {
	sync.RWMutex
	data []byte
}

func (c *multiContext) Reset() {
	c.Init()
}

func (c *multiContext) Init() {
	c.data = emptyObject
}

func (c *multiContext) CompletePart(attr string, data []byte) {
	c.Lock()
	if len(data) > 0 && attr != "" {
		c.data, _ = jsonparser.Set(c.data, data, attr)
	}
	c.Unlock()
}

func (c *multiContext) GetData() []byte {
	return c.data
}

func (c *multiContext) getAttr(paths ...string) string {
	c.RLock()
	value, _, _, err := jsonparser.Get(c.data, paths...)
	c.RUnlock()
	if err != nil {
		//log.Errorf("extract %+v failed, errors:\n%+v", paths, err)
		return ""
	}

	return stringsExt.SliceToString(value)
}
