package zhexrt

// Unit is Zhex Unit: exactly one value.
type Unit struct{}

// Result is the runtime repr for every Zhex Result<T, E>, whatever the
// error family — like the interpreter's inputResult. Err is nil on
// success.
type Result[T any] struct {
	Value T
	Err   *Error
}

func Ok[T any](value T) Result[T] { return Result[T]{Value: value} }

// WithOperation records the builtin that produced a failure, preserving
// the original operation when an error has already been propagated.
func WithOperation[T any](result Result[T], operation string) Result[T] {
	if result.Err != nil && result.Err.Operation == "" {
		result.Err.Operation = operation
	}
	return result
}

// Zero is the zero of T, for pairing an early error return with the
// fallible-word signature shape.
func Zero[T any]() T { var z T; return z }

// ErrAt builds a failed Result with a positioned diagnostic.
func ErrAt[T any](pos Pos, code, message string, details map[string]string) Result[T] {
	return Result[T]{Err: &Error{Pos: pos, Code: code, Message: message, Details: details}}
}
