// Package selector is a way to pick a list of service nodes.
package selector

import (
	"errors"
	"sync"
	"time"

	"github.com/hvritual/yunka.io/pkg/registry"
)

// Selector builds on the registry as a mechanism to pick nodes and record
// passive health feedback. The legacy interface remains source-compatible;
// adaptive callers should prefer Picker.Pick so in-flight and latency feedback
// are recorded as one lifecycle.
type Selector interface {
	Init(opts ...Option) error
	Options() Options
	Select(service string, opts ...SelectOption) (Next, error)
	Mark(service string, node *registry.Node, err error)
	Reset(service string)
	Close() error
	String() string
}

// Picker is the W5 selection lifecycle. Pick increments the selected node's
// in-flight count and Selection.Done records latency and outcome exactly once.
type Picker interface {
	Pick(service string, opts ...SelectOption) (*Selection, error)
}

// Snapshotter exposes selector state for diagnostics and observability without
// leaking implementation internals.
type Snapshotter interface {
	Snapshot(service string) []NodeSnapshot
}

// Next is a legacy function that returns the next node.
type Next func() (*registry.Node, error)

// Filter is used to filter a service during the selection process.
type Filter func([]*registry.Service) []*registry.Service

// Strategy is a legacy stateless selection strategy e.g. random, round robin.
type Strategy func([]*registry.Service) Next

// Outcome classifies passive feedback. OutcomeAuto delegates classification to
// the selector's configured classifier.
type Outcome uint8

const (
	OutcomeAuto Outcome = iota
	OutcomeSuccess
	OutcomeFailure
	OutcomeIgnore
	OutcomeEject
)

// Selection is one adaptive pick lifecycle.
type Selection struct {
	Node *registry.Node

	started time.Time
	once    sync.Once
	finish  func(duration time.Duration, err error, outcome Outcome)
}

// Done records the call result using the configured classifier.
func (selection *Selection) Done(err error) {
	if selection == nil {
		return
	}
	selection.DoneWithDuration(err, time.Since(selection.started), OutcomeAuto)
}

// DoneWithDuration records an explicitly measured duration. This is useful for
// transports that already measure call latency and for deterministic tests.
func (selection *Selection) DoneWithDuration(err error, duration time.Duration, outcome Outcome) {
	if selection == nil || selection.finish == nil {
		return
	}
	selection.once.Do(func() { selection.finish(duration, err, outcome) })
}

// DoneWithOutcome records an explicit outcome. It is safe to call more than
// once; only the first completion updates selector state.
func (selection *Selection) DoneWithOutcome(err error, outcome Outcome) {
	if selection == nil {
		return
	}
	selection.DoneWithDuration(err, time.Since(selection.started), outcome)
}

// NodeSnapshot is a point-in-time view of passive node health.
type NodeSnapshot struct {
	Service             string
	Version             string
	NodeID              string
	Address             string
	EWMA                time.Duration
	InFlight            int64
	ConsecutiveFailures int
	Ejected             bool
	EjectedUntil        time.Time
	EjectionCount       int
	LastOutcome         Outcome
	Score               float64
	Selections          uint64
	Successes           uint64
	Failures            uint64
	Ignored             uint64
}

var (
	DefaultSelector = NewSelector()

	ErrNotFound      = errors.New("not found")
	ErrNoneAvailable = errors.New("none available")
	ErrInvalidMethod = errors.New("invalid rpc method")
)
