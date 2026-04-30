package custom_error

import (
	"fmt"
	"net/http"
)

type AppError struct {
	Code       string
	Msg        string
	HttpStatus int
	Cause      error
}

func (e *AppError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Msg)
}

func (e *AppError) Unwrap() error {
	return e.Cause
}

func New(code, msg string, status int, cause error) *AppError {
	return &AppError{
		Code:       code,
		Msg:        msg,
		HttpStatus: status,
		Cause:      cause,
	}
}

// Default internal error fallback
func Internal(msg string, cause error) *AppError {
	return New("INTERNAL_ERROR", msg, http.StatusInternalServerError, cause)
}
