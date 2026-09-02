package selector

import (
	"context"
	"errors"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/hvritual/yunka.io/pkg/registry"
)

type nodeState struct {
	service string
	version string
	key     string
	node    *registry.Node

	ewma                float64
	inflight            int64
	consecutiveFailures int
	ejectedUntil        time.Time
	ejectionCount       int
	lastOutcome         Outcome
	selections          uint64
	successes           uint64
	failures            uint64
	ignored             uint64
}

type adaptiveCandidate struct {
	state *nodeState
	node  *registry.Node
}

func defaultFeedbackClassifier(err error) Outcome {
	if err == nil {
		return OutcomeSuccess
	}
	if errors.Is(err, context.Canceled) {
		return OutcomeIgnore
	}
	return OutcomeFailure
}

func cloneNode(node *registry.Node) *registry.Node {
	if node == nil {
		return nil
	}
	clone := *node
	if node.Metadata != nil {
		clone.Metadata = make(map[string]string, len(node.Metadata))
		for key, value := range node.Metadata {
			clone.Metadata[key] = value
		}
	}
	return &clone
}

func nodeIdentity(version string, node *registry.Node) string {
	if node == nil {
		return ""
	}
	identity := strings.TrimSpace(node.Id)
	if identity == "" {
		identity = strings.TrimSpace(node.Address)
	}
	return version + "\x00" + identity
}

func (c *registrySelector) stateCandidates(service string, services []*registry.Service) []adaptiveCandidate {
	now := c.so.Adaptive.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stateCandidatesLocked(service, services, now)
}

func (c *registrySelector) stateCandidatesLocked(service string, services []*registry.Service, now time.Time) []adaptiveCandidate {
	if c.states == nil {
		c.states = make(map[string]map[string]*nodeState)
	}
	states := c.states[service]
	if states == nil {
		states = make(map[string]*nodeState)
		c.states[service] = states
	}

	seen := make(map[string]struct{})
	candidates := make([]adaptiveCandidate, 0)
	for _, registered := range services {
		if registered == nil {
			continue
		}
		for _, node := range registered.Nodes {
			if node == nil {
				continue
			}
			key := nodeIdentity(registered.Version, node)
			if key == "" {
				continue
			}
			seen[key] = struct{}{}
			state := states[key]
			if state == nil {
				state = &nodeState{
					version: registered.Version,
					key:     key,
					node:    cloneNode(node),
					ewma:    float64(c.so.Adaptive.InitialLatency),
				}
				states[key] = state
			} else {
				state.node = cloneNode(node)
				state.version = registered.Version
			}
			candidates = append(candidates, adaptiveCandidate{state: state, node: cloneNode(node)})
		}
	}

	// Only reconcile against the unfiltered registry snapshot. Callers invoke
	// this before Select filters so version/locality filters do not delete state
	// for temporarily excluded nodes.
	for key := range states {
		if _, ok := seen[key]; !ok {
			delete(states, key)
		}
	}
	return candidates
}

func (c *registrySelector) candidatesForFiltered(service string, services []*registry.Service) []adaptiveCandidate {
	c.mu.Lock()
	defer c.mu.Unlock()
	states := c.states[service]
	if states == nil {
		return nil
	}
	candidates := make([]adaptiveCandidate, 0)
	for _, registered := range services {
		if registered == nil {
			continue
		}
		for _, node := range registered.Nodes {
			key := nodeIdentity(registered.Version, node)
			if state := states[key]; state != nil {
				candidates = append(candidates, adaptiveCandidate{state: state, node: cloneNode(node)})
			}
		}
	}
	return candidates
}

