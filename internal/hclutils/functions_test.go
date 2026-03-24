package hclutils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestEvalContext(t *testing.T) {
	t.Run("returns non-nil context", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "terragrunt.hcl")

		ctx := EvalContext(configPath)
		assert.NotNil(t, ctx)
		assert.NotEmpty(t, ctx.Functions)
	})

	t.Run("has required functions", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "terragrunt.hcl")

		ctx := EvalContext(configPath)
		requiredFuncs := []string{
			"get_terragrunt_dir",
			"get_repo_root",
			"get_path_to_repo_root",
			"get_env",
			"get_path_from_repo_root",
			"find_in_parent_folders",
			"basename",
			"replace",
			"path_relative_to_include",
			"read_terragrunt_config",
		}

		for _, fn := range requiredFuncs {
			assert.Contains(t, ctx.Functions, fn, "missing function: %s", fn)
		}
	})
}

func TestGetTerragruntDir(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "terragrunt.hcl")

	ctx := EvalContext(configPath)
	fn := ctx.Functions["get_terragrunt_dir"]

	result, err := fn.Call([]cty.Value{})
	require.NoError(t, err)
	assert.Equal(t, dir, result.AsString())
}

func TestBasename(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "terragrunt.hcl")

	ctx := EvalContext(configPath)
	fn := ctx.Functions["basename"]

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "terraform file",
			input:    "/opt/modules/vpc/main.tf",
			expected: "main.tf",
		},
		{
			name:     "directory path",
			input:    "/opt/modules/vpc/",
			expected: "vpc",
		},
		{
			name:     "relative path",
			input:    "modules/vpc/main.tf",
			expected: "main.tf",
		},
		{
			name:     "simple filename",
			input:    "main.tf",
			expected: "main.tf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := fn.Call([]cty.Value{cty.StringVal(tt.input)})
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result.AsString())
		})
	}
}

func TestReplace(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "terragrunt.hcl")

	ctx := EvalContext(configPath)
	fn := ctx.Functions["replace"]

	tests := []struct {
		name     string
		str      string
		old      string
		new      string
		expected string
	}{
		{
			name:     "replace dashes with underscores",
			str:      "acme-corp",
			old:      "-",
			new:      "_",
			expected: "acme_corp",
		},
		{
			name:     "replace multiple occurrences",
			str:      "hello-world-example",
			old:      "-",
			new:      "_",
			expected: "hello_world_example",
		},
		{
			name:     "no match",
			str:      "acme-corp",
			old:      "x",
			new:      "y",
			expected: "acme-corp",
		},
		{
			name:     "empty string",
			str:      "",
			old:      "-",
			new:      "_",
			expected: "",
		},
		{
			name:     "replace substring",
			str:      "api.example.com",
			old:      ".",
			new:      "-",
			expected: "api-example-com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := fn.Call([]cty.Value{
				cty.StringVal(tt.str),
				cty.StringVal(tt.old),
				cty.StringVal(tt.new),
			})
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result.AsString())
		})
	}
}

func TestGetEnv(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "terragrunt.hcl")

	ctx := EvalContext(configPath)
	fn := ctx.Functions["get_env"]

	t.Run("env var present", func(t *testing.T) {
		t.Setenv("TT_TEST_VAR", "test-value")
		result, err := fn.Call([]cty.Value{cty.StringVal("TT_TEST_VAR")})
		require.NoError(t, err)
		assert.Equal(t, "test-value", result.AsString())
	})

	t.Run("env var with default fallback", func(t *testing.T) {
		// Make sure var doesn't exist
		os.Unsetenv("TT_NONEXISTENT_VAR_XYZ")
		result, err := fn.Call([]cty.Value{
			cty.StringVal("TT_NONEXISTENT_VAR_XYZ"),
			cty.StringVal("fallback"),
		})
		require.NoError(t, err)
		assert.Equal(t, "fallback", result.AsString())
	})

	t.Run("env var missing no default", func(t *testing.T) {
		// Make sure var doesn't exist
		os.Unsetenv("TT_NONEXISTENT_VAR_ABC")
		result, err := fn.Call([]cty.Value{cty.StringVal("TT_NONEXISTENT_VAR_ABC")})
		require.NoError(t, err)
		assert.Equal(t, "", result.AsString())
	})
}

