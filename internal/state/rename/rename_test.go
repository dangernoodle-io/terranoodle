package rename

import (
	"encoding/json"
	"strings"
	"testing"

	tfjson "github.com/hashicorp/terraform-json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func parsePlan(t *testing.T, jsonStr string) *tfjson.Plan {
	t.Helper()
	var p tfjson.Plan
	err := json.NewDecoder(strings.NewReader(jsonStr)).Decode(&p)
	require.NoError(t, err)
	return &p
}

const planWithPreviousAddress = `{
  "format_version": "1.0",
  "resource_changes": [
    {
      "address": "module.storage.aws_s3_bucket.data",
      "previous_address": "aws_s3_bucket.data",
      "type": "aws_s3_bucket",
      "change": {"actions": ["no-op"]}
    },
    {
      "address": "module.storage.aws_s3_bucket.logs",
      "previous_address": "aws_s3_bucket.logs",
      "type": "aws_s3_bucket",
      "change": {"actions": ["no-op"]}
    },
    {
      "address": "aws_iam_role.app",
      "type": "aws_iam_role",
      "change": {"actions": ["no-op"]}
    }
  ]
}`

const planWithNoPreviousAddress = `{
  "format_version": "1.0",
  "resource_changes": [
    {
      "address": "aws_s3_bucket.data",
      "type": "aws_s3_bucket",
      "change": {"actions": ["no-op"]}
    }
  ]
}`

const planWithDestroyCreatePairs = `{
  "format_version": "1.0",
  "resource_changes": [
    {
      "address": "aws_s3_bucket.old_name",
      "type": "aws_s3_bucket",
      "change": {"actions": ["delete"]}
    },
    {
      "address": "aws_s3_bucket.new_name",
      "type": "aws_s3_bucket",
      "change": {"actions": ["create"]}
    },
    {
      "address": "aws_iam_role.app",
      "type": "aws_iam_role",
      "change": {"actions": ["no-op"]}
    }
  ]
}`

const planWithMultipleCreates = `{
  "format_version": "1.0",
  "resource_changes": [
    {
      "address": "aws_s3_bucket.old_bucket",
      "type": "aws_s3_bucket",
      "change": {"actions": ["delete"]}
    },
    {
      "address": "aws_s3_bucket.new_bucket_a",
      "type": "aws_s3_bucket",
      "change": {"actions": ["create"]}
    },
    {
      "address": "aws_s3_bucket.new_bucket_b",
      "type": "aws_s3_bucket",
      "change": {"actions": ["create"]}
    }
  ]
}`

const planWithMixedRenamesAndCandidates = `{
  "format_version": "1.0",
  "resource_changes": [
    {
      "address": "module.storage.aws_s3_bucket.data",
      "previous_address": "aws_s3_bucket.data",
      "type": "aws_s3_bucket",
      "change": {"actions": ["no-op"]}
    },
    {
      "address": "aws_iam_role.old_role",
      "type": "aws_iam_role",
      "change": {"actions": ["delete"]}
    },
    {
      "address": "aws_iam_role.new_role",
      "type": "aws_iam_role",
      "change": {"actions": ["create"]}
    }
  ]
}`

const planWithDifferentTypeDestroyCreate = `{
  "format_version": "1.0",
  "resource_changes": [
    {
      "address": "aws_s3_bucket.old",
      "type": "aws_s3_bucket",
      "change": {"actions": ["delete"]}
    },
    {
      "address": "aws_iam_role.new",
      "type": "aws_iam_role",
      "change": {"actions": ["create"]}
    }
  ]
}`

func TestDetectFromPlan_WithPreviousAddress(t *testing.T) {
	p := parsePlan(t, planWithPreviousAddress)
	pairs := DetectFromPlan(p)

	require.Len(t, pairs, 2)
	// Sorted by From address
	assert.Equal(t, "aws_s3_bucket.data", pairs[0].From)
	assert.Equal(t, "module.storage.aws_s3_bucket.data", pairs[0].To)
	assert.Equal(t, "aws_s3_bucket.logs", pairs[1].From)
	assert.Equal(t, "module.storage.aws_s3_bucket.logs", pairs[1].To)
}

func TestDetectFromPlan_NoPreviousAddress(t *testing.T) {
	p := parsePlan(t, planWithNoPreviousAddress)
	pairs := DetectFromPlan(p)
	assert.Empty(t, pairs)
}

func TestDetectFromPlan_NilPlan(t *testing.T) {
	pairs := DetectFromPlan(&tfjson.Plan{})
	assert.Empty(t, pairs)
}

func TestMatchDestroyCreate_SinglePair(t *testing.T) {
	p := parsePlan(t, planWithDestroyCreatePairs)
	candidates := MatchDestroyCreate(p)

	require.Len(t, candidates, 1)
	assert.Equal(t, "aws_s3_bucket.old_name", candidates[0].Destroy.Address)
	require.Len(t, candidates[0].Creates, 1)
	assert.Equal(t, "aws_s3_bucket.new_name", candidates[0].Creates[0].Address)
}

func TestMatchDestroyCreate_MultipleCreates(t *testing.T) {
	p := parsePlan(t, planWithMultipleCreates)
	candidates := MatchDestroyCreate(p)

	require.Len(t, candidates, 1)
	assert.Equal(t, "aws_s3_bucket.old_bucket", candidates[0].Destroy.Address)
	require.Len(t, candidates[0].Creates, 2)
}

func TestMatchDestroyCreate_ExcludesKnownMoved(t *testing.T) {
	p := parsePlan(t, planWithMixedRenamesAndCandidates)
	candidates := MatchDestroyCreate(p)

	// Only the IAM role pair should be a candidate; the S3 bucket has PreviousAddress
	require.Len(t, candidates, 1)
	assert.Equal(t, "aws_iam_role.old_role", candidates[0].Destroy.Address)
	require.Len(t, candidates[0].Creates, 1)
	assert.Equal(t, "aws_iam_role.new_role", candidates[0].Creates[0].Address)
}

func TestMatchDestroyCreate_DifferentTypes(t *testing.T) {
	p := parsePlan(t, planWithDifferentTypeDestroyCreate)
	candidates := MatchDestroyCreate(p)
	assert.Empty(t, candidates)
}

func TestMatchDestroyCreate_EmptyPlan(t *testing.T) {
	candidates := MatchDestroyCreate(&tfjson.Plan{})
	assert.Empty(t, candidates)
}
