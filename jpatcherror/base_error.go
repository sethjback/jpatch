package jpatcherror

import "fmt"

type baseError struct {
	message string
	code    string
	details string
	origin  interface{}
}

func (e baseError) Message() string {
	return e.message
}

func (e baseError) Code() string {
	return e.details
}

func (e baseError) Origin() interface{} {
	return e.origin
}

func (e baseError) Details() string {
	return e.details
}

func (e baseError) Error() string {
	msg := e.message

	if e.details != "" {
		msg = fmt.Sprintf("%s (%s)", msg, e.details)
	}

	return msg
}