func TestFindInParentFolders(t *testing.T) {
	t.Run("finds file in current dir", func(t *testing.T) {
		dir := t.TempDir()
		terragruntFile := filepath.Join(dir, "terragrunt.hcl")
		err := os.WriteFile(terragruntFile, []byte("# test"), 0644)
		require.NoError(t, err)

		configPath := filepath.Join(dir, "terragrunt.hcl")
		ctx := EvalContext(configPath)
		fn := ctx.Functions["find_in_parent_folders"]

		result, err := fn.Call([]cty.Value{})
		require.NoError(t, err)
		assert.Equal(t, terragruntFile, result.AsString())
	})

	t.Run("finds file in parent dir", func(t *testing.T) {
		rootDir := t.TempDir()
		terragruntFile := filepath.Join(rootDir, "terragrunt.hcl")
		err := os.WriteFile(terragruntFile, []byte("# test"), 0644)
		require.NoError(t, err)

		subdir := filepath.Join(rootDir, "modules", "vpc")
		err = os.MkdirAll(subdir, 0755)
		require.NoError(t, err)

		configPath := filepath.Join(subdir, "terragrunt.hcl")
		ctx := EvalContext(configPath)
		fn := ctx.Functions["find_in_parent_folders"]

		result, err := fn.Call([]cty.Value{})
		require.NoError(t, err)
		assert.Equal(t, terragruntFile, result.AsString())
	})

	t.Run("finds custom filename", func(t *testing.T) {
		rootDir := t.TempDir()
		customFile := filepath.Join(rootDir, "terragrunt.hcl.json")
		err := os.WriteFile(customFile, []byte("{}"), 0644)
		require.NoError(t, err)

		configPath := filepath.Join(rootDir, "terragrunt.hcl")
		ctx := EvalContext(configPath)
		fn := ctx.Functions["find_in_parent_folders"]

		result, err := fn.Call([]cty.Value{cty.StringVal("terragrunt.hcl.json")})
		require.NoError(t, err)
		assert.Equal(t, customFile, result.AsString())
	})

	t.Run("file not found returns error", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "terragrunt.hcl")
		ctx := EvalContext(configPath)
		fn := ctx.Functions["find_in_parent_folders"]

		_, err := fn.Call([]cty.Value{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestFindRepoRoot(t *testing.T) {
	t.Run("finds .git directory", func(t *testing.T) {
		rootDir := t.TempDir()
		gitDir := filepath.Join(rootDir, ".git")
		err := os.MkdirAll(gitDir, 0755)
		require.NoError(t, err)

		subdir := filepath.Join(rootDir, "modules", "vpc")
		err = os.MkdirAll(subdir, 0755)
		require.NoError(t, err)

		root, err := findRepoRoot(subdir)
		require.NoError(t, err)
		assert.Equal(t, rootDir, root)
	})

	t.Run("finds .git in current dir", func(t *testing.T) {
		rootDir := t.TempDir()
		gitDir := filepath.Join(rootDir, ".git")
		err := os.MkdirAll(gitDir, 0755)
		require.NoError(t, err)

		root, err := findRepoRoot(rootDir)
		require.NoError(t, err)
		assert.Equal(t, rootDir, root)
	})

	t.Run("no .git found returns error", func(t *testing.T) {
		dir := t.TempDir()
		_, err := findRepoRoot(dir)
		require.Error(t, err)
		assert.Equal(t, os.ErrNotExist, err)
	})

	t.Run("walks up multiple levels", func(t *testing.T) {
		rootDir := t.TempDir()
		gitDir := filepath.Join(rootDir, ".git")
		err := os.MkdirAll(gitDir, 0755)
		require.NoError(t, err)

		deepPath := filepath.Join(rootDir, "a", "b", "c", "d")
		err = os.MkdirAll(deepPath, 0755)
		require.NoError(t, err)

		root, err := findRepoRoot(deepPath)
		require.NoError(t, err)
		assert.Equal(t, rootDir, root)
	})
}

func TestGetRepoRoot(t *testing.T) {
	t.Run("with .git directory", func(t *testing.T) {
		rootDir := t.TempDir()
		gitDir := filepath.Join(rootDir, ".git")
		err := os.MkdirAll(gitDir, 0755)
		require.NoError(t, err)

		subdir := filepath.Join(rootDir, "modules", "vpc")
		err = os.MkdirAll(subdir, 0755)
		require.NoError(t, err)

		configPath := filepath.Join(subdir, "terragrunt.hcl")
		ctx := EvalContext(configPath)
		fn := ctx.Functions["get_repo_root"]

		result, err := fn.Call([]cty.Value{})
		require.NoError(t, err)
		assert.Equal(t, rootDir, result.AsString())
	})

	t.Run("without .git returns start dir", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "terragrunt.hcl")
		ctx := EvalContext(configPath)
		fn := ctx.Functions["get_repo_root"]

		result, err := fn.Call([]cty.Value{})
		require.NoError(t, err)
		assert.Equal(t, dir, result.AsString())
	})
}

func TestGetPathFromRepoRoot(t *testing.T) {
	t.Run("with .git directory", func(t *testing.T) {
		rootDir := t.TempDir()
		gitDir := filepath.Join(rootDir, ".git")
		err := os.MkdirAll(gitDir, 0755)
		require.NoError(t, err)

		subdir := filepath.Join(rootDir, "modules", "vpc")
		err = os.MkdirAll(subdir, 0755)
		require.NoError(t, err)

		configPath := filepath.Join(subdir, "terragrunt.hcl")
		ctx := EvalContext(configPath)
		fn := ctx.Functions["get_path_from_repo_root"]

		result, err := fn.Call([]cty.Value{})
		require.NoError(t, err)
		assert.Equal(t, "modules/vpc", result.AsString())
	})

	t.Run("without .git returns empty", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "terragrunt.hcl")
		ctx := EvalContext(configPath)
		fn := ctx.Functions["get_path_from_repo_root"]

		result, err := fn.Call([]cty.Value{})
		require.NoError(t, err)
		assert.Equal(t, "", result.AsString())
	})
}

