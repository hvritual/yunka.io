package contract

const (
	// AssemblyPlanFilename is the canonical C10 derived assembly artifact name.
	// It is intentionally not emitted by RenderArtifacts in C10.1 because that
	// pipeline does not own the qualified static module snapshot required for a
	// complete plan. C10.2 must emit it only after Contract/OperationPlan facts
	// and the resolved module snapshot are joined.
	AssemblyPlanFilename = "assembly-plan.json"

	// AssemblyPlanRelativePath is the consumer-project committed artifact path.
	AssemblyPlanRelativePath = "contracts/generated/" + AssemblyPlanFilename
)
