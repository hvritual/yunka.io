package resource

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestGroup_Do_Exclusive(t *testing.T) {

	// success
	g := NewSharedCalls()

	do, err := g.Do("foo", func() (interface{}, error) {
		return "bar", nil
	})

	if err != nil {
		t.Errorf("do err, e: %s", err.Error())
	}

	if got, want := fmt.Sprintf("%v-%T", do, do), "bar-string"; got != want {
		t.Errorf("do = %v, want: %v", got, want)
	}

	// error
	g2 := NewSharedCalls()
	errFoo := errors.New("foo err")
	doE, errE := g2.Do("foo err", func() (interface{}, error) {
		return nil, errFoo
	})

	if errE != errFoo {
		t.Errorf("Do error = %v; want errFoo", err)
	}

	if doE != nil {
		t.Errorf("unexpected non-nil value %#v", doE)
	}

}

func TestGroup_Do_Concurrent(t *testing.T) {

	g := NewSharedCalls()

	var (
		counter int32
		c       = make(chan string)
	)
	fn := func() (interface{}, error) {
		atomic.AddInt32(&counter, 1)
		return <-c, nil
	}

	var (
		loop = 10
		wg   sync.WaitGroup
	)

	for i := 0; i < loop; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			do, err := g.Do("foo", fn)
			if err != nil {
				t.Errorf("do err, e: %s", err.Error())
			}
			if v, ok := do.(string); !ok || v != "bar" {
				t.Errorf("do = %v, want: %v", v, "bar")
			}
		}()
	}

	time.Sleep(100 * time.Millisecond) // let all goroutines above block
	c <- "bar"

	wg.Wait()

	if v := atomic.LoadInt32(&counter); v != 1 {
		t.Errorf("do = %v, want: %v", v, 1)

	}
}

func TestGroup_Do_Concurrent_DiffKey(t *testing.T) {
	g := NewSharedCalls()

	var (
		broadcast = make(chan struct{})
		counter   int32
		cases     = []string{"e", "a", "e", "a", "b", "c", "b", "a", "c", "d", "b", "c", "d"}
		wg        sync.WaitGroup
	)

	fn := func() (interface{}, error) {
		atomic.AddInt32(&counter, 1)
		time.Sleep(10 * time.Millisecond)
		return nil, nil
	}

	for _, v := range cases {
		wg.Add(1)

		v := v
		go func() {
			defer wg.Done()
			<-broadcast
			_, err := g.Do(v, fn)
			if err != nil {
				t.Errorf("Do error: %v", err)
			}
		}()
	}

	close(broadcast)
	wg.Wait()

	if got := atomic.LoadInt32(&counter); got != 5 {
		t.Errorf("number of calls = %d; want 5", got)
	}

}

func TestGroup_DoEx(t *testing.T) {
	g := NewSharedCalls()
	c := make(chan string)
	var calls int32
	fn := func() (interface{}, error) {
		atomic.AddInt32(&calls, 1)
		return <-c, nil
	}

	const n = 10
	var wg sync.WaitGroup
	var freshes int32
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			v, fresh, err := g.DoEx("key", fn)
			if err != nil {
				t.Errorf("Do error: %v", err)
			}
			if fresh {
				atomic.AddInt32(&freshes, 1)
			}
			if v.(string) != "bar" {
				t.Errorf("got %q; want %q", v, "bar")
			}
			wg.Done()
		}()
	}
	time.Sleep(100 * time.Millisecond) // let goroutines above block
	c <- "bar"
	wg.Wait()
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("number of calls = %d; want 1", got)
	}
	if got := atomic.LoadInt32(&freshes); got != 1 {
		t.Errorf("freshes = %d; want 1", got)
	}
}