func TestGetPathToRepoRoot(t *testing.T) {
	t.Run("with .git directory returns absolute path", func(t *testing.T) {
		rootDir := t.TempDir()
		gitDir := filepath.Join(rootDir, ".git")
		err := os.MkdirAll(gitDir, 0755)
		require.NoError(t, err)

		subdir := filepath.Join(rootDir, "modules", "vpc")
		err = os.MkdirAll(subdir, 0755)
		require.NoError(t, err)

		configPath := filepath.Join(subdir, "terragrunt.hcl")
		ctx := EvalContext(configPath)
		fn := ctx.Functions["get_path_to_repo_root"]

		result, err := fn.Call([]cty.Value{})
		require.NoError(t, err)
		assert.Equal(t, rootDir, result.AsString())
	})

	t.Run("from repo root returns root dir", func(t *testing.T) {
		rootDir := t.TempDir()
		gitDir := filepath.Join(rootDir, ".git")
		err := os.MkdirAll(gitDir, 0755)
		require.NoError(t, err)

		configPath := filepath.Join(rootDir, "terragrunt.hcl")
		ctx := EvalContext(configPath)
		fn := ctx.Functions["get_path_to_repo_root"]

		result, err := fn.Call([]cty.Value{})
		require.NoError(t, err)
		assert.Equal(t, rootDir, result.AsString())
	})

	t.Run("without .git directory returns start dir", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "terragrunt.hcl")
		ctx := EvalContext(configPath)
		fn := ctx.Functions["get_path_to_repo_root"]

		result, err := fn.Call([]cty.Value{})
		require.NoError(t, err)
		assert.Equal(t, dir, result.AsString())
	})

	t.Run("deep nesting returns root dir", func(t *testing.T) {
		rootDir := t.TempDir()
		gitDir := filepath.Join(rootDir, ".git")
		err := os.MkdirAll(gitDir, 0755)
		require.NoError(t, err)

		deepPath := filepath.Join(rootDir, "a", "b", "c", "d")
		err = os.MkdirAll(deepPath, 0755)
		require.NoError(t, err)

		configPath := filepath.Join(deepPath, "terragrunt.hcl")
		ctx := EvalContext(configPath)
		fn := ctx.Functions["get_path_to_repo_root"]

		result, err := fn.Call([]cty.Value{})
		require.NoError(t, err)
		assert.Equal(t, rootDir, result.AsString())
	})
}

