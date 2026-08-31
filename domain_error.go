package main

import (
	"errors"
	"fmt"
	"net/http"
)

var (
	ErrConfiguration       = errors.New("configuration error")
	ErrRPC                 = errors.New("RPC error")
	ErrDatabase            = errors.New("database error")
	ErrValidation          = errors.New("validation error")
	ErrIndexing            = errors.New("indexing error")
	ErrMetadata            = errors.New("metadata error")
	ErrNotFound            = errors.New("not found")
	ErrBadInput            = errors.New("bad input")
	ErrTemporaryDependency = errors.New("temporary dependency failure")
)

// DomainError adds a stable error category without discarding the original
// cause. Both the category and cause remain discoverable with errors.Is.
type DomainError struct {
	Category  error
	Operation string
	Cause     error
}

func (e *DomainError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Cause == nil {
		return e.Operation
	}
	return fmt.Sprintf("%s: %v", e.Operation, e.Cause)
}

func (e *DomainError) Unwrap() []error {
	if e == nil {
		return nil
	}

	errors := make([]error, 0, 2)
	if e.Category != nil {
		errors = append(errors, e.Category)
	}
	if e.Cause != nil {
		errors = append(errors, e.Cause)
	}
	return errors
}

func NewDomainError(category error, operation string, cause error) error {
	return &DomainError{
		Category:  category,
		Operation: operation,
		Cause:     cause,
	}
}

func classifyDomainError(target *error, category error, operation string) {
	if target == nil || *target == nil || errors.Is(*target, category) {
		return
	}
	*target = NewDomainError(category, operation, *target)
}

// PublicError is an error whose status and message are safe to expose through
// the HTTP API. Its cause remains available to logs and errors.Is/errors.As.
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
		Cause: NewDomainError(
			ErrBadInput,
			"validate API input",
			cause,
		),
	}
}
