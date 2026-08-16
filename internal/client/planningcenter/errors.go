package planningcenter

import (
	"errors"
	"fmt"
)

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

// ErrPaginationLimitExceeded is returned by GetCheckoutsForEvent when the
// Planning Center API does not terminate pagination within
// maxCheckoutPagesPerEvent pages. It is a distinguishable sentinel so callers
// can treat truncation as a failure (e.g. not advancing the event window)
// rather than silently losing checkouts.
var ErrPaginationLimitExceeded = errors.New("pagination limit exceeded")
