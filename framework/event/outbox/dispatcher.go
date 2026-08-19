package outbox

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"yunka.io/framework/event"
)

type Observer interface {
	Published(context.Context, Record)
	Retried(context.Context, Record, error, time.Time)
	DeadLettered(context.Context, Record, error)
}

type NopObserver struct{}

func (NopObserver) Published(context.Context, Record)                 {}
func (NopObserver) Retried(context.Context, Record, error, time.Time) {}
func (NopObserver) DeadLettered(context.Context, Record, error)       {}

type DispatcherConfig struct {
	WorkerID               string
	PollInterval           time.Duration
	BatchSize              int
	Concurrency            int
	LeaseDuration          time.Duration
	PublishTimeout         time.Duration
	MaxAttempts            int
	RetryBase              time.Duration
	RetryMax               time.Duration
	RetryJitter            float64
	HealthFailureThreshold int
	Observer               Observer
	Now                    func() time.Time
	Rand                   *rand.Rand
}

type Dispatcher struct {
	store  Store
	broker event.Broker
	config DispatcherConfig

	mu          sync.Mutex
	running     bool
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	failures    int
	lastLoopErr error
	randMu      sync.Mutex
}

func NewDispatcher(store Store, broker event.Broker, config DispatcherConfig) (*Dispatcher, error) {
	if store == nil {
		return nil, errors.New("outbox: store is required")
	}
	if broker == nil {
		return nil, errors.New("outbox: broker is required")
	}
	config = normalizeDispatcherConfig(config)
	if err := validateDispatcherConfig(config); err != nil {
		return nil, err
	}
	return &Dispatcher{store: store, broker: broker, config: config}, nil
}

