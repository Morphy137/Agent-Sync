package apperr_test

import (
	"errors"
	"testing"

	"agentsync/internal/apperr"
)

func TestAppError(t *testing.T) {
	err := apperr.New(apperr.RootNotFound)
	if err.Code != apperr.RootNotFound {
		t.Fatalf("expected code %s, got %s", apperr.RootNotFound, err.Code)
	}

	wrapped := apperr.Wrap(apperr.WriteFailed, errors.New("disk full"))
	if wrapped.Code != apperr.WriteFailed {
		t.Fatalf("expected code %s, got %s", apperr.WriteFailed, wrapped.Code)
	}
	if !errors.Is(wrapped, wrapped.Cause) {
		t.Fatalf("expected wrapped error to unwrap to cause")
	}

	withParam := apperr.New(apperr.TargetNotFound).WithParam("path", "/test/path")
	if withParam.Params["path"] != "/test/path" {
		t.Fatalf("expected param path to be set")
	}
}
