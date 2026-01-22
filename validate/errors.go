package validate

import "strings"

// ErrorCollector provides common error formatting for validation errors.
type ErrorCollector map[string]string

func (e ErrorCollector) Error() string {
	var b strings.Builder
	b.WriteString("validation failed:")
	for k, msg := range e {
		b.WriteString("\n - ")
		b.WriteString(k)
		b.WriteString(": ")
		b.WriteString(msg)
	}
	return b.String()
}

// ValidationErrors aggregates rule errors.
type ValidationErrors struct {
	Errors map[string]error
}

func (e ValidationErrors) Error() string {
	collector := make(ErrorCollector, len(e.Errors))
	for k, err := range e.Errors {
		collector[k] = err.Error()
	}
	return collector.Error()
}

// FieldErrors maps struct field paths to messages.
type FieldErrors ErrorCollector

func (f FieldErrors) Error() string {
	return ErrorCollector(f).Error()
}
