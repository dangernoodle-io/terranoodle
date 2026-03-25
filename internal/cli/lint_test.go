package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLint_FormatFlag(t *testing.T) {
	// Verify that the --format flag is registered on the lint command.
	formatFlag := lintCmd.Flag("format")
	assert.NotNil(t, formatFlag)
	assert.Equal(t, "format", formatFlag.Name)
	assert.Equal(t, "text", formatFlag.DefValue)
}
