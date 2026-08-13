package planningcenter

import "fmt"

type ServerError struct {
	statusCode int
	errMsg     string
}

func (e *ServerError) Error() string {
	return fmt.Sprintf("server error (%d): %s", e.statusCode, e.errMsg)
}

type ClientError struct {
	statusCode int
	errMsg     string
}

func (e *ClientError) Error() string {
	return fmt.Sprintf("client error (%d): %s", e.statusCode, e.errMsg)
}

type TimeoutError struct {
	Err error
}

func (e *TimeoutError) Error() string {
	return "timeout: " + e.Err.Error()
}
