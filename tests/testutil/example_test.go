package testutil

import (
	"testing"
)

// TestExample demonstrates the basic test structure
func TestExample(t *testing.T) {
	t.Run("simple assertion", func(t *testing.T) {
		got := 1 + 1
		want := 2
		AssertEqual(t, got, want)
	})

	t.Run("with test suite", func(t *testing.T) {
		suite := NewTestSuite(t)
		suite.RunTest("example test", func() {
			AssertTrue(t, true, "this should pass")
		})
	})
}

// BenchmarkExample demonstrates benchmark testing
func BenchmarkExample(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = i * 2
	}
}
