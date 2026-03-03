// Package types provides core type definitions for the configuration system.
// All types are designed for zero-allocation reads and thread-safe operations.
package types

import (
	"context"
	"strconv"
	"time"
)

// Value represents a configuration value with type information.
// It is immutable after creation and safe for concurrent access.
type Value struct {
	raw      any
	str      string
	typ      ValueType
	source   SourceType
	priority int
}

// ValueType represents the type of a configuration value.
type ValueType uint8

const (
	TypeUnknown ValueType = iota
	TypeString
	TypeInt
	TypeInt64
	TypeFloat64
	TypeBool
	TypeDuration
	TypeTime
	TypeSlice
	TypeMap
)

// SourceType identifies where a configuration value originated.
type SourceType uint8

const (
	SourceNone SourceType = iota
	SourceFile
	SourceEnv
	SourceMemory
	SourceRemote
	SourceDefault
)

func (v Value) Any() any {
	return v.raw
}

// String returns the string representation of SourceType.
func (s SourceType) String() string {
	switch s {
	case SourceFile:
		return "file"
	case SourceEnv:
		return "env"
	case SourceMemory:
		return "memory"
	case SourceRemote:
		return "remote"
	case SourceDefault:
		return "default"
	default:
		return "unknown"
	}
}

// String returns the string representation of ValueType.
func (t ValueType) String() string {
	switch t {
	case TypeString:
		return "string"
	case TypeInt:
		return "int"
	case TypeInt64:
		return "int64"
	case TypeFloat64:
		return "float64"
	case TypeBool:
		return "bool"
	case TypeDuration:
		return "duration"
	case TypeTime:
		return "time"
	case TypeSlice:
		return "slice"
	case TypeMap:
		return "map"
	default:
		return "unknown"
	}
}

// NewValue creates a new immutable Value.
func NewValue(raw any, typ ValueType, source SourceType, priority int) Value {
	return Value{
		raw:      raw,
		str:      formatValue(raw),
		typ:      typ,
		source:   source,
		priority: priority,
	}
}

// Raw returns the raw underlying value.
func (v Value) Raw() any { return v.raw }

// String returns the string representation.
func (v Value) String() string { return v.str }

// Type returns the value type.
func (v Value) Type() ValueType { return v.typ }

// Source returns the source type.
func (v Value) Source() SourceType { return v.source }

// Priority returns the priority level.
func (v Value) Priority() int { return v.priority }

// Int returns the value as int.
func (v Value) Int() (int, bool) {
	if v.typ == TypeInt {
		return v.raw.(int), true
	}

	return 0, false
}

// Int64 returns the value as int64.
func (v Value) Int64() (int64, bool) {
	if v.typ == TypeInt64 {
		return v.raw.(int64), true
	}

	return 0, false
}

// Float64 returns the value as float64.
func (v Value) Float64() (float64, bool) {
	if v.typ == TypeFloat64 {
		return v.raw.(float64), true
	}

	return 0, false
}

// Bool returns the value as bool.
func (v Value) Bool() (bool, bool) {
	if v.typ == TypeBool {
		return v.raw.(bool), true
	}

	return false, false
}

// Duration returns the value as time.Duration.
func (v Value) Duration() (time.Duration, bool) {
	if v.typ == TypeDuration {
		return v.raw.(time.Duration), true
	}

	return 0, false
}

// formatValue converts a value to its string representation.
func formatValue(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case int:
		return intToStr(val)
	case int64:
		return int64ToStr(val)
	case float64:
		return float64ToStr(val)
	case bool:
		if val {
			return "true"
		}

		return "false"
	case time.Duration:
		return val.String()
	case time.Time:
		return val.Format(time.RFC3339)
	default:
		return ""
	}
}

// Helper functions to avoid fmt package overhead.
func intToStr(v int) string {
	return strconv.Itoa(v)
}

func int64ToStr(v int64) string {
	return strconv.FormatInt(v, 10)
}

