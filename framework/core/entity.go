package core

type PersistObject struct {
	conds map[string]any `json:"-"`
}

func (object *PersistObject) Conds() map[string]any {
	if object == nil || object.conds == nil {
		return map[string]any{}
	}
	result := make(map[string]any, len(object.conds))
	for key, value := range object.conds {
		result[key] = value
	}
	object.conds = nil
	return result
}

func (object *PersistObject) ConfField(name string, value any) {
	if object.conds == nil {
		object.conds = make(map[string]any)
	}
	object.conds[name] = value
}
