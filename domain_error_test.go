package main

import (
	"errors"
	"testing"
)

func TestDomainErrorPreservesCategoryAndCause(t *testing.T) {
	cause := errors.New("connection refused")
	err := NewDomainError(ErrRPC, "fetch latest block", cause)

	if !errors.Is(err, ErrRPC) {
		t.Fatal("expected RPC classification")
	}
	if !errors.Is(err, cause) {
		t.Fatal("expected original cause")
	}

	var domainError *DomainError
	if !errors.As(err, &domainError) {
		t.Fatal("expected typed domain error")
	}
	if domainError.Operation != "fetch latest block" {
		t.Fatalf("operation = %q", domainError.Operation)
	}
}

func TestBadInputErrorPreservesSafeMessageAndCategory(t *testing.T) {
	cause := errors.New("strconv details")
	err := NewBadInputError("invalid block", cause)

	if !errors.Is(err, ErrBadInput) {
		t.Fatal("expected bad input classification")
	}
	if !errors.Is(err, cause) {
		t.Fatal("expected original cause")
	}

	var publicError *PublicError
	if !errors.As(err, &publicError) {
		t.Fatal("expected public error")
	}
	if publicError.Message != "invalid block" {
		t.Fatalf("message = %q", publicError.Message)
	}
}