func float64ToStr(v float64) string {
	// Simple implementation for common cases
	if v == 0 {
		return "0"
	}

	return string(appendFloat(v))
}

func appendFloat(v float64) []byte {
	// Handle integer part
	intPart := int64(v)
	fracPart := v - float64(intPart)
	if fracPart < 0 {
		fracPart = -fracPart
	}

	result := int64ToStr(intPart)
	if fracPart > 0.0000001 {
		result += "."
		// Get 6 decimal places
		fracPart *= 1000000
		fracInt := int64(fracPart + 0.5)
		fracStr := int64ToStr(fracInt)
		// Pad with zeros if needed
		for len(fracStr) < 6 {
			fracStr = "0" + fracStr
		}
		// Trim trailing zeros
		for fracStr != "" && fracStr[len(fracStr)-1] == '0' {
			fracStr = fracStr[:len(fracStr)-1]
		}
		result += fracStr
	}

	return []byte(result)
}

// Event represents a configuration change event.
type Event struct {
	Type      EventType
	Key       string
	OldValue  Value
	NewValue  Value
	Timestamp time.Time
	Source    SourceType
}

// EventType represents the type of configuration event.
type EventType uint8

const (
	EventCreate EventType = iota
	EventUpdate
	EventDelete
	EventReload
)

// String returns the string representation of EventType.
func (e EventType) String() string {
	switch e {
	case EventCreate:
		return "create"
	case EventUpdate:
		return "update"
	case EventDelete:
		return "delete"
	case EventReload:
		return "reload"
	default:
		return "unknown"
	}
}

// Observer is a function that receives configuration events.
type Observer func(ctx context.Context, event Event) error

// HookType represents lifecycle hook types.
type HookType uint8

const (
	HookBeforeLoad HookType = iota
	HookAfterLoad
	HookBeforeReload
	HookAfterReload
	HookBeforeSet
	HookAfterSet
	HookBeforeDelete
	HookAfterDelete
)

// Hook is a lifecycle hook function.
type Hook func(ctx context.Context) error

// Error represents a configuration error with context.
type Error struct {
	Code    ErrorCode
	Message string
	Key     string
	Source  SourceType
	Cause   error
}

// ErrorCode represents error types.
type ErrorCode uint16

const (
	ErrUnknown ErrorCode = iota
	ErrNotFound
	ErrTypeMismatch
	ErrInvalidFormat
	ErrParseError
	ErrValidationError
	ErrSourceError
	ErrCryptoError
	ErrWatchError
	ErrBindError
	ErrContextCanceled
)

// Error implements the error interface.
func (e *Error) Error() string {
	msg := e.Message
	if e.Key != "" {
		msg += " (key: " + e.Key + ")"
	}
	if e.Cause != nil {
		msg += ": " + e.Cause.Error()
	}

	return msg
}

// Unwrap returns the underlying cause.
func (e *Error) Unwrap() error {
	return e.Cause
}

// NewError creates a new configuration error.
func NewError(code ErrorCode, message string, opts ...func(*Error)) *Error {
	e := &Error{
		Code:    code,
		Message: message,
	}
	for _, opt := range opts {
		opt(e)
	}

	return e
}

// WithKey sets the key for the error.
func WithKey(key string) func(*Error) {
	return func(e *Error) { e.Key = key }
}

// WithSource sets the source for the error.
func WithSource(source SourceType) func(*Error) {
	return func(e *Error) { e.Source = source }
}

// WithCause sets the cause for the error.
func WithCause(cause error) func(*Error) {
	return func(e *Error) { e.Cause = cause }
}

// ContextKey type for context values.
type ContextKey string

const (
	// ContextKeyTraceID is used for distributed tracing.
	ContextKeyTraceID ContextKey = "trace_id"
	// ContextKeySource indicates the current source being processed.
	ContextKeySource ContextKey = "source"
)
