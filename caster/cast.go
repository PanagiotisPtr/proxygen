package caster

func Cast[T any](val interface{}) T {
	var defaultVal T
	if v, ok := val.(T); ok {
		return v
	}

	return defaultVal
}
