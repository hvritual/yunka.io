package applicationboundary

import "go/types"

// Public aliases can name unnamed structural values as well as private named
// types. A caller can assert the value, assign it to an addressable local, then
// invoke pointer-receiver methods. Do not turn interfaces or pointers into
// pointer-to-interface/pointer-to-pointer method sets.
func (c *checker) reachableMethodSet(t types.Type) (*types.MethodSet, bool) {
	value := types.NewMethodSet(t)
	concrete := types.Unalias(t)
	if _, pointer := concrete.(*types.Pointer); pointer || types.IsInterface(concrete) {
		return value, false
	}
	nameable := false
	if named, ok := concrete.(*types.Named); ok {
		nameable = named.Obj().Exported()
	}
	// This also preserves the identity of an explicitly instantiated exported
	// generic alias, before unaliasing erases that public spelling.
	if alias, ok := t.(*types.Alias); ok && alias.Obj().Exported() {
		nameable = true
	}
	possibleGenericAlias := false
	for _, p := range c.packages {
		for _, name := range p.Scope().Names() {
			obj, ok := p.Scope().Lookup(name).(*types.TypeName)
			if !ok || !obj.Exported() {
				continue
			}
			if types.Identical(types.Unalias(obj.Type()), concrete) {
				nameable = true
			}
			if alias, ok := obj.Type().(*types.Alias); ok && alias.TypeParams().Len() > 0 && aliasCanName(alias, concrete) {
				possibleGenericAlias = true
			}
		}
	}
	pointer := types.NewMethodSet(types.NewPointer(concrete))
	if nameable {
		return pointer, false
	}
	// We do not solve arbitrary generic-alias instantiations. A possible public
	// spelling with additional pointer authority makes proof incomplete rather
	// than certifying the private spelling as an encapsulation boundary.
	if possibleGenericAlias {
		for i := 0; i < pointer.Len(); i++ {
			m := pointer.At(i).Obj()
			if m.Exported() && value.Lookup(m.Pkg(), m.Name()) == nil {
				return value, true
			}
		}
	}
	return value, false
}
