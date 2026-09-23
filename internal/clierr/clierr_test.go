package clierr_test

import (
	"errors"
	"testing"

	"github.com/thegapcloser/kanban-md/internal/clierr"
)

func TestErrorImplementsError(t *testing.T) {
	var err error = clierr.New(clierr.TaskNotFound, "task not found: #42")
	if err.Error() != "task not found: #42" {
		t.Errorf("Error() = %q, want %q", err.Error(), "task not found: #42")
	}
}

func TestErrorsAs(t *testing.T) {
	err := clierr.New(clierr.InvalidStatus, "bad status")
	var wrapped error = err

	var target *clierr.Error
	if !errors.As(wrapped, &target) {
		t.Fatal("errors.As failed to unwrap *clierr.Error")
	}
	if target.Code != clierr.InvalidStatus {
		t.Errorf("Code = %q, want %q", target.Code, clierr.InvalidStatus)
	}
}

func TestExitCode(t *testing.T) {
	tests := [2]struct {
		code string
		want int
	}{
		{clierr.TaskNotFound, 1},
		{clierr.InternalError, 2},
	}
	for _, tt := range tests {
		err := clierr.New(tt.code, "msg")
		if got := err.ExitCode(); got != tt.want {
			t.Errorf("ExitCode(%s) = %d, want %d", tt.code, got, tt.want)
		}
	}
}

func TestNewf(t *testing.T) {
	err := clierr.Newf(clierr.InvalidTaskID, "invalid ID %q", "abc")
	if err.Message != `invalid ID "abc"` {
		t.Errorf("Message = %q, want %q", err.Message, `invalid ID "abc"`)
	}
}

func TestWithDetails(t *testing.T) {
	err := clierr.New(clierr.TaskNotFound, "not found").
		WithDetails(map[string]any{"id": 42})
	if err.Details == nil {
		t.Fatal("Details is nil after WithDetails")
	}
	if err.Details["id"] != 42 {
		t.Errorf("Details[id] = %v, want 42", err.Details["id"])
	}
}

func TestSilentError(t *testing.T) {
	err := &clierr.SilentError{Code: 1}
	if err.Error() != "exit 1" {
		t.Errorf("Error() = %q, want %q", err.Error(), "exit 1")
	}

	var target *clierr.SilentError
	if !errors.As(err, &target) {
		t.Fatal("errors.As failed to unwrap *SilentError")
	}
}
