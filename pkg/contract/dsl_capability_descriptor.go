package contract

import "fmt"

// applyDSLCapabilityDeclarations is a narrow additive parser for the
// ApplicationDeclaration.capabilities field. It deliberately leaves the
// established DSL descriptor parser unchanged while the capability field is
// projected into the canonical Manifest.
func applyDSLCapabilityDeclarations(manifest *Manifest, data []byte) error {
	capabilities, err := parseDSLCapabilityDeclarations(data)
	if err != nil {
		return err
	}
	for index := range manifest.Services {
		service := &manifest.Services[index]
		if service.Application == nil {
			continue
		}
		values, ok := capabilities[service.FullName]
		if !ok {
			continue
		}
		service.Application.Capabilities = append([]CapabilityRequirement(nil), values...)
	}
	return nil
}

func parseDSLCapabilityDeclarations(data []byte) (map[string][]CapabilityRequirement, error) {
	result := map[string][]CapabilityRequirement{}
	err := scanWire(data, func(field wireField) error {
		if field.Number != 1 || field.Type != 2 {
			return nil
		}
		return parseDSLCapabilityFile(field.Bytes, result)
	})
	return result, err
}

func parseDSLCapabilityFile(data []byte, out map[string][]CapabilityRequirement) error {
	var pkg string
	var services [][]byte
	if err := scanWire(data, func(field wireField) error {
		switch field.Number {
		case 2:
			pkg = string(field.Bytes)
		case 6:
			services = append(services, append([]byte(nil), field.Bytes...))
		}
		return nil
	}); err != nil {
		return err
	}
	for _, raw := range services {
		if err := parseDSLCapabilityService(raw, pkg, out); err != nil {
			return err
		}
	}
	return nil
}

func parseDSLCapabilityService(data []byte, pkg string, out map[string][]CapabilityRequirement) error {
	var name string
	var options []byte
	if err := scanWire(data, func(field wireField) error {
		switch field.Number {
		case 1:
			name = string(field.Bytes)
		case 3:
			options = append([]byte(nil), field.Bytes...)
		}
		return nil
	}); err != nil {
		return err
	}
	application := optionPayload(options, dslServiceApplicationOpt)
	if len(application) == 0 {
		return nil
	}
	var values []CapabilityRequirement
	if err := scanWire(application, func(field wireField) error {
		if field.Number != 4 || field.Type != 2 {
			return nil
		}
		value, err := parseDSLCapabilityRequirement(field.Bytes)
		if err != nil {
			return err
		}
		values = append(values, value)
		return nil
	}); err != nil {
		return fmt.Errorf("contract: %s capability option: %w", fullName(pkg, "", name), err)
	}
	if len(values) > 0 {
		out[fullName(pkg, "", name)] = normalizeCapabilityRequirements(values)
	}
	return nil
}

func parseDSLCapabilityRequirement(data []byte) (CapabilityRequirement, error) {
	var result CapabilityRequirement
	if err := scanWire(data, func(field wireField) error {
		switch field.Number {
		case 1:
			result.Name = string(field.Bytes)
		case 2:
			result.Package = string(field.Bytes)
		case 3:
			result.Type = string(field.Bytes)
		}
		return nil
	}); err != nil {
		return CapabilityRequirement{}, err
	}
	return normalizeCapabilityRequirement(result), nil
}
