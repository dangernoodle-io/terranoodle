package testutil

import (
	"os"
	"testing"
)

// SkipUnlessAcc skips the test unless ACC_TERRANOODLE is set.
func SkipUnlessAcc(t *testing.T) {
	t.Helper()
	if os.Getenv("ACC_TERRANOODLE") == "" {
		t.Skip("set ACC_TERRANOODLE=1 to run integration tests")
	}
}
