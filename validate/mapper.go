package validate

// MapErrorsToFields converts config keys to struct field paths.
func MapErrorsToFields(err error, keyMap map[string]string) error {
	ve, ok := err.(ValidationErrors)
	if !ok {
		return err
	}

	fe := make(FieldErrors, len(ve.Errors))
	for key, e := range ve.Errors {
		fieldPath := mapKeyToField(key, keyMap)
		fe[fieldPath] = e.Error()
	}
	return fe
}

func mapKeyToField(key string, keyMap map[string]string) string {
	if field, ok := keyMap[key]; ok {
		return field
	}
	// Return the original key as fallback
	return key
}
