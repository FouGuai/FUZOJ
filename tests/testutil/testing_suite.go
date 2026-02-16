package testutil

import (
	"testing"
)

// TestSuite provides common setup and teardown functionality for tests
type TestSuite struct {
	t *testing.T
}

// NewTestSuite creates a new test suite
func NewTestSuite(t *testing.T) *TestSuite {
	return &TestSuite{t: t}
}

// Setup runs before each test
func (s *TestSuite) Setup() {
	s.t.Helper()
	// Add common setup logic here
	// e.g., database connections, mock servers, etc.
}

// Teardown runs after each test
func (s *TestSuite) Teardown() {
	s.t.Helper()
	// Add common cleanup logic here
	// e.g., close connections, clean up resources, etc.
}

// RunTest wraps a test function with setup and teardown
func (s *TestSuite) RunTest(name string, testFunc func()) {
	s.t.Run(name, func(t *testing.T) {
		s.Setup()
		defer s.Teardown()
		testFunc()
	})
}
