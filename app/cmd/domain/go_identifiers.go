package domain

func objectGoName(object ObjectSpec) string {
	if object.GoName != "" {
		return object.GoName
	}
	return exportedIdentifier(object.Name)
}

func fieldGoName(field Field) string {
	if field.GoName != "" {
		return field.GoName
	}
	return exportedIdentifier(field.Name)
}
