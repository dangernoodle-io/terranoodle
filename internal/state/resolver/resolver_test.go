package resolver_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	tfjson "github.com/hashicorp/terraform-json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dangernoodle.io/terranoodle/internal/state/config"
	"dangernoodle.io/terranoodle/internal/state/resolver"
)

// makeRC is a convenience constructor for tfjson.ResourceChange.
func makeRC(address, typ string, index interface{}, after map[string]interface{}) *tfjson.ResourceChange {
	return &tfjson.ResourceChange{
		Address: address,
		Type:    typ,
		Index:   index,
		Change: &tfjson.Change{
			After: after,
		},
	}
}

// newGitLabServer builds an httptest.Server whose mux handles the two
// endpoints used by tests 2 and 3.
//
// pathEncode("acme-corp", "docker-images") produces "acme-corp/docker-images"
// (each segment percent-encoded then joined with "/"), so the project lookup
// path is /projects/acme-corp/docker-images — two separate path segments.
//
//	GET /projects/acme-corp/docker-images  → {"id": 42}
//	GET /projects/42/badges                → [{"name":"coverage","id":99}]
func newGitLabServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	// Matches /projects/acme-corp/docker-images exactly.
	mux.HandleFunc("/projects/acme-corp/docker-images", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": 42})
	})

	mux.HandleFunc("/projects/42/badges", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]interface{}{
			map[string]interface{}{"name": "coverage", "id": 99},
		})
	})

	return httptest.NewServer(mux)
}

// gitlabConfig returns a *config.Config shaped like the testdata gitlab.yaml
// but pointing resolvers at the provided base URL.  Only the types needed by
// the active test case are included so each test can trim it down further.
//
// Note: the badge_id resolver's where clause uses a literal "coverage" value
// rather than the template "{{ .name }}" from the YAML testdata, because
// Pick's matchesWhere performs a direct string comparison — where values are
// not rendered as templates by the current implementation.
func gitlabConfig(baseURL string) *config.Config {
	return &config.Config{
		API: &config.API{
			BaseURL:  baseURL,
			TokenEnv: "TEST_TOKEN",
		},
		Vars: map[string]string{
			"group_path": "acme-corp",
		},
		Resolvers: map[string]config.Resolver{
			"project_id": {
				Get:  "/projects/{{ pathEncode $group_path .key }}",
				Pick: "id",
			},
			"badge_id": {
				Use: []string{"project_id"},
				Get: "/projects/{{ $project_id }}/badges",
				Pick: map[string]interface{}{
					"where": map[string]interface{}{"name": "coverage"},
					"field": "id",
				},
			},
		},
		Types: map[string]config.TypeMapping{
			"gitlab_project": {
				ID: "{{ $group_path }}/{{ .key }}",
			},
			"gitlab_project_push_rules": {
				Use: []string{"project_id"},
				ID:  "{{ $project_id }}",
			},
			"gitlab_project_badge": {
				Use: []string{"project_id", "badge_id"},
				ID:  "{{ $project_id }}:{{ $badge_id }}",
			},
		},
	}
}

// TestStaticTemplates verifies that GCP-style configs with no resolvers
// correctly substitute .name, .zone, and $project from cfg.Vars.
func TestStaticTemplates(t *testing.T) {
	cfg := &config.Config{
		Vars: map[string]string{
			"project": "my-project",
		},
		Types: map[string]config.TypeMapping{
			"google_compute_instance": {
				ID: "projects/{{ $project }}/zones/{{ .zone }}/instances/{{ .name }}",
			},
		},
	}

	resources := []*tfjson.ResourceChange{
		makeRC(
			"google_compute_instance.web",
			"google_compute_instance",
			nil,
			map[string]interface{}{
				"name": "web-server",
				"zone": "us-central1-a",
			},
		),
	}

	result, err := resolver.Resolve(resources, cfg, nil, nil)
	require.NoError(t, err)
	require.Len(t, result.Matched, 1)
	assert.Equal(t, "projects/my-project/zones/us-central1-a/instances/web-server", result.Matched[0].ID)
	assert.Equal(t, "google_compute_instance.web", result.Matched[0].Address)
	assert.Empty(t, result.Unmatched)
}