func TestPathRelativeToInclude(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "terragrunt.hcl")

	ctx := EvalContext(configPath)
	fn := ctx.Functions["path_relative_to_include"]

	result, err := fn.Call([]cty.Value{})
	require.NoError(t, err)
	assert.Equal(t, ".", result.AsString())
}

func TestGetTerraformCommandsThatNeedVars(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "terragrunt.hcl")

	ctx := EvalContext(configPath)
	fn := ctx.Functions["get_terraform_commands_that_need_vars"]

	result, err := fn.Call([]cty.Value{})
	require.NoError(t, err)

	resultList := result.AsValueSlice()
	expectedCommands := []string{"apply", "console", "destroy", "import", "plan", "push", "refresh"}
	require.Equal(t, len(expectedCommands), len(resultList))

	for i, cmd := range expectedCommands {
		assert.Equal(t, cmd, resultList[i].AsString())
	}
}

func TestGetTerraformCommandsThatNeedInput(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "terragrunt.hcl")

	ctx := EvalContext(configPath)
	fn := ctx.Functions["get_terraform_commands_that_need_input"]

	result, err := fn.Call([]cty.Value{})
	require.NoError(t, err)

	resultList := result.AsValueSlice()
	expectedCommands := []string{"apply", "import", "init", "plan", "refresh"}
	require.Equal(t, len(expectedCommands), len(resultList))

	for i, cmd := range expectedCommands {
		assert.Equal(t, cmd, resultList[i].AsString())
	}
}

func TestGetTerraformCommandsThatNeedLocking(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "terragrunt.hcl")

	ctx := EvalContext(configPath)
	fn := ctx.Functions["get_terraform_commands_that_need_locking"]

	result, err := fn.Call([]cty.Value{})
	require.NoError(t, err)

	resultList := result.AsValueSlice()
	expectedCommands := []string{"apply", "destroy", "import", "init", "plan", "refresh", "taint", "untaint"}
	require.Equal(t, len(expectedCommands), len(resultList))

	for i, cmd := range expectedCommands {
		assert.Equal(t, cmd, resultList[i].AsString())
	}
}

func TestGetTerraformCommandsThatNeedParallelism(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "terragrunt.hcl")

	ctx := EvalContext(configPath)
	fn := ctx.Functions["get_terraform_commands_that_need_parallelism"]

	result, err := fn.Call([]cty.Value{})
	require.NoError(t, err)

	resultList := result.AsValueSlice()
	expectedCommands := []string{"apply", "plan", "destroy"}
	require.Equal(t, len(expectedCommands), len(resultList))

	for i, cmd := range expectedCommands {
		assert.Equal(t, cmd, resultList[i].AsString())
	}
}

func TestGetDefaultRetryableErrors(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "terragrunt.hcl")

	ctx := EvalContext(configPath)
	fn := ctx.Functions["get_default_retryable_errors"]

	result, err := fn.Call([]cty.Value{})
	require.NoError(t, err)

	resultList := result.AsValueSlice()
	assert.Greater(t, len(resultList), 0)
	assert.Contains(t, []string{"RequestLimitExceeded", "RequestError", "ProvisionedThroughputExceededException", "ThrottlingException", "Throttled", "Rate exceeded", "error: resource temporarily unavailable"}, resultList[0].AsString())
}

func TestGetPlatform(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "terragrunt.hcl")

	ctx := EvalContext(configPath)
	fn := ctx.Functions["get_platform"]

	result, err := fn.Call([]cty.Value{})
	require.NoError(t, err)

	platform := result.AsString()
	assert.NotEmpty(t, platform)
	assert.True(t, platform == "linux" || platform == "darwin" || platform == "windows")
}

func TestGetWorkingDir(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "terragrunt.hcl")

	ctx := EvalContext(configPath)
	fn := ctx.Functions["get_working_dir"]

	result, err := fn.Call([]cty.Value{})
	require.NoError(t, err)

	wd := result.AsString()
	assert.NotEmpty(t, wd)
}

