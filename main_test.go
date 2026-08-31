package main

import (
	"errors"
	"os"
	"os/exec"
	"testing"
)

func TestMainExitsNonZeroForInvalidConfiguration(t *testing.T) {
	if os.Getenv("CHAINWATCH_TEST_INVALID_CONFIG") == "1" {
		main()
		return
	}

	command := exec.Command(os.Args[0], "-test.run=TestMainExitsNonZeroForInvalidConfiguration")
	command.Env = []string{"CHAINWATCH_TEST_INVALID_CONFIG=1"}
	err := command.Run()

	var exitError *exec.ExitError
	if !errors.As(err, &exitError) {
		t.Fatalf("main error = %v, want process exit error", err)
	}
	if exitError.ExitCode() != 1 {
		t.Fatalf("main exit code = %d, want 1", exitError.ExitCode())
	}
}