// TestSingleResolver mocks /projects/acme-corp%2Fdocker-images → {"id": 42}
// and verifies that $project_id resolves to "42".
func TestSingleResolver(t *testing.T) {
	t.Setenv("TEST_TOKEN", "fake-token")

	srv := newGitLabServer(t)
	defer srv.Close()

	cfg := gitlabConfig(srv.URL)
	// Trim to only the type we need for this test.
	cfg.Types = map[string]config.TypeMapping{
		"gitlab_project_push_rules": cfg.Types["gitlab_project_push_rules"],
	}

	resources := []*tfjson.ResourceChange{
		makeRC(
			`module.project.gitlab_project_push_rules.rules["docker-images"]`,
			"gitlab_project_push_rules",
			"docker-images",
			map[string]interface{}{
				"commit_committer_check": true,
			},
		),
	}

	client := resolver.NewAPIClient(srv.URL, "fake-token")
	result, err := resolver.Resolve(resources, cfg, nil, client)
	require.NoError(t, err)
	require.Len(t, result.Matched, 1)
	assert.Equal(t, "42", result.Matched[0].ID)
	assert.Empty(t, result.Unmatched)
}

// TestChainedResolvers mocks both API endpoints and verifies that $project_id
// and $badge_id both resolve correctly for a gitlab_project_badge resource.
func TestChainedResolvers(t *testing.T) {
	t.Setenv("TEST_TOKEN", "fake-token")

	srv := newGitLabServer(t)
	defer srv.Close()

	cfg := gitlabConfig(srv.URL)
	// Trim to only the types we need.
	cfg.Types = map[string]config.TypeMapping{
		"gitlab_project_badge": cfg.Types["gitlab_project_badge"],
	}

	resources := []*tfjson.ResourceChange{
		makeRC(
			`module.project.gitlab_project_badge.badge["docker-images"]`,
			"gitlab_project_badge",
			"docker-images",
			map[string]interface{}{
				"name":      "coverage",
				"link_url":  "https://example.com",
				"image_url": "https://example.com/badge.svg",
			},
		),
	}

	client := resolver.NewAPIClient(srv.URL, "fake-token")
	result, err := resolver.Resolve(resources, cfg, nil, client)
	require.NoError(t, err)
	require.Len(t, result.Matched, 1)
	assert.Equal(t, "42:99", result.Matched[0].ID)
	assert.Empty(t, result.Unmatched)
}

// TestUnmatchedResources verifies that resource types without a mapping in
// cfg.Types are collected in Result.Unmatched and not in Result.Matched.
func TestUnmatchedResources(t *testing.T) {
	cfg := &config.Config{
		Types: map[string]config.TypeMapping{
			"known_resource_type": {
				ID: "{{ .name }}",
			},
		},
	}

	resources := []*tfjson.ResourceChange{
		makeRC("known_resource_type.foo", "known_resource_type", nil,
			map[string]interface{}{"name": "foo"}),
		makeRC("unknown_resource_type.bar", "unknown_resource_type", nil,
			map[string]interface{}{"name": "bar"}),
		makeRC("another_unknown.baz", "another_unknown", nil,
			map[string]interface{}{}),
	}

	result, err := resolver.Resolve(resources, cfg, nil, nil)
	require.NoError(t, err)
	require.Len(t, result.Matched, 1)
	assert.Equal(t, "foo", result.Matched[0].ID)

	require.Len(t, result.Unmatched, 2)
	assert.Contains(t, result.Unmatched, "unknown_resource_type.bar")
	assert.Contains(t, result.Unmatched, "another_unknown.baz")
}

