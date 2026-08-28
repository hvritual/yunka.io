from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def edit(path: str, old: str, new: str) -> None:
    target = ROOT / path
    text = target.read_text()
    if old not in text:
        raise SystemExit(f"expected fragment not found in {path}: {old[:180]!r}")
    target.write_text(text.replace(old, new, 1))


def append(path: str, text: str) -> None:
    target = ROOT / path
    current = target.read_text()
    if text.strip() not in current:
        target.write_text(current.rstrip() + "\n\n" + text.lstrip())

edit(
    "framework/execution/idempotency.go",
    '''type AtomicIdempotencyCoordinator interface {
\tIdempotencyCoordinator
\tCompleteInTransaction(context.Context, operationplan.Plan, any) error
}
''',
    '''type AtomicIdempotencyCoordinator interface {
\tIdempotencyCoordinator
\tCompleteInTransaction(context.Context, operationplan.Plan, any) error
}

// IdempotencyCapabilityReporter lets the Executor distinguish a durable store
// that can join the business transaction from compatibility stores that only
// support post-commit completion.
type IdempotencyCapabilityReporter interface {
\tSupportsAtomicCompletion() bool
}
''',
)
edit(
    "framework/execution/idempotency.go",
    '''func (coordinator *coordinator) CompleteInTransaction(ctx context.Context, _ operationplan.Plan, transaction any) error {
''',
    '''func (coordinator *coordinator) SupportsAtomicCompletion() bool {
\tif coordinator == nil || coordinator.store == nil {
\t\treturn false
\t}
\t_, ok := coordinator.store.(TransactionalIdempotencyStore)
\treturn ok
}

func (coordinator *coordinator) CompleteInTransaction(ctx context.Context, _ operationplan.Plan, transaction any) error {
''',
)

edit(
    "pkg/operationplan/plan.go",
    '''\t\tswitch item.Execution.Idempotency {
\t\tcase "none", "required":
\t\tdefault:
\t\t\treturn fmt.Errorf("operationplan: operation %s has invalid idempotency policy %q", item.OperationID, item.Execution.Idempotency)
\t\t}
''',
    '''\t\tswitch item.Execution.Idempotency {
\t\tcase "none", "required":
\t\tdefault:
\t\t\treturn fmt.Errorf("operationplan: operation %s has invalid idempotency policy %q", item.OperationID, item.Execution.Idempotency)
\t\t}
\t\tif item.Execution.Idempotency == "required" && item.Execution.Transaction != "local" {
\t\t\treturn fmt.Errorf("operationplan: operation %s requires local transaction for durable idempotency", item.OperationID)
\t\t}
''',
)

edit(
    "framework/operation/executor.go",
    '''\truntime.observe(ctx, plan.OperationID, InvocationRoot, PhaseTransactionFinalize, OutcomeStarted)
\tif commitErr := root.Commit(ctx); commitErr != nil {
''',
    '''\truntime.observe(ctx, plan.OperationID, InvocationRoot, PhaseTransactionFinalize, OutcomeStarted)
\tatomicIdempotency, atomicErr := runtime.stageAtomicIdempotency(ctx, plan, idempotent)
\tif atomicErr != nil {
\t\trollbackErr := root.Rollback(ctx)
\t\truntime.observe(ctx, plan.OperationID, InvocationRoot, PhaseTransactionFinalize, OutcomeFailure)
\t\tvar idempotencyErr error
\t\tif idempotent {
\t\t\tidempotencyErr = runtime.idempotency.Fail(ctx, plan, atomicErr)
\t\t}
\t\truntime.observe(ctx, plan.OperationID, InvocationRoot, PhaseIdempotencyFinalize, outcomeFor(idempotencyErr))
\t\truntime.observe(ctx, plan.OperationID, InvocationRoot, PhaseOutcome, OutcomeFailure)
\t\treturn result, errors.Join(atomicErr, rollbackErr, idempotencyErr)
\t}
\tif commitErr := root.Commit(ctx); commitErr != nil {
''',
)
edit(
    "framework/operation/executor.go",
    '''\tif idempotent {
\t\tif completeErr := runtime.idempotency.Complete(ctx, plan); completeErr != nil {
''',
    '''\tif idempotent && !atomicIdempotency {
\t\tif completeErr := runtime.idempotency.Complete(ctx, plan); completeErr != nil {
''',
)
edit(
    "framework/operation/executor.go",
    '''func outcomeFor(err error) Outcome {
''',
    '''func (runtime *executor) stageAtomicIdempotency(ctx context.Context, plan operationplan.Plan, idempotent bool) (bool, error) {
\tif !idempotent || plan.Execution.Transaction != "local" || runtime.idempotency == nil {
\t\treturn false, nil
\t}
\tcapabilities, ok := runtime.idempotency.(execution.IdempotencyCapabilityReporter)
\tif !ok || !capabilities.SupportsAtomicCompletion() {
\t\treturn false, nil
\t}
\tatomic, ok := runtime.idempotency.(execution.AtomicIdempotencyCoordinator)
\tif !ok {
\t\treturn false, execution.ErrIdempotencyAtomicUnavailable
\t}
\ttransaction, err := execution.TransactionHandleFrom(ctx)
\tif err != nil {
\t\treturn false, errors.Join(execution.ErrIdempotencyAtomicUnavailable, err)
\t}
\tif err := atomic.CompleteInTransaction(ctx, plan, transaction); err != nil {
\t\treturn false, err
\t}
\treturn true, nil
}

func outcomeFor(err error) Outcome {
''',
)

append(
    "pkg/operationplan/plan_test.go",
    r'''
func TestValidateRequiresLocalTransactionForDurableIdempotency(t *testing.T) {
	plan := Plan{
		OperationID: "command", Domain: "d", Application: "app", UseCase: "command",
		RequestType: "d.CommandRequest", ResponseType: "d.CommandResponse",
		Security: Security{PermissionMode: "all"},
		Execution: Execution{Transaction: "none", Idempotency: "required"},
		Bindings: Bindings{RPC: "/d.App/Command"},
	}
	if err := Validate(Set{Operations: []Plan{plan}}); err == nil || !strings.Contains(err.Error(), "requires local transaction") {
		t.Fatalf("durable idempotency transaction err=%v", err)
	}
	plan.Execution.Transaction = "local"
	if err := Validate(Set{Operations: []Plan{plan}}); err != nil {
		t.Fatalf("local durable idempotency should validate: %v", err)
	}
}
''',
)

print("C9.7 durable idempotency hardening staged")
