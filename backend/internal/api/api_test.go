package api

import "testing"

func TestReturnedHandler(t *testing.T) {
	handler := NewHandler()
	if handler == nil {
		t.Error("Expected handler to be non-nil")
	}

	// Additional tests can be added here to check specific routes or functionality
	t.Log("Handler created successfully")
}
