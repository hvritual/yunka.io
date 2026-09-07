package contract

import (
	"fmt"
	"strings"
)

// ValidateBoundaryIntent validates explicit data only. Nil is legacy UNKNOWN,
// not a successful boundary decision. This does not infer business ownership.
func ValidateBoundaryIntent(intent *BoundaryIntent) error {
	if intent == nil {
		return nil
	}
	if !validPolicyKey(intent.Context) {
		return fmt.Errorf("boundary intent: context must be a stable lowercase key")
	}
	aggregate := strings.TrimSpace(intent.Aggregate)
	reason := strings.TrimSpace(intent.AggregateNotApplicableReason)
	if (aggregate == "") == (reason == "") {
		return fmt.Errorf("boundary intent: exactly one of aggregate or aggregate_not_applicable_reason is required")
	}
	if aggregate != "" && !validPolicyKey(aggregate) {
		return fmt.Errorf("boundary intent: aggregate must be a stable lowercase key")
	}
	if strings.ContainsRune(reason, '\x00') {
		return fmt.Errorf("boundary intent: aggregate explanation contains NUL")
	}
	return nil
}

func parseBoundaryIntent(data []byte) (*BoundaryIntent, error) {
	result := &BoundaryIntent{}
	if err := scanWire(data, func(field wireField) error {
		if field.Type != 2 {
			return fmt.Errorf("boundary intent: field %d must be a string", field.Number)
		}
		switch field.Number {
		case 1:
			result.Context = string(field.Bytes)
		case 2:
			result.Aggregate = string(field.Bytes)
		case 3:
			result.AggregateNotApplicableReason = string(field.Bytes)
		default:
			return fmt.Errorf("boundary intent: unsupported field %d", field.Number)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	if err := ValidateBoundaryIntent(result); err != nil {
		return nil, err
	}
	return result, nil
}

func validateManifestBoundaryIntents(manifest Manifest) error {
	for _, service := range manifest.Services {
		if service.Application != nil {
			for _, op := range service.Application.Operations {
				if err := ValidateBoundaryIntent(op.Boundary); err != nil {
					return fmt.Errorf("%s: %w", op.ID, err)
				}
			}
		}
		for _, method := range service.Methods {
			if method.Operation != nil {
				if err := ValidateBoundaryIntent(method.Operation.Boundary); err != nil {
					return fmt.Errorf("%s: %w", method.FullName, err)
				}
			}
		}
	}
	return nil
}
