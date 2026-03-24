package toposort

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSort_EmptyGraph tests that an empty graph returns an empty result with no error.
func TestSort_EmptyGraph(t *testing.T) {
	adjacency := map[string][]string{}
	result, err := Sort(adjacency)
	require.NoError(t, err)
	assert.Empty(t, result)
}

// TestSort_SingleNode tests a single node with no dependencies.
func TestSort_SingleNode(t *testing.T) {
	adjacency := map[string][]string{
		"A": nil,
	}
	result, err := Sort(adjacency)
	require.NoError(t, err)
	assert.Equal(t, []string{"A"}, result)
}

// TestSort_LinearChain tests a linear dependency chain A->B->C.
// Result should be C, B, A (dependencies first).
func TestSort_LinearChain(t *testing.T) {
	adjacency := map[string][]string{
		"A": {"B"},
		"B": {"C"},
		"C": nil,
	}
	result, err := Sort(adjacency)
	require.NoError(t, err)
	// C should come first (no dependencies), then B, then A.
	assert.Equal(t, []string{"C", "B", "A"}, result)
}

// TestSort_Diamond tests a diamond: A depends on B and C, B and C depend on D.
// Result should be D first, then B and C in any order, then A.
func TestSort_Diamond(t *testing.T) {
	adjacency := map[string][]string{
		"A": {"B", "C"},
		"B": {"D"},
		"C": {"D"},
		"D": nil,
	}
	result, err := Sort(adjacency)
	require.NoError(t, err)
	require.Len(t, result, 4)
	// D must come first.
	assert.Equal(t, "D", result[0])
	// A must come last.
	assert.Equal(t, "A", result[3])
	// B and C should be in the middle.
	assert.Contains(t, result, "B")
	assert.Contains(t, result, "C")
}

// TestSort_CycleDetection tests that a simple cycle A->B->A is detected.
func TestSort_CycleDetection(t *testing.T) {
	adjacency := map[string][]string{
		"A": {"B"},
		"B": {"A"},
	}
	result, err := Sort(adjacency)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "cycle")
}

// TestSort_SelfReferencing tests that a self-referencing node is detected as a cycle.
func TestSort_SelfReferencing(t *testing.T) {
	adjacency := map[string][]string{
		"A": {"A"},
	}
	result, err := Sort(adjacency)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "cycle")
}

// TestSort_ComplexDAG tests a more complex DAG.
func TestSort_ComplexDAG(t *testing.T) {
	adjacency := map[string][]string{
		"A": {"B", "C"},
		"B": {"D"},
		"C": {"D"},
		"D": {"E"},
		"E": nil,
	}
	result, err := Sort(adjacency)
	require.NoError(t, err)
	require.Len(t, result, 5)
	// E must be first, A must be last.
	assert.Equal(t, "E", result[0])
	assert.Equal(t, "A", result[4])
}