// TestGitLabResolverChain exercises the full GitLab resolver chain matching the
// testdata/configs/gitlab.yaml pattern. It verifies static IDs, single-resolver
// IDs (project_id), chained IDs (project_id + badge_id / rule_id), and IDs
// that compose project_id with plan values (branch, file_path).
//
// The where clause values use "{{ .name }}" to test that matchesWhere renders
// them as templates before comparing.
func TestGitLabResolverChain(t *testing.T) {
	t.Setenv("TEST_TOKEN", "fake")

	mux := http.NewServeMux()
	mux.HandleFunc("/projects/acme-corp/docker-images", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": 42, "name": "docker-images"})
	})
	mux.HandleFunc("/projects/42/badges", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]interface{}{
			map[string]interface{}{"name": "coverage", "id": 99},
			map[string]interface{}{"name": "pipeline", "id": 100},
		})
	})
	mux.HandleFunc("/projects/42/approval_rules", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]interface{}{
			map[string]interface{}{"name": "devops-approval", "id": 55},
			map[string]interface{}{"name": "other", "id": 56},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cfg := &config.Config{
		API: &config.API{
			BaseURL:  srv.URL,
			TokenEnv: "TEST_TOKEN",
		},
		Vars: map[string]string{
			"group_path": "acme-corp",
		},
		Resolvers: map[string]config.Resolver{
			"project_id": {
				Get:  "/projects/{{ pathEncode $group_path .key }}",
				Pick: "id",
			},
			"badge_id": {
				Use: []string{"project_id"},
				Get: "/projects/{{ $project_id }}/badges",
				Pick: map[string]interface{}{
					"where": map[string]interface{}{"name": "{{ .name }}"},
					"field": "id",
				},
			},
			"rule_id": {
				Use: []string{"project_id"},
				Get: "/projects/{{ $project_id }}/approval_rules",
				Pick: map[string]interface{}{
					"where": map[string]interface{}{"name": "{{ .name }}"},
					"field": "id",
				},
			},
		},
		Types: map[string]config.TypeMapping{
			"gitlab_project": {
				ID: "{{ $group_path }}/{{ .key }}",
			},
			"gitlab_project_push_rules": {
				Use: []string{"project_id"},
				ID:  "{{ $project_id }}",
			},
			"gitlab_project_badge": {
				Use: []string{"project_id", "badge_id"},
				ID:  "{{ $project_id }}:{{ $badge_id }}",
			},
			"gitlab_project_approval_rule": {
				Use: []string{"project_id", "rule_id"},
				ID:  "{{ $project_id }}:{{ $rule_id }}",
			},
			"gitlab_branch_protection": {
				Use: []string{"project_id"},
				ID:  "{{ $project_id }}:{{ .branch }}",
			},
			"gitlab_repository_file": {
				Use: []string{"project_id"},
				ID:  "{{ $project_id }}:{{ .branch }}:{{ .file_path }}",
			},
		},
	}

	resources := []*tfjson.ResourceChange{
		makeRC(
			`module.project.gitlab_project.project["docker-images"]`,
			"gitlab_project",
			"docker-images",
			map[string]interface{}{},
		),
		makeRC(
			`module.project.gitlab_project_push_rules.rules["docker-images"]`,
			"gitlab_project_push_rules",
			"docker-images",
			map[string]interface{}{},
		),
		makeRC(
			`module.project.gitlab_project_badge.badge["docker-images"]`,
			"gitlab_project_badge",
			"docker-images",
			map[string]interface{}{"name": "coverage"},
		),
		makeRC(
			`module.project.gitlab_project_approval_rule.rule["docker-images"]`,
			"gitlab_project_approval_rule",
			"docker-images",
			map[string]interface{}{"name": "devops-approval"},
		),
		makeRC(
			`module.project.gitlab_branch_protection.main["docker-images"]`,
			"gitlab_branch_protection",
			"docker-images",
			map[string]interface{}{"branch": "main"},
		),
		makeRC(
			`module.project.gitlab_repository_file.codeowners["docker-images"]`,
			"gitlab_repository_file",
			"docker-images",
			map[string]interface{}{"branch": "main", "file_path": "CODEOWNERS"},
		),
	}

	client := resolver.NewAPIClient(srv.URL, "fake")
	result, err := resolver.Resolve(resources, cfg, nil, client)
	require.NoError(t, err)
	require.Len(t, result.Matched, 6)
	assert.Empty(t, result.Unmatched)

	byAddr := make(map[string]string, len(result.Matched))
	for _, e := range result.Matched {
		byAddr[e.Address] = e.ID
	}

	assert.Equal(t, "acme-corp/docker-images",
		byAddr[`module.project.gitlab_project.project["docker-images"]`])
	assert.Equal(t, "42",
		byAddr[`module.project.gitlab_project_push_rules.rules["docker-images"]`])
	assert.Equal(t, "42:99",
		byAddr[`module.project.gitlab_project_badge.badge["docker-images"]`])
	assert.Equal(t, "42:55",
		byAddr[`module.project.gitlab_project_approval_rule.rule["docker-images"]`])
	assert.Equal(t, "42:main",
		byAddr[`module.project.gitlab_branch_protection.main["docker-images"]`])
	assert.Equal(t, "42:main:CODEOWNERS",
		byAddr[`module.project.gitlab_repository_file.codeowners["docker-images"]`])
}

