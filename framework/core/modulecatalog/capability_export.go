package modulecatalog

// Export binds a runtime value to this typed capability key.
//
// A receiver method is intentional here: T is fixed by CapabilityKey[T]
// before the value argument is checked, so a concrete implementation can be
// passed directly when T is an interface contract. This keeps export-time
// assignability checked by Go without an untyped registry or runtime cast.
func (key CapabilityKey[T]) Export(value T) CapabilityExport {
	return CapabilityExport{contract: key.contract, value: value}
}
