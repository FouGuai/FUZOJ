package tests

import (
	"encoding/json"
	"testing"
)

// AssertEqual checks if two values are equal
func AssertEqual(t *testing.T, got, want interface{}) {
	t.Helper()
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

// AssertNotNil checks if a value is not nil
func AssertNotNil(t *testing.T, value interface{}) {
	t.Helper()
	if value == nil {
		t.Error("expected non-nil value, got nil")
	}
}

// AssertNil checks if a value is nil
func AssertNil(t *testing.T, value interface{}) {
	t.Helper()
	if value != nil {
		t.Errorf("expected nil, got %v", value)
	}
}

// AssertTrue checks if a condition is true
func AssertTrue(t *testing.T, condition bool, message string) {
	t.Helper()
	if !condition {
		t.Errorf("assertion failed: %s", message)
	}
}

// AssertFalse checks if a condition is false
func AssertFalse(t *testing.T, condition bool, message string) {
	t.Helper()
	if condition {
		t.Errorf("assertion failed: %s", message)
	}
}

// MustMarshalJSON marshals an object to JSON or fails the test
func MustMarshalJSON(t *testing.T, v interface{}) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("failed to marshal JSON: %v", err)
	}
	return data
}

// MustUnmarshalJSON unmarshals JSON data or fails the test
func MustUnmarshalJSON(t *testing.T, data []byte, v interface{}) {
	t.Helper()
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}
}
