package resilience

// PeekSnapshot returns an existing operation snapshot without creating any
// breaker, token bucket, or load-shedder state. Diagnostics must use this
// method rather than Snapshot so observation remains side-effect free.
func (policy *RPCPolicy) PeekSnapshot(key string) (PolicySnapshot, bool) {
	var snapshot PolicySnapshot
	if policy == nil {
		return snapshot, false
	}
	found := false
	if policy.breakers != nil {
		if current, ok := policy.breakers.peekSnapshot(key); ok {
			snapshot.Circuit = current
			found = true
		}
	}
	if policy.limiters != nil {
		if current, ok := policy.limiters.peekSnapshot(key); ok {
			snapshot.Rate = current
			found = true
		}
	}
	if policy.shedders != nil {
		if current, ok := policy.shedders.peekSnapshot(key); ok {
			snapshot.Load = current
			found = true
		}
	}
	return snapshot, found
}

func (group *CircuitBreakerGroup) peekSnapshot(key string) (CircuitSnapshot, bool) {
	if group == nil {
		return CircuitSnapshot{}, false
	}
	group.mu.Lock()
	breaker := group.breakers[key]
	group.mu.Unlock()
	if breaker == nil {
		return CircuitSnapshot{}, false
	}
	return breaker.Snapshot(), true
}

func (group *RateLimiterGroup) peekSnapshot(key string) (RateLimitSnapshot, bool) {
	if group == nil {
		return RateLimitSnapshot{}, false
	}
	group.mu.Lock()
	bucket := group.buckets[key]
	group.mu.Unlock()
	if bucket == nil {
		return RateLimitSnapshot{}, false
	}
	return bucket.Snapshot(), true
}

func (group *LoadShedderGroup) peekSnapshot(key string) (LoadShedSnapshot, bool) {
	if group == nil {
		return LoadShedSnapshot{}, false
	}
	group.mu.Lock()
	shedder := group.shedders[key]
	group.mu.Unlock()
	if shedder == nil {
		return LoadShedSnapshot{}, false
	}
	return shedder.Snapshot(), true
}
