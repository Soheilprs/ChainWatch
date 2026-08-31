package api

import (
	"net/http"

	"github.com/soheilprs/chainwatch/internal/domain"
)

// PublicError carries an HTTP status and message that are safe to expose while
// preserving the internal cause for logging and classification.
type PublicError struct {
	StatusCode int
	Message    string
	Cause      error
}

func (e *PublicError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return e.Message
}

func (e *PublicError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func NewBadInputError(message string, cause error) error {
	return &PublicError{
		StatusCode: http.StatusBadRequest,
		Message:    message,
		Cause: domain.NewDomainError(
			domain.ErrBadInput,
			"validate API input",
			cause,
		),
	}
}