func normalizeDispatcherConfig(config DispatcherConfig) DispatcherConfig {
	if config.WorkerID == "" {
		config.WorkerID = "outbox-" + fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if config.PollInterval <= 0 {
		config.PollInterval = 500 * time.Millisecond
	}
	if config.BatchSize <= 0 {
		config.BatchSize = 50
	}
	if config.Concurrency <= 0 {
		config.Concurrency = 4
	}
	if config.LeaseDuration <= 0 {
		config.LeaseDuration = 30 * time.Second
	}
	if config.PublishTimeout <= 0 {
		config.PublishTimeout = 10 * time.Second
	}
	if config.MaxAttempts <= 0 {
		config.MaxAttempts = 8
	}
	if config.RetryBase <= 0 {
		config.RetryBase = time.Second
	}
	if config.RetryMax <= 0 {
		config.RetryMax = 5 * time.Minute
	}
	if config.RetryMax < config.RetryBase {
		config.RetryMax = config.RetryBase
	}
	if config.RetryJitter < 0 {
		config.RetryJitter = 0
	}
	if config.RetryJitter > 1 {
		config.RetryJitter = 1
	}
	if config.HealthFailureThreshold <= 0 {
		config.HealthFailureThreshold = 5
	}
	if config.Observer == nil {
		config.Observer = NopObserver{}
	}
	if config.Now == nil {
		config.Now = func() time.Time { return time.Now().UTC() }
	}
	if config.Rand == nil {
		config.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	return config
}

func validateDispatcherConfig(config DispatcherConfig) error {
	waves := (config.BatchSize + config.Concurrency - 1) / config.Concurrency
	minimumLease := time.Duration(waves) * config.PublishTimeout
	if config.LeaseDuration < minimumLease {
		return fmt.Errorf("outbox: lease duration %s is shorter than worst-case batch publish window %s", config.LeaseDuration, minimumLease)
	}
	return nil
}

func (dispatcher *Dispatcher) Start(ctx context.Context) error {
	if dispatcher == nil {
		return errors.New("outbox: dispatcher is nil")
	}
	dispatcher.mu.Lock()
	defer dispatcher.mu.Unlock()
	if dispatcher.running {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	runCtx, cancel := context.WithCancel(ctx)
	dispatcher.cancel = cancel
	dispatcher.running = true
	dispatcher.failures = 0
	dispatcher.lastLoopErr = nil
	dispatcher.wg.Add(1)
	go dispatcher.loop(runCtx)
	return nil
}

func (dispatcher *Dispatcher) loop(ctx context.Context) {
	defer dispatcher.wg.Done()
	ticker := time.NewTicker(dispatcher.config.PollInterval)
	defer ticker.Stop()
	for {
		if err := dispatcher.RunOnce(ctx); err != nil {
			dispatcher.recordLoopFailure(err)
		} else {
			dispatcher.recordLoopSuccess()
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (dispatcher *Dispatcher) Shutdown(ctx context.Context) error {
	if dispatcher == nil {
		return nil
	}
	dispatcher.mu.Lock()
	if !dispatcher.running {
		dispatcher.mu.Unlock()
		return nil
	}
	cancel := dispatcher.cancel
	dispatcher.running = false
	dispatcher.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	done := make(chan struct{})
	go func() { dispatcher.wg.Wait(); close(done) }()
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (dispatcher *Dispatcher) Health(context.Context) error {
	if dispatcher == nil {
		return errors.New("outbox: dispatcher is nil")
	}
	dispatcher.mu.Lock()
	defer dispatcher.mu.Unlock()
	if !dispatcher.running {
		return errors.New("outbox: dispatcher not running")
	}
	if dispatcher.failures >= dispatcher.config.HealthFailureThreshold {
		return fmt.Errorf("outbox: dispatcher unhealthy after %d failures: %w", dispatcher.failures, dispatcher.lastLoopErr)
	}
	return nil
}
func (dispatcher *Dispatcher) recordLoopFailure(err error) {
	dispatcher.mu.Lock()
	dispatcher.failures++
	dispatcher.lastLoopErr = err
	dispatcher.mu.Unlock()
}
func (dispatcher *Dispatcher) recordLoopSuccess() {
	dispatcher.mu.Lock()
	dispatcher.failures = 0
	dispatcher.lastLoopErr = nil
	dispatcher.mu.Unlock()
}

func (dispatcher *Dispatcher) RunOnce(ctx context.Context) error {
	if dispatcher == nil {
		return errors.New("outbox: dispatcher is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	now := dispatcher.config.Now()
	records, err := dispatcher.store.Claim(ctx, ClaimOptions{Owner: dispatcher.config.WorkerID, Limit: dispatcher.config.BatchSize, Lease: dispatcher.config.LeaseDuration, Now: now})
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return nil
	}
	sem := make(chan struct{}, dispatcher.config.Concurrency)
	errs := make(chan error, len(records))
	var wg sync.WaitGroup
	for _, record := range records {
		record := record
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				errs <- ctx.Err()
				return
			}
			defer func() { <-sem }()
			if err := dispatcher.process(ctx, record); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	var joined error
	for err := range errs {
		joined = errors.Join(joined, err)
	}
	return joined
}

func (dispatcher *Dispatcher) process(ctx context.Context, record Record) error {
	if record.Attempts > dispatcher.config.MaxAttempts {
		cause := errors.New("outbox: maximum attempts exceeded before publish")
		if err := dispatcher.store.DeadLetter(ctx, record.ID, dispatcher.config.WorkerID, cause); err != nil {
			return err
		}
		safeDeadLettered(dispatcher.config.Observer, ctx, record, cause)
		return nil
	}
	publishCtx, cancel := context.WithTimeout(ctx, dispatcher.config.PublishTimeout)
	defer cancel()
	envelope := record.Envelope.Clone()
	envelope.DeliveryAttempt = record.Attempts
	err := safePublish(dispatcher.broker, publishCtx, envelope)
	if err == nil {
		at := dispatcher.config.Now()
		if markErr := dispatcher.store.MarkPublished(ctx, record.ID, dispatcher.config.WorkerID, at); markErr != nil {
			return markErr
		}
		record.Status = StatusPublished
		record.PublishedAt = at
		safePublished(dispatcher.config.Observer, ctx, record)
		return nil
	}
	if record.Attempts >= dispatcher.config.MaxAttempts {
		if deadErr := dispatcher.store.DeadLetter(ctx, record.ID, dispatcher.config.WorkerID, err); deadErr != nil {
			return errors.Join(err, deadErr)
		}
		record.Status = StatusDeadLetter
		safeDeadLettered(dispatcher.config.Observer, ctx, record, err)
		return nil
	}
	next := dispatcher.config.Now().Add(dispatcher.backoff(record.Attempts))
	if retryErr := dispatcher.store.Retry(ctx, record.ID, dispatcher.config.WorkerID, next, err); retryErr != nil {
		return errors.Join(err, retryErr)
	}
	record.Status = StatusPending
	record.NextAttemptAt = next
	safeRetried(dispatcher.config.Observer, ctx, record, err, next)
	return nil
}

func (dispatcher *Dispatcher) backoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	exponent := float64(attempt - 1)
	if exponent > 30 {
		exponent = 30
	}
	delay := time.Duration(float64(dispatcher.config.RetryBase) * math.Pow(2, exponent))
	if delay > dispatcher.config.RetryMax {
		delay = dispatcher.config.RetryMax
	}
	if dispatcher.config.RetryJitter <= 0 {
		return delay
	}
	dispatcher.randMu.Lock()
	sample := dispatcher.config.Rand.Float64()
	dispatcher.randMu.Unlock()
	factor := 1 - dispatcher.config.RetryJitter + 2*dispatcher.config.RetryJitter*sample
	jittered := time.Duration(float64(delay) * factor)
	if jittered < 0 {
		return 0
	}
	if jittered > dispatcher.config.RetryMax {
		return dispatcher.config.RetryMax
	}
	return jittered
}

func (dispatcher *Dispatcher) Snapshot(ctx context.Context) (Snapshot, error) {
	if dispatcher == nil {
		return Snapshot{}, errors.New("outbox: dispatcher is nil")
	}
	return dispatcher.store.Snapshot(ctx)
}

func safePublish(broker event.Broker, ctx context.Context, envelope event.Envelope) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("outbox: broker panic: %v", recovered)
		}
	}()
	return broker.Publish(ctx, envelope)
}

func safePublished(observer Observer, ctx context.Context, record Record) {
	defer func() { _ = recover() }()
	observer.Published(ctx, record)
}
func safeRetried(observer Observer, ctx context.Context, record Record, cause error, next time.Time) {
	defer func() { _ = recover() }()
	observer.Retried(ctx, record, cause, next)
}
func safeDeadLettered(observer Observer, ctx context.Context, record Record, cause error) {
	defer func() { _ = recover() }()
	observer.DeadLettered(ctx, record, cause)
}
