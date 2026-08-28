package operation

type RuntimeSnapshot struct {
	Phases           []Phase `json:"phases"`
	SecurityBound    bool    `json:"securityBound"`
	TransactionBound bool    `json:"transactionBound"`
	IdempotencyBound bool    `json:"idempotencyBound"`
	ChildExecution   bool    `json:"childExecution"`
	ObserverCount    int     `json:"observerCount"`
}

type snapshotter interface {
	Snapshot() RuntimeSnapshot
}

func (runtime *executor) Snapshot() RuntimeSnapshot {
	if runtime == nil {
		return RuntimeSnapshot{Phases: canonicalPhases()}
	}
	return RuntimeSnapshot{
		Phases:           canonicalPhases(),
		SecurityBound:    runtime.security != nil,
		TransactionBound: runtime.transactions != nil,
		IdempotencyBound: runtime.idempotency != nil,
		ChildExecution:   true,
		ObserverCount:    len(runtime.observers),
	}
}

func Snapshot(runtime Executor) (RuntimeSnapshot, bool) {
	if runtime == nil {
		return RuntimeSnapshot{}, false
	}
	value, ok := runtime.(snapshotter)
	if !ok {
		return RuntimeSnapshot{}, false
	}
	return value.Snapshot(), true
}

func canonicalPhases() []Phase {
	return []Phase{
		PhasePlan,
		PhaseMetadata,
		PhaseSecurity,
		PhaseIdempotencyBegin,
		PhaseExecutionScope,
		PhaseApplication,
		PhaseTransactionFinalize,
		PhaseIdempotencyFinalize,
		PhaseOutcome,
	}
}