// TestGitHubResolverChain exercises the GitHub resolver chain matching the
// testdata/configs/github.yaml pattern. It covers static IDs, team_id resolver
// (used by two types), and repo_ruleset_id (where + field pick).
func TestGitHubResolverChain(t *testing.T) {
	t.Setenv("TEST_TOKEN", "fake")

	mux := http.NewServeMux()
	mux.HandleFunc("/orgs/acme-corp-io/teams/developers", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": 777, "name": "developers"})
	})
	mux.HandleFunc("/repos/acme-corp-io/hello-world/rulesets", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]interface{}{
			map[string]interface{}{"name": "default", "id": 333},
			map[string]interface{}{"name": "other", "id": 444},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cfg := &config.Config{
		API: &config.API{
			BaseURL:  srv.URL,
			TokenEnv: "TEST_TOKEN",
		},
		Vars: map[string]string{
			"owner": "acme-corp-io",
		},
		Resolvers: map[string]config.Resolver{
			"team_id": {
				Get:  "/orgs/{{ $owner }}/teams/{{ .name }}",
				Pick: "id",
			},
			"repo_ruleset_id": {
				Get: "/repos/{{ $owner }}/{{ .key }}/rulesets",
				Pick: map[string]interface{}{
					"where": map[string]interface{}{"name": "{{ .name }}"},
					"field": "id",
				},
			},
		},
		Types: map[string]config.TypeMapping{
			"github_repository": {
				ID: "{{ .name }}",
			},
			"github_team": {
				Use: []string{"team_id"},
				ID:  "{{ $team_id }}",
			},
			"github_team_members": {
				Use: []string{"team_id"},
				ID:  "{{ $team_id }}",
			},
			"github_membership": {
				ID: "{{ $owner }}:{{ .username }}",
			},
			"github_repository_ruleset": {
				Use: []string{"repo_ruleset_id"},
				ID:  "{{ $owner }}/{{ .key }}:{{ $repo_ruleset_id }}",
			},
		},
	}

	resources := []*tfjson.ResourceChange{
		makeRC(
			`github_repository.hello-world`,
			"github_repository",
			nil,
			map[string]interface{}{"name": "hello-world"},
		),
		makeRC(
			`github_team.developers`,
			"github_team",
			nil,
			map[string]interface{}{"name": "developers"},
		),
		makeRC(
			`github_team_members.developers`,
			"github_team_members",
			nil,
			map[string]interface{}{"name": "developers"},
		),
		makeRC(
			`github_membership.jdoe`,
			"github_membership",
			nil,
			map[string]interface{}{"username": "jdoe"},
		),
		makeRC(
			`github_repository_ruleset.hello-world`,
			"github_repository_ruleset",
			"hello-world",
			map[string]interface{}{"name": "default"},
		),
	}

	client := resolver.NewAPIClient(srv.URL, "fake")
	result, err := resolver.Resolve(resources, cfg, nil, client)
	require.NoError(t, err)
	require.Len(t, result.Matched, 5)
	assert.Empty(t, result.Unmatched)

	byAddr := make(map[string]string, len(result.Matched))
	for _, e := range result.Matched {
		byAddr[e.Address] = e.ID
	}

	assert.Equal(t, "hello-world", byAddr[`github_repository.hello-world`])
	assert.Equal(t, "777", byAddr[`github_team.developers`])
	assert.Equal(t, "777", byAddr[`github_team_members.developers`])
	assert.Equal(t, "acme-corp-io:jdoe", byAddr[`github_membership.jdoe`])
	assert.Equal(t, "acme-corp-io/hello-world:333", byAddr[`github_repository_ruleset.hello-world`])
}