func (c *registrySelector) choose(service string, services []*registry.Service, mode AdaptiveMode, track bool) (*Selection, error) {
	candidates := c.candidatesForFiltered(service, services)
	if len(candidates) == 0 {
		return nil, ErrNoneAvailable
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.so.Adaptive.Now()
	states := c.states[service]
	available := make([]adaptiveCandidate, 0, len(candidates))
	currentCandidates := make([]adaptiveCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if states == nil || states[candidate.state.key] != candidate.state {
			continue
		}
		currentCandidates = append(currentCandidates, candidate)
		if !candidate.state.ejectedUntil.After(now) {
			available = append(available, candidate)
		}
	}
	if len(currentCandidates) == 0 {
		return nil, ErrNoneAvailable
	}
	if len(available) == 0 {
		if c.so.Adaptive.Outlier.FailClosed {
			return nil, ErrNoneAvailable
		}
		available = currentCandidates
	}

	chosen := c.chooseLocked(available, mode)
	if chosen.state == nil || chosen.node == nil {
		return nil, ErrNoneAvailable
	}
	chosen.state.selections++
	if track {
		chosen.state.inflight++
	}
	selection := &Selection{
		Node:    cloneNode(chosen.node),
		started: time.Now(),
	}
	key := chosen.state.key
	selection.finish = func(duration time.Duration, err error, outcome Outcome) {
		c.record(service, key, duration, err, outcome, track)
	}
	return selection, nil
}

func (c *registrySelector) chooseLocked(candidates []adaptiveCandidate, mode AdaptiveMode) adaptiveCandidate {
	if len(candidates) == 1 {
		return candidates[0]
	}
	if mode == "" {
		mode = c.so.Adaptive.Mode
	}
	switch mode {
	case AdaptiveEWMA:
		best := candidates[0]
		for _, candidate := range candidates[1:] {
			if c.scoreLocked(candidate.state) < c.scoreLocked(best.state) {
				best = candidate
			}
		}
		return best
	case AdaptiveLeastRequest:
		best := candidates[0]
		for _, candidate := range candidates[1:] {
			if candidate.state.inflight < best.state.inflight ||
				(candidate.state.inflight == best.state.inflight && c.scoreLocked(candidate.state) < c.scoreLocked(best.state)) {
				best = candidate
			}
		}
		return best
	case AdaptiveP2C:
		fallthrough
	default:
		first := c.rng.Intn(len(candidates))
		second := c.rng.Intn(len(candidates) - 1)
		if second >= first {
			second++
		}
		left, right := candidates[first], candidates[second]
		if c.scoreLocked(left.state) <= c.scoreLocked(right.state) {
			return left
		}
		return right
	}
}

func (c *registrySelector) scoreLocked(state *nodeState) float64 {
	latency := state.ewma
	if latency <= 0 {
		latency = float64(c.so.Adaptive.InitialLatency)
	}
	penalty := 1 + float64(state.consecutiveFailures)*c.so.Adaptive.FailurePenalty
	return latency * float64(state.inflight+1) * penalty
}

func (c *registrySelector) record(service, key string, duration time.Duration, err error, outcome Outcome, tracked bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	states := c.states[service]
	if states == nil {
		return
	}
	state := states[key]
	if state == nil {
		return
	}
	if tracked && state.inflight > 0 {
		state.inflight--
	}
	if outcome == OutcomeAuto {
		outcome = c.so.Adaptive.Classify(err)
	}
	state.lastOutcome = outcome
	if duration > 0 && outcome != OutcomeIgnore {
		observed := float64(duration)
		if state.ewma <= 0 {
			state.ewma = observed
		} else {
			alpha := c.so.Adaptive.EWMAAlpha
			state.ewma = alpha*observed + (1-alpha)*state.ewma
		}
	}

	switch outcome {
	case OutcomeSuccess:
		state.successes++
		state.consecutiveFailures = 0
		state.ejectionCount = 0
	case OutcomeIgnore:
		state.ignored++
		return
	case OutcomeEject:
		state.failures++
		state.consecutiveFailures = c.so.Adaptive.Outlier.ConsecutiveFailures
		c.maybeEjectLocked(service, state, true)
	default:
		state.failures++
		state.consecutiveFailures++
		c.maybeEjectLocked(service, state, false)
	}
}

func (c *registrySelector) maybeEjectLocked(service string, state *nodeState, force bool) {
	if !force && state.consecutiveFailures < c.so.Adaptive.Outlier.ConsecutiveFailures {
		return
	}
	states := c.states[service]
	if len(states) <= 1 {
		return
	}
	now := c.so.Adaptive.Now()
	active := 0
	for _, current := range states {
		if current.ejectedUntil.After(now) {
			active++
		}
	}
	maxEjected := len(states) * c.so.Adaptive.Outlier.MaxEjectionPercent / 100
	if maxEjected < 1 {
		return
	}
	if active >= maxEjected && !state.ejectedUntil.After(now) {
		return
	}

	duration := c.so.Adaptive.Outlier.BaseEjectionTime
	multiplier := math.Pow(2, float64(state.ejectionCount))
	if multiplier > float64(c.so.Adaptive.Outlier.MaxEjectionTime)/float64(duration) {
		duration = c.so.Adaptive.Outlier.MaxEjectionTime
	} else {
		duration = time.Duration(float64(duration) * multiplier)
		if duration > c.so.Adaptive.Outlier.MaxEjectionTime {
			duration = c.so.Adaptive.Outlier.MaxEjectionTime
		}
	}
	state.ejectionCount++
	state.ejectedUntil = now.Add(duration)
	state.consecutiveFailures = 0
}

func (c *registrySelector) Snapshot(service string) []NodeSnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.so.Adaptive.Now()
	states := c.states[service]
	result := make([]NodeSnapshot, 0, len(states))
	for _, state := range states {
		result = append(result, NodeSnapshot{
			Service:             service,
			Version:             state.version,
			NodeID:              state.node.Id,
			Address:             state.node.Address,
			EWMA:                time.Duration(state.ewma),
			InFlight:            state.inflight,
			ConsecutiveFailures: state.consecutiveFailures,
			Ejected:             state.ejectedUntil.After(now),
			EjectedUntil:        state.ejectedUntil,
			EjectionCount:       state.ejectionCount,
			LastOutcome:         state.lastOutcome,
			Score:               c.scoreLocked(state),
			Selections:          state.selections,
			Successes:           state.successes,
			Failures:            state.failures,
			Ignored:             state.ignored,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].NodeID == result[j].NodeID {
			return result[i].Address < result[j].Address
		}
		return result[i].NodeID < result[j].NodeID
	})
	return result
}
