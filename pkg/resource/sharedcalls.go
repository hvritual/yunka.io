package resource

import "sync"

type (
	// SharedCalls lets the concurrent calls with the same key to share the call result.
	// When A and B concurrently request the same key, A and B will share the same result.
	// Before the end of A request, B request comes, B will not execute, only wait for the result of A,
	SharedCalls interface {
		Do(key string, fn func() (interface{}, error)) (interface{}, error)
		DoEx(key string, fn func() (interface{}, error)) (interface{}, bool, error)
	}

	call struct {
		wg  sync.WaitGroup
		val interface{}
		err error
	}

	group struct {
		calls map[string]*call
		mu    sync.Mutex
	}
)

func NewSharedCalls() SharedCalls {
	return &group{
		calls: make(map[string]*call),
	}
}

func (g *group) Do(key string, fn func() (interface{}, error)) (interface{}, error) {
	c, done := g.createCall(key)
	if done {
		return c.val, c.err
	}

	g.doCall(c, key, fn)
	return c.val, c.err
}

func (g *group) DoEx(key string, fn func() (interface{}, error)) (val interface{}, fresh bool, err error) {
	c, done := g.createCall(key)
	if done {
		return c.val, false, c.err
	}

	g.doCall(c, key, fn)
	return c.val, true, c.err

}

func (g *group) doCall(c *call, key string, fn func() (interface{}, error)) {
	defer func() {
		g.mu.Lock()
		delete(g.calls, key)
		g.mu.Unlock()
		c.wg.Done()
	}()

	c.val, c.err = fn()
}

func (g *group) createCall(key string) (c *call, done bool) {
	g.mu.Lock()
	if c, ok := g.calls[key]; ok {
		g.mu.Unlock()
		c.wg.Wait()
		return c, true
	}

	c = new(call)
	c.wg.Add(1)
	g.calls[key] = c
	g.mu.Unlock()

	return c, false

}
