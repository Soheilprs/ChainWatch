package api

import (
	"errors"
	"testing"

	"github.com/soheilprs/chainwatch/internal/domain"
)

func TestBadInputErrorPreservesSafeMessageAndCategory(t *testing.T) {
	cause := errors.New("strconv details")
	err := NewBadInputError("invalid block", cause)

	if !errors.Is(err, domain.ErrBadInput) {
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