// TestResolverCaching verifies that the APIClient caches responses — two
// resources that hit the same API endpoint result in only one HTTP request.
func TestResolverCaching(t *testing.T) {
	t.Setenv("TEST_TOKEN", "fake")

	hitCount := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/projects/acme-corp/alpha", func(w http.ResponseWriter, r *http.Request) {
		hitCount++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": 10})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cfg := &config.Config{
		API: &config.API{
			BaseURL:  srv.URL,
			TokenEnv: "TEST_TOKEN",
		},
		Vars: map[string]string{
			"group_path": "acme-corp",
		},
		Resolvers: map[string]config.Resolver{
			"project_id": {
				Get:  "/projects/{{ pathEncode $group_path .key }}",
				Pick: "id",
			},
		},
		Types: map[string]config.TypeMapping{
			"gitlab_project_push_rules": {
				Use: []string{"project_id"},
				ID:  "{{ $project_id }}",
			},
		},
	}

	// Two resources with the same key "alpha" — both resolve via the same URL.
	resources := []*tfjson.ResourceChange{
		makeRC(
			`module.project.gitlab_project_push_rules.rules["alpha-1"]`,
			"gitlab_project_push_rules",
			"alpha",
			map[string]interface{}{},
		),
		makeRC(
			`module.project.gitlab_project_push_rules.rules["alpha-2"]`,
			"gitlab_project_push_rules",
			"alpha",
			map[string]interface{}{},
		),
	}

	client := resolver.NewAPIClient(srv.URL, "fake-token")
	result, err := resolver.Resolve(resources, cfg, nil, client)
	require.NoError(t, err)
	require.Len(t, result.Matched, 2)

	for _, e := range result.Matched {
		assert.Equal(t, "10", e.ID)
	}

	assert.Equal(t, 1, hitCount, "expected API endpoint to be called only once due to caching")
}

// TestVarOverrides verifies that varOverrides supplied to Resolve win over the
// same key defined in cfg.Vars.
func TestVarOverrides(t *testing.T) {
	cfg := &config.Config{
		Vars: map[string]string{
			"project": "base-project",
			"region":  "us-east1",
		},
		Types: map[string]config.TypeMapping{
			"google_compute_subnetwork": {
				ID: "projects/{{ $project }}/regions/{{ $region }}/subnetworks/{{ .name }}",
			},
		},
	}

	resources := []*tfjson.ResourceChange{
		makeRC("google_compute_subnetwork.main", "google_compute_subnetwork", nil,
			map[string]interface{}{"name": "main-subnet"}),
	}

	// Override "project" — "region" stays from cfg.Vars.
	overrides := map[string]string{
		"project": "override-project",
	}

	result, err := resolver.Resolve(resources, cfg, overrides, nil)
	require.NoError(t, err)
	require.Len(t, result.Matched, 1)
	// "project" should come from overrides, "region" from cfg.Vars.
	assert.Equal(t, "projects/override-project/regions/us-east1/subnetworks/main-subnet", result.Matched[0].ID)
}
