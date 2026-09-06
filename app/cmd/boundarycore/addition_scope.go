package boundarycore

import "github.com/hvritual/yunka.io/pkg/contract"

// The single-addition policy must not bless a simultaneous edit to an existing
// Operation, DTO, Application or dependency. New reachable types/imports may be
// present, but they do not automatically establish a comparable client contract.
func preservesBaseline(before, after contract.Manifest, candidate OperationEvidence) bool {
	rest := detachedManifest(after)
	for i := range rest.Services {
		s := &rest.Services[i]
		methods := []contract.Method(nil)
		for _, method := range s.Methods {
			if method.Operation == nil || method.Operation.ID != candidate.Plan.OperationID {
				methods = append(methods, method)
			}
		}
		s.Methods = methods
		if s.Application != nil {
			ops := []contract.OperationDeclaration(nil)
			for _, op := range s.Application.Operations {
				if op.ID != candidate.Plan.OperationID {
					ops = append(ops, op)
				}
			}
			s.Application.Operations = ops
		}
	}
	var ok bool
	rest.Messages, ok = retainExisting(before.Messages, rest.Messages, func(m contract.Message) string { return m.FullName }, candidate.Sources.MessageTypes)
	if !ok {
		return false
	}
	rest.Enums, ok = retainExisting(before.Enums, rest.Enums, func(e contract.Enum) string { return e.FullName }, candidate.Sources.EnumTypes)
	if !ok {
		return false
	}
	oldFiles := map[string]contract.File{}
	for _, f := range before.Files {
		oldFiles[f.Name] = f
	}
	files := []contract.File(nil)
	for _, f := range rest.Files {
		old, exists := oldFiles[f.Name]
		if !exists {
			if !containsString(candidate.Sources.SourceFiles, f.Name) {
				return false
			}
			continue
		}
		// New local imports are allowed only for newly introduced reachable files.
		// Existing imported files, external includes and removed imports cannot
		// silently change the original compilation environment under this policy.
		deps := []string(nil)
		for _, dep := range f.Dependencies {
			if containsString(old.Dependencies, dep) {
				deps = append(deps, dep)
				continue
			}
			if _, existed := oldFiles[dep]; existed || !containsString(candidate.Sources.SourceFiles, dep) {
				return false
			}
		}
		f.Dependencies = deps
		files = append(files, f)
	}
	rest.Files = files
	rest.Normalize()
	return jsonEqual(before, rest)
}

func retainExisting[T any](before, after []T, key func(T) string, reachable []string) ([]T, bool) {
	old := map[string]bool{}
	for _, item := range before {
		old[key(item)] = true
	}
	kept := []T(nil)
	for _, item := range after {
		if old[key(item)] {
			kept = append(kept, item)
		} else if !containsString(reachable, key(item)) {
			return nil, false
		}
	}
	return kept, true
}
