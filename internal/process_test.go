package internal

import (
	"errors"
	"os"
	"testing"
)

func TestChildHelper(t *testing.T) {
	if os.Getenv("Q_SECRETS_TEST_HELPER") != "yes" {
		return
	}
	if os.Getenv("Q_SECRET_KEY") != "" || os.Getenv("INJECTED") != "value" || os.Getenv("PARENT_ONLY") != "" {
		os.Exit(90)
	}
	os.Exit(37)
}

func TestRunChildPropagatesExitAndFiltersEnvironment(t *testing.T) {
	t.Setenv("Q_SECRET_KEY", "must-not-inherit")
	t.Setenv("PARENT_ONLY", "must-not-inherit")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	child, err := NewChildCommand(map[string]string{"Q_SECRETS_TEST_HELPER": "yes", "INJECTED": "value"}, exe, []string{"-test.run=^TestChildHelper$"}, true)
	if err != nil {
		t.Fatal(err)
	}
	err = RunChild(child)
	var status *ExitStatus
	if !errors.As(err, &status) || status.Code != 37 {
		t.Fatalf("expected exit 37: %v", err)
	}
}
