package rename

import (
	"bytes"
	"strings"
	"testing"

	tfjson "github.com/hashicorp/terraform-json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeCandidate(destroyAddr string, createAddrs ...string) Candidate {
	creates := make([]*tfjson.ResourceChange, len(createAddrs))
	for i, addr := range createAddrs {
		creates[i] = &tfjson.ResourceChange{Address: addr, Type: "aws_s3_bucket"}
	}
	return Candidate{
		Destroy: &tfjson.ResourceChange{Address: destroyAddr, Type: "aws_s3_bucket"},
		Creates: creates,
	}
}

func TestConfirmCandidates_SingleCreate_Yes(t *testing.T) {
	candidates := []Candidate{makeCandidate("aws_s3_bucket.old", "aws_s3_bucket.new")}
	input := strings.NewReader("y\n")
	var output bytes.Buffer

	pairs, err := ConfirmCandidates(input, &output, candidates)
	require.NoError(t, err)
	require.Len(t, pairs, 1)
	assert.Equal(t, "aws_s3_bucket.old", pairs[0].From)
	assert.Equal(t, "aws_s3_bucket.new", pairs[0].To)
}

func TestConfirmCandidates_SingleCreate_No(t *testing.T) {
	candidates := []Candidate{makeCandidate("aws_s3_bucket.old", "aws_s3_bucket.new")}
	input := strings.NewReader("n\n")
	var output bytes.Buffer

	pairs, err := ConfirmCandidates(input, &output, candidates)
	require.NoError(t, err)
	assert.Empty(t, pairs)
}

func TestConfirmCandidates_MultipleCreates_SelectSecond(t *testing.T) {
	candidates := []Candidate{makeCandidate("aws_s3_bucket.old", "aws_s3_bucket.new_a", "aws_s3_bucket.new_b")}
	input := strings.NewReader("2\n")
	var output bytes.Buffer

	pairs, err := ConfirmCandidates(input, &output, candidates)
	require.NoError(t, err)
	require.Len(t, pairs, 1)
	assert.Equal(t, "aws_s3_bucket.old", pairs[0].From)
	assert.Equal(t, "aws_s3_bucket.new_b", pairs[0].To)
}

func TestConfirmCandidates_MultipleCreates_Skip(t *testing.T) {
	candidates := []Candidate{makeCandidate("aws_s3_bucket.old", "aws_s3_bucket.new_a", "aws_s3_bucket.new_b")}
	input := strings.NewReader("s\n")
	var output bytes.Buffer

	pairs, err := ConfirmCandidates(input, &output, candidates)
	require.NoError(t, err)
	assert.Empty(t, pairs)
}

func TestConfirmCandidates_MultipleCreates_InvalidChoice(t *testing.T) {
	candidates := []Candidate{makeCandidate("aws_s3_bucket.old", "aws_s3_bucket.new_a", "aws_s3_bucket.new_b")}
	input := strings.NewReader("9\n")
	var output bytes.Buffer

	pairs, err := ConfirmCandidates(input, &output, candidates)
	require.NoError(t, err)
	assert.Empty(t, pairs)
	assert.Contains(t, output.String(), "skipping")
}

func TestConfirmCandidates_EOF(t *testing.T) {
	candidates := []Candidate{makeCandidate("aws_s3_bucket.old", "aws_s3_bucket.new")}
	input := strings.NewReader("")
	var output bytes.Buffer

	pairs, err := ConfirmCandidates(input, &output, candidates)
	require.NoError(t, err)
	assert.Empty(t, pairs)
}

func TestConfirmCandidates_ConsumedCreatesRemoved(t *testing.T) {
	// Two destroys share the same create pool
	candidates := []Candidate{
		makeCandidate("aws_s3_bucket.old_a", "aws_s3_bucket.new_x"),
		makeCandidate("aws_s3_bucket.old_b", "aws_s3_bucket.new_x"),
	}
	input := strings.NewReader("y\n")
	var output bytes.Buffer

	pairs, err := ConfirmCandidates(input, &output, candidates)
	require.NoError(t, err)
	// First candidate claims new_x, second candidate has no available creates
	require.Len(t, pairs, 1)
	assert.Equal(t, "aws_s3_bucket.old_a", pairs[0].From)
	assert.Equal(t, "aws_s3_bucket.new_x", pairs[0].To)
}

func TestConfirmCandidates_Empty(t *testing.T) {
	var output bytes.Buffer
	pairs, err := ConfirmCandidates(strings.NewReader(""), &output, nil)
	require.NoError(t, err)
	assert.Empty(t, pairs)
}
