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
      "name": "old_name",
      "change": {"actions": ["delete"]}
    },
    {
      "address": "aws_s3_bucket.new_name",
      "type": "aws_s3_bucket",
      "name": "new_name",
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
      "name": "old_bucket",
      "change": {"actions": ["delete"]}
    },
    {
      "address": "aws_s3_bucket.new_bucket_a",
      "type": "aws_s3_bucket",
      "name": "new_bucket_a",
      "change": {"actions": ["create"]}
    },
    {
      "address": "aws_s3_bucket.new_bucket_b",
      "type": "aws_s3_bucket",
      "name": "new_bucket_b",
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
      "name": "old_role",
      "change": {"actions": ["delete"]}
    },
    {
      "address": "aws_iam_role.new_role",
      "type": "aws_iam_role",
      "name": "new_role",
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
      "name": "old",
      "change": {"actions": ["delete"]}
    },
    {
      "address": "aws_iam_role.new",
      "type": "aws_iam_role",
      "name": "new",
      "change": {"actions": ["create"]}
    }
  ]
}`

const planWithSameTypeMultipleName = `{
  "format_version": "1.0",
  "resource_changes": [
    {
      "address": "acme_badge.coverage[\"old-proj\"]",
      "type": "acme_badge",
      "name": "coverage",
      "change": {"actions": ["delete"]}
    },
    {
      "address": "acme_badge.coverage[\"new-proj\"]",
      "type": "acme_badge",
      "name": "coverage",
      "change": {"actions": ["create"]}
    },
    {
      "address": "acme_badge.develop[\"new-proj\"]",
      "type": "acme_badge",
      "name": "develop",
      "change": {"actions": ["create"]}
    },
    {
      "address": "acme_badge.main[\"new-proj\"]",
      "type": "acme_badge",
      "name": "main",
      "change": {"actions": ["create"]}
    }
  ]
}`

const planWithSameTypeAndNameMultipleCreates = `{
  "format_version": "1.0",
  "resource_changes": [
    {
      "address": "acme_badge.coverage[\"a\"]",
      "type": "acme_badge",
      "name": "coverage",
      "change": {"actions": ["delete"]}
    },
    {
      "address": "acme_badge.coverage[\"b\"]",
      "type": "acme_badge",
      "name": "coverage",
      "change": {"actions": ["create"]}
    },
    {
      "address": "acme_badge.coverage[\"c\"]",
      "type": "acme_badge",
      "name": "coverage",
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

// TestMatchDestroyCreate_TypeOnlyFallback verifies the second-tier matching:
// a destroy and create of the same type but different names should still produce
// a candidate via the type-only fallback.
func TestMatchDestroyCreate_TypeOnlyFallback(t *testing.T) {
	plan := `{
  "format_version": "1.0",
  "resource_changes": [
    {
      "address": "aws_s3_bucket.old_data",
      "type": "aws_s3_bucket",
      "name": "old_data",
      "change": {"actions": ["delete"]}
    },
    {
      "address": "aws_s3_bucket.new_data",
      "type": "aws_s3_bucket",
      "name": "new_data",
      "change": {"actions": ["create"]}
    }
  ]
}`
	p := parsePlan(t, plan)
	candidates := MatchDestroyCreate(p)

	// No exact type+name match, but type-only fallback should find it
	require.Len(t, candidates, 1)
	assert.Equal(t, "aws_s3_bucket.old_data", candidates[0].Destroy.Address)
	require.Len(t, candidates[0].Creates, 1)
	assert.Equal(t, "aws_s3_bucket.new_data", candidates[0].Creates[0].Address)
}

// TestMatchDestroyCreate_TypeOnlySkipsMatched verifies that the second-tier
// matching does not re-offer destroys or creates already matched in tier one.
func TestMatchDestroyCreate_TypeOnlySkipsMatched(t *testing.T) {
	plan := `{
  "format_version": "1.0",
  "resource_changes": [
    {
      "address": "aws_s3_bucket.data[\"old\"]",
      "type": "aws_s3_bucket",
      "name": "data",
      "change": {"actions": ["delete"]}
    },
    {
      "address": "aws_s3_bucket.data[\"new\"]",
      "type": "aws_s3_bucket",
      "name": "data",
      "change": {"actions": ["create"]}
    },
    {
      "address": "aws_s3_bucket.orphan",
      "type": "aws_s3_bucket",
      "name": "orphan",
      "change": {"actions": ["delete"]}
    }
  ]
}`
	p := parsePlan(t, plan)
	candidates := MatchDestroyCreate(p)

	// Tier 1 matches data[old] -> data[new] (same name).
	// Tier 2 should match orphan -> but NO available creates (data[new] already matched).
	// So only 1 candidate total.
	require.Len(t, candidates, 1)
	assert.Equal(t, "aws_s3_bucket.data[\"old\"]", candidates[0].Destroy.Address)
}

func TestMatchDestroyCreate_SameTypeDifferentName(t *testing.T) {
	p := parsePlan(t, planWithSameTypeMultipleName)
	candidates := MatchDestroyCreate(p)

	// Should have exactly 1 candidate (the coverage destroy)
	// with exactly 1 create (coverage["new-proj"]), NOT 3 creates
	require.Len(t, candidates, 1)
	assert.Equal(t, "acme_badge.coverage[\"old-proj\"]", candidates[0].Destroy.Address)
	require.Len(t, candidates[0].Creates, 1)
	assert.Equal(t, "acme_badge.coverage[\"new-proj\"]", candidates[0].Creates[0].Address)
}

func TestMatchDestroyCreate_SameTypeNameMultipleCreates(t *testing.T) {
	p := parsePlan(t, planWithSameTypeAndNameMultipleCreates)
	candidates := MatchDestroyCreate(p)

	// Should have exactly 1 candidate (the coverage destroy)
	// with exactly 2 creates (both coverage name)
	require.Len(t, candidates, 1)
	assert.Equal(t, "acme_badge.coverage[\"a\"]", candidates[0].Destroy.Address)
	require.Len(t, candidates[0].Creates, 2)
}
