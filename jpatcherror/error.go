package jpatcherror

// Error codes
const (
	ErrorInvalidOperation = "InvalidOperation"
	ErrorInvalidPath      = "InvalidPath"
	ErrorInvalidSegment   = "InvalidSegment"
	ErrorInvalidValue     = "InvalidValue"
)

// Error staisfies the built in error interface in addition to providing more detailed information about what went wrong
type Error interface {
	error

	Message() string

	Code() string

	Origin() interface{}

	Details() string
}

func New(message, code, details string, origin interface{}) Error {
	return baseError{message, code, details, origin}
}
