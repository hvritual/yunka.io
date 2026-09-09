package applicationboundary

import "go/types"

// aliasCanName rules out impossible public generic spellings without solving
// arbitrary generic instantiations. Structural positions infer repeated type
// arguments; fully inferred arguments are validated by Go's own instantiator.
// Unused or unsupported arguments remain possible, hence INCOMPLETE at the
// capability boundary rather than a guessed public or private representation.
func aliasCanName(alias *types.Alias, concrete types.Type) bool {
	bindings := map[*types.TypeParam]types.Type{}
	budget := 256
	var match func(types.Type, types.Type) bool
	var tuple func(*types.Tuple, *types.Tuple) bool
	tuple = func(a, b *types.Tuple) bool {
		if a.Len() != b.Len() {
			return false
		}
		for i := 0; i < a.Len(); i++ {
			if !match(a.At(i).Type(), b.At(i).Type()) {
				return false
			}
		}
		return true
	}
	match = func(pattern, value types.Type) bool {
		pattern, value = types.Unalias(pattern), types.Unalias(value)
		budget--
		if budget < 0 {
			return true // an unresolved shape must not certify private authority
		}
		if parameter, ok := pattern.(*types.TypeParam); ok {
			if previous, exists := bindings[parameter]; exists {
				return types.Identical(previous, value)
			}
			bindings[parameter] = value
			return true
		}
		if types.Identical(pattern, value) {
			return true
		}
		switch a := pattern.(type) {
		case *types.Named:
			b, ok := value.(*types.Named)
			if !ok || a.Origin() != b.Origin() || a.TypeArgs().Len() != b.TypeArgs().Len() {
				return false
			}
			for i := 0; i < a.TypeArgs().Len(); i++ {
				if !match(a.TypeArgs().At(i), b.TypeArgs().At(i)) {
					return false
				}
			}
			return true
		case *types.Struct:
			b, ok := value.(*types.Struct)
			if !ok || a.NumFields() != b.NumFields() {
				return false
			}
			for i := 0; i < a.NumFields(); i++ {
				left, right := a.Field(i), b.Field(i)
				if left.Id() != right.Id() || left.Embedded() != right.Embedded() || a.Tag(i) != b.Tag(i) || !match(left.Type(), right.Type()) {
					return false
				}
			}
			return true
		case *types.Pointer:
			b, ok := value.(*types.Pointer)
			return ok && match(a.Elem(), b.Elem())
		case *types.Slice:
			b, ok := value.(*types.Slice)
			return ok && match(a.Elem(), b.Elem())
		case *types.Array:
			b, ok := value.(*types.Array)
			return ok && a.Len() == b.Len() && match(a.Elem(), b.Elem())
		case *types.Map:
			b, ok := value.(*types.Map)
			return ok && match(a.Key(), b.Key()) && match(a.Elem(), b.Elem())
		case *types.Chan:
			b, ok := value.(*types.Chan)
			return ok && a.Dir() == b.Dir() && match(a.Elem(), b.Elem())
		case *types.Signature:
			b, ok := value.(*types.Signature)
			return ok && a.Variadic() == b.Variadic() && tuple(a.Params(), b.Params()) && tuple(a.Results(), b.Results())
		case *types.Interface:
			b, ok := value.(*types.Interface)
			if !ok || !a.IsMethodSet() || !b.IsMethodSet() || a.NumMethods() != b.NumMethods() {
				return false
			}
			for i := 0; i < a.NumMethods(); i++ {
				if a.Method(i).Id() != b.Method(i).Id() || !match(a.Method(i).Type(), b.Method(i).Type()) {
					return false
				}
			}
			return true
		default:
			return containsTypeParameter(pattern, map[types.Type]bool{})
		}
	}
	if !match(alias.Rhs(), concrete) {
		return false
	}
	args := make([]types.Type, alias.TypeParams().Len())
	complete := true
	for i := range args {
		parameter := alias.TypeParams().At(i)
		arg, known := bindings[parameter]
		if !known {
			complete = false
			continue
		}
		args[i] = arg
		constraint := parameter.Constraint()
		if !containsTypeParameter(constraint, map[types.Type]bool{}) && !types.Satisfies(arg, constraint.Underlying().(*types.Interface)) {
			return false
		}
	}
	if complete {
		instance, err := types.Instantiate(nil, alias, args, true)
		return err == nil && types.Identical(types.Unalias(instance), concrete)
	}
	return true
}

func containsTypeParameter(t types.Type, seen map[types.Type]bool) bool {
	t = types.Unalias(t)
	if seen[t] {
		return false
	}
	seen[t] = true
	switch x := t.(type) {
	case *types.TypeParam:
		return true
	case *types.Named:
		for i := 0; i < x.TypeArgs().Len(); i++ {
			if containsTypeParameter(x.TypeArgs().At(i), seen) {
				return true
			}
		}
		return containsTypeParameter(x.Underlying(), seen)
	case *types.Pointer:
		return containsTypeParameter(x.Elem(), seen)
	case *types.Slice:
		return containsTypeParameter(x.Elem(), seen)
	case *types.Array:
		return containsTypeParameter(x.Elem(), seen)
	case *types.Map:
		return containsTypeParameter(x.Key(), seen) || containsTypeParameter(x.Elem(), seen)
	case *types.Chan:
		return containsTypeParameter(x.Elem(), seen)
	case *types.Struct:
		for i := 0; i < x.NumFields(); i++ {
			if containsTypeParameter(x.Field(i).Type(), seen) {
				return true
			}
		}
	case *types.Tuple:
		for i := 0; i < x.Len(); i++ {
			if containsTypeParameter(x.At(i).Type(), seen) {
				return true
			}
		}
	case *types.Signature:
		return containsTypeParameter(x.Params(), seen) || containsTypeParameter(x.Results(), seen)
	case *types.Interface:
		for i := 0; i < x.NumEmbeddeds(); i++ {
			if containsTypeParameter(x.EmbeddedType(i), seen) {
				return true
			}
		}
		for i := 0; i < x.NumExplicitMethods(); i++ {
			if containsTypeParameter(x.ExplicitMethod(i).Type(), seen) {
				return true
			}
		}
	case *types.Union:
		for i := 0; i < x.Len(); i++ {
			if containsTypeParameter(x.Term(i).Type(), seen) {
				return true
			}
		}
	}
	return false
}
