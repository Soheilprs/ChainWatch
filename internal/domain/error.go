package domain

import (
	"errors"
	"fmt"
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
	ErrInvalidTokenAddress = errors.New("invalid token address")
	ErrChainReorg          = errors.New("Ethereum chain reorganization detected")
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

// ClassifyError adds a stable category to an error returned through a package
// boundary while preserving its original cause.
func ClassifyError(target *error, category error, operation string) {
	if target == nil || *target == nil || errors.Is(*target, category) {
		return
	}
	*target = NewDomainError(category, operation, *target)
}
