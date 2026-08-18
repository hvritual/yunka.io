package resilience

import (
	"context"
	"math"
	"runtime"
	"sync"
	"time"

	"yunka.io/framework/core/middleware"
)

type LoadShedConfig struct {
	Enabled        bool
	MinLimit       int
	MaxLimit       int
	InitialLimit   int
	TargetLatency  time.Duration
	IncreaseEvery  int
	DecreaseFactor float64
	MinimumBudget  time.Duration
	Overload       func(error) bool
}

type LoadShedSnapshot struct {
	Limit    int
	InFlight int
}

type LoadShedder struct {
	config    LoadShedConfig
	mu        sync.Mutex
	limit     int
	inFlight  int
	successes int
	now       func() time.Time
}

func NewLoadShedder(config LoadShedConfig) *LoadShedder {
	config = normalizeLoadShedConfig(config)
	return &LoadShedder{config: config, limit: config.InitialLimit, now: time.Now}
}

func normalizeLoadShedConfig(config LoadShedConfig) LoadShedConfig {
	if config.MinLimit <= 0 {
		config.MinLimit = 1
	}
	if config.MaxLimit <= 0 {
		config.MaxLimit = runtime.GOMAXPROCS(0) * 16
		if config.MaxLimit < 16 {
			config.MaxLimit = 16
		}
	}
	if config.InitialLimit <= 0 || config.InitialLimit > config.MaxLimit {
		config.InitialLimit = config.MaxLimit
	}
	if config.InitialLimit < config.MinLimit {
		config.InitialLimit = config.MinLimit
	}
	if config.TargetLatency <= 0 {
		config.TargetLatency = 250 * time.Millisecond
	}
	if config.IncreaseEvery <= 0 {
		config.IncreaseEvery = 20
	}
	if config.DecreaseFactor <= 0 || config.DecreaseFactor >= 1 {
		config.DecreaseFactor = 0.8
	}
	if config.MinimumBudget < 0 {
		config.MinimumBudget = 0
	}
	return config
}

func (shedder *LoadShedder) Execute(ctx context.Context, next middleware.Handler) (err error) {
	if shedder == nil || !shedder.config.Enabled {
		return next(ctx)
	}
	start, ok := shedder.acquire(ctx)
	if !ok {
		return ErrLoadShed
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			shedder.release(start, ErrLoadShed)
			panic(recovered)
		}
	}()
	err = next(ctx)
	shedder.release(start, err)
	return err
}

func (shedder *LoadShedder) acquire(ctx context.Context) (time.Time, bool) {
	if shedder.config.MinimumBudget > 0 {
		if remaining, ok := RemainingBudget(ctx); ok && remaining < shedder.config.MinimumBudget {
			return time.Time{}, false
		}
	}
	shedder.mu.Lock()
	defer shedder.mu.Unlock()
	if shedder.inFlight >= shedder.limit {
		return time.Time{}, false
	}
	shedder.inFlight++
	return shedder.now(), true
}

func (shedder *LoadShedder) release(start time.Time, err error) {
	latency := shedder.now().Sub(start)
	shedder.mu.Lock()
	defer shedder.mu.Unlock()
	if shedder.inFlight > 0 {
		shedder.inFlight--
	}
	overloaded := latency > shedder.config.TargetLatency
	if shedder.config.Overload != nil && shedder.config.Overload(err) {
		overloaded = true
	}
	if overloaded {
		newLimit := int(math.Floor(float64(shedder.limit) * shedder.config.DecreaseFactor))
		if newLimit < shedder.config.MinLimit {
			newLimit = shedder.config.MinLimit
		}
		shedder.limit = newLimit
		shedder.successes = 0
		return
	}
	if err != nil {
		shedder.successes = 0
		return
	}
	shedder.successes++
	if shedder.successes >= shedder.config.IncreaseEvery && shedder.limit < shedder.config.MaxLimit {
		shedder.limit++
		shedder.successes = 0
	}
}

func (shedder *LoadShedder) Snapshot() LoadShedSnapshot {
	if shedder == nil {
		return LoadShedSnapshot{}
	}
	shedder.mu.Lock()
	defer shedder.mu.Unlock()
	return LoadShedSnapshot{Limit: shedder.limit, InFlight: shedder.inFlight}
}

type LoadShedderGroup struct {
	config   LoadShedConfig
	mu       sync.Mutex
	shedders map[string]*LoadShedder
}

func NewLoadShedderGroup(config LoadShedConfig) *LoadShedderGroup {
	return &LoadShedderGroup{config: normalizeLoadShedConfig(config), shedders: make(map[string]*LoadShedder)}
}

func (group *LoadShedderGroup) Shedder(key string) *LoadShedder {
	if group == nil {
		return nil
	}
	group.mu.Lock()
	defer group.mu.Unlock()
	shedder := group.shedders[key]
	if shedder == nil {
		shedder = NewLoadShedder(group.config)
		group.shedders[key] = shedder
	}
	return shedder
}

func (group *LoadShedderGroup) Middleware(keyFn KeyFunc) middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context) error {
			key := resolveKey(ctx, keyFn)
			shedder := group.Shedder(key)
			if shedder == nil || !shedder.config.Enabled {
				return next(ctx)
			}
			err := shedder.Execute(ctx, next)
			if err == ErrLoadShed {
				return reject("load-shed", key, ErrLoadShed)
			}
			return err
		}
	}
}