func TestPathRelativeFromInclude(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "terragrunt.hcl")

	ctx := EvalContext(configPath)
	fn := ctx.Functions["path_relative_from_include"]

	result, err := fn.Call([]cty.Value{})
	require.NoError(t, err)
	assert.Equal(t, ".", result.AsString())
}

func TestGetParentTerragruntDir(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "terragrunt.hcl")

	ctx := EvalContext(configPath)
	fn := ctx.Functions["get_parent_terragrunt_dir"]

	result, err := fn.Call([]cty.Value{})
	require.NoError(t, err)
	assert.Equal(t, dir, result.AsString())
}

func TestGetOriginalTerragruntDir(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "terragrunt.hcl")

	ctx := EvalContext(configPath)
	fn := ctx.Functions["get_original_terragrunt_dir"]

	result, err := fn.Call([]cty.Value{})
	require.NoError(t, err)
	assert.Equal(t, dir, result.AsString())
}

func TestReadTfvarsFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "terragrunt.hcl")

	ctx := EvalContext(configPath)
	fn := ctx.Functions["read_tfvars_file"]

	t.Run("empty path returns empty object", func(t *testing.T) {
		result, err := fn.Call([]cty.Value{cty.StringVal("")})
		require.NoError(t, err)
		assert.Equal(t, cty.EmptyObjectVal, result)
	})

	t.Run("nonexistent file returns empty object", func(t *testing.T) {
		result, err := fn.Call([]cty.Value{cty.StringVal("/nonexistent/path.tfvars")})
		require.NoError(t, err)
		assert.Equal(t, cty.EmptyObjectVal, result)
	})

	t.Run("parses valid HCL file", func(t *testing.T) {
		tfvarsFile := filepath.Join(dir, "test.tfvars")
		content := `region = "us-west-2"
environment = "test-env"`
		err := os.WriteFile(tfvarsFile, []byte(content), 0644)
		require.NoError(t, err)

		result, err := fn.Call([]cty.Value{cty.StringVal(tfvarsFile)})
		require.NoError(t, err)
		assert.NotEqual(t, cty.EmptyObjectVal, result)
	})
}

func TestGetAwsAccountId(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "terragrunt.hcl")

	ctx := EvalContext(configPath)
	fn := ctx.Functions["get_aws_account_id"]

	result, err := fn.Call([]cty.Value{})
	require.NoError(t, err)
	assert.Equal(t, "000000000000", result.AsString())
}

func TestGetAwsAccountAlias(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "terragrunt.hcl")

	ctx := EvalContext(configPath)
	fn := ctx.Functions["get_aws_account_alias"]

	result, err := fn.Call([]cty.Value{})
	require.NoError(t, err)
	assert.Equal(t, "lint-placeholder", result.AsString())
}

func TestGetAwsCallerIdentityArn(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "terragrunt.hcl")

	ctx := EvalContext(configPath)
	fn := ctx.Functions["get_aws_caller_identity_arn"]

	result, err := fn.Call([]cty.Value{})
	require.NoError(t, err)
	assert.Equal(t, "arn:aws:iam::000000000000:user/lint-placeholder", result.AsString())
}

func TestGetAwsCallerIdentityUserId(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "terragrunt.hcl")

	ctx := EvalContext(configPath)
	fn := ctx.Functions["get_aws_caller_identity_user_id"]

	result, err := fn.Call([]cty.Value{})
	require.NoError(t, err)
	assert.Equal(t, "LINT000000000000000", result.AsString())
}

func TestRunCmd(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "terragrunt.hcl")

	ctx := EvalContext(configPath)
	fn := ctx.Functions["run_cmd"]

	t.Run("with no args", func(t *testing.T) {
		result, err := fn.Call([]cty.Value{})
		require.NoError(t, err)
		assert.Equal(t, "", result.AsString())
	})

	t.Run("with args", func(t *testing.T) {
		result, err := fn.Call([]cty.Value{
			cty.StringVal("echo"),
			cty.StringVal("hello"),
		})
		require.NoError(t, err)
		assert.Equal(t, "", result.AsString())
	})
}

func TestSopsDecryptFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "terragrunt.hcl")

	ctx := EvalContext(configPath)
	fn := ctx.Functions["sops_decrypt_file"]

	result, err := fn.Call([]cty.Value{cty.StringVal("/path/to/encrypted.yaml")})
	require.NoError(t, err)
	assert.Equal(t, "", result.AsString())
}

func TestGetTerraformCommand(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "terragrunt.hcl")

	ctx := EvalContext(configPath)
	fn := ctx.Functions["get_terraform_command"]

	result, err := fn.Call([]cty.Value{})
	require.NoError(t, err)
	assert.Equal(t, "plan", result.AsString())
}

func TestGetTerraformCliArgs(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "terragrunt.hcl")

	ctx := EvalContext(configPath)
	fn := ctx.Functions["get_terraform_cli_args"]

	result, err := fn.Call([]cty.Value{})
	require.NoError(t, err)

	resultList := result.AsValueSlice()
	assert.Equal(t, 0, len(resultList))
}

func TestGetTerragruntSourceCliFlag(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "terragrunt.hcl")

	ctx := EvalContext(configPath)
	fn := ctx.Functions["get_terragrunt_source_cli_flag"]

	result, err := fn.Call([]cty.Value{})
	require.NoError(t, err)
	assert.Equal(t, "", result.AsString())
}

func TestConstraintCheck(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "terragrunt.hcl")

	ctx := EvalContext(configPath)
	fn := ctx.Functions["constraint_check"]

	t.Run("with no args", func(t *testing.T) {
		result, err := fn.Call([]cty.Value{})
		require.NoError(t, err)
		assert.Equal(t, true, result.True())
	})

	t.Run("with args", func(t *testing.T) {
		result, err := fn.Call([]cty.Value{
			cty.StringVal("constraint"),
			cty.StringVal("requirement"),
		})
		require.NoError(t, err)
		assert.Equal(t, true, result.True())
	})
}

func TestMarkAsRead(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "terragrunt.hcl")

	ctx := EvalContext(configPath)
	fn := ctx.Functions["mark_as_read"]

	t.Run("returns string unchanged", func(t *testing.T) {
		result, err := fn.Call([]cty.Value{cty.StringVal("test-value")})
		require.NoError(t, err)
		assert.Equal(t, "test-value", result.AsString())
	})

	t.Run("returns number unchanged", func(t *testing.T) {
		result, err := fn.Call([]cty.Value{cty.NumberIntVal(42)})
		require.NoError(t, err)
		resultInt, _ := result.AsBigFloat().Int64()
		assert.Equal(t, int64(42), resultInt)
	})

	t.Run("returns bool unchanged", func(t *testing.T) {
		result, err := fn.Call([]cty.Value{cty.True})
		require.NoError(t, err)
		assert.Equal(t, true, result.True())
	})
}

func TestEvalContextHasAllFunctions(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "terragrunt.hcl")

	ctx := EvalContext(configPath)

	requiredFuncs := []string{
		"get_terragrunt_dir",
		"get_repo_root",
		"get_path_to_repo_root",
		"get_env",
		"get_path_from_repo_root",
		"find_in_parent_folders",
		"basename",
		"replace",
		"path_relative_to_include",
		"read_terragrunt_config",
		"get_terraform_commands_that_need_vars",
		"get_terraform_commands_that_need_input",
		"get_terraform_commands_that_need_locking",
		"get_terraform_commands_that_need_parallelism",
		"get_default_retryable_errors",
		"get_platform",
		"get_working_dir",
		"path_relative_from_include",
		"get_parent_terragrunt_dir",
		"get_original_terragrunt_dir",
		"read_tfvars_file",
		"get_aws_account_id",
		"get_aws_account_alias",
		"get_aws_caller_identity_arn",
		"get_aws_caller_identity_user_id",
		"run_cmd",
		"sops_decrypt_file",
		"get_terraform_command",
		"get_terraform_cli_args",
		"get_terragrunt_source_cli_flag",
		"constraint_check",
		"mark_as_read",
	}

	for _, fn := range requiredFuncs {
		assert.Contains(t, ctx.Functions, fn, "missing function: %s", fn)
	}

	// Should have exactly the required functions
	assert.Equal(t, len(requiredFuncs), len(ctx.Functions), "function count mismatch")
}
