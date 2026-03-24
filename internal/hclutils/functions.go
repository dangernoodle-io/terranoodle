package hclutils

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

// EvalContext builds an hcl.EvalContext with Terragrunt-compatible functions
// scoped to the given config file path.
func EvalContext(configPath string) *hcl.EvalContext {
	configDir := filepath.Dir(configPath)

	return &hcl.EvalContext{
		Functions: map[string]function.Function{
			"get_terragrunt_dir":                           makeGetTerragruntDir(configDir),
			"get_repo_root":                                makeGetRepoRoot(configDir),
			"get_path_to_repo_root":                        makeGetPathToRepoRoot(configDir),
			"get_env":                                      makeGetEnv(),
			"get_path_from_repo_root":                      makeGetPathFromRepoRoot(configDir),
			"find_in_parent_folders":                       makeFindInParentFolders(configDir),
			"basename":                                     makeBasename(),
			"replace":                                      makeReplace(),
			"path_relative_to_include":                     makePathRelativeToInclude(configDir),
			"read_terragrunt_config":                       makeReadTerragruntConfig(),
			"get_terraform_commands_that_need_vars":        makeGetTerraformCommandsThatNeedVars(),
			"get_terraform_commands_that_need_input":       makeGetTerraformCommandsThatNeedInput(),
			"get_terraform_commands_that_need_locking":     makeGetTerraformCommandsThatNeedLocking(),
			"get_terraform_commands_that_need_parallelism": makeGetTerraformCommandsThatNeedParallelism(),
			"get_default_retryable_errors":                 makeGetDefaultRetryableErrors(),
			"get_platform":                                 makeGetPlatform(),
			"get_working_dir":                              makeGetWorkingDir(),
			"path_relative_from_include":                   makePathRelativeFromInclude(),
			"get_parent_terragrunt_dir":                    makeGetParentTerragruntDir(configDir),
			"get_original_terragrunt_dir":                  makeGetOriginalTerragruntDir(configDir),
			"read_tfvars_file":                             makeReadTfvarsFile(),
			"run_cmd":                                      makeRunCmd(),
			"sops_decrypt_file":                            makeSopsDecryptFile(),
			"get_terraform_command":                        makeGetTerraformCommand(),
			"get_terraform_cli_args":                       makeGetTerraformCliArgs(),
			"get_terragrunt_source_cli_flag":               makeGetTerragruntSourceCliFlag(),
			"get_aws_account_id":                           makeStaticStringFunc("000000000000"),
			"get_aws_account_alias":                        makeStaticStringFunc("lint-placeholder"),
			"get_aws_caller_identity_arn":                  makeStaticStringFunc("arn:aws:iam::000000000000:user/lint-placeholder"),
			"get_aws_caller_identity_user_id":              makeStaticStringFunc("LINT000000000000000"),
			"constraint_check":                             makeConstraintCheck(),
			"mark_as_read":                                 makeMarkAsRead(),
		},
	}
}

func makeGetTerragruntDir(configDir string) function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{},
		Type:   function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			return cty.StringVal(configDir), nil
		},
	})
}

func makeGetRepoRoot(startDir string) function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{},
		Type:   function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			root, _ := findRepoRoot(startDir)
			if root == "" {
				root = startDir
			}
			return cty.StringVal(root), nil
		},
	})
}

func makeGetPathToRepoRoot(startDir string) function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{},
		Type:   function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			root, _ := findRepoRoot(startDir)
			if root == "" {
				root = startDir
			}
			return cty.StringVal(root), nil
		},
	})
}

func makeGetEnv() function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{
			{Name: "name", Type: cty.String},
		},
		VarParam: &function.Parameter{Name: "default", Type: cty.String},
		Type:     function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			name := args[0].AsString()
			val := os.Getenv(name)
			if val == "" && len(args) > 1 {
				val = args[1].AsString()
			}
			return cty.StringVal(val), nil
		},
	})
}

func makeGetPathFromRepoRoot(startDir string) function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{},
		Type:   function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			root, _ := findRepoRoot(startDir)
			if root == "" {
				return cty.StringVal(""), nil
			}
			rel, _ := filepath.Rel(root, startDir)
			return cty.StringVal(rel), nil
		},
	})
}

func makeFindInParentFolders(startDir string) function.Function {
	return function.New(&function.Spec{
		VarParam: &function.Parameter{Name: "name", Type: cty.String},
		Type:     function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			name := "terragrunt.hcl"
			if len(args) > 0 {
				name = args[0].AsString()
			}

			dir := startDir
			for {
				candidate := filepath.Join(dir, name)
				if _, err := os.Stat(candidate); err == nil {
					return cty.StringVal(candidate), nil
				}

				parent := filepath.Dir(dir)
				if parent == dir {
					return cty.NilVal, fmt.Errorf(
						"find_in_parent_folders: %q not found above %s", name, startDir)
				}
				dir = parent
			}
		},
	})
}

func makeBasename() function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{
			{Name: "path", Type: cty.String},
		},
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			return cty.StringVal(filepath.Base(args[0].AsString())), nil
		},
	})
}

func makeReplace() function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{
			{Name: "str", Type: cty.String},
			{Name: "old", Type: cty.String},
			{Name: "new", Type: cty.String},
		},
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			s := args[0].AsString()
			old := args[1].AsString()
			replacement := args[2].AsString()
			return cty.StringVal(strings.ReplaceAll(s, old, replacement)), nil
		},
	})
}

// makePathRelativeToInclude returns the relative path from the parent
// config's directory to the child config's directory. In our lint context
// the child is the config being parsed, so we return "." as a safe default
// (this function is primarily used in generate blocks which we don't validate).
func makePathRelativeToInclude(_ string) function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{},
		Type:   function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			return cty.StringVal("."), nil
		},
	})
}

// makeReadTerragruntConfig parses another terragrunt config file and returns
// its locals as a cty object. This allows configs to reference values from
// other configs via `read_terragrunt_config(path).locals.*`.
func makeReadTerragruntConfig() function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{
			{Name: "config_path", Type: cty.String},
		},
		VarParam: &function.Parameter{Name: "opts", Type: cty.DynamicPseudoType},
		Type:     function.StaticReturnType(cty.DynamicPseudoType),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			path := args[0].AsString()
			if path == "" {
				return cty.EmptyObjectVal, nil
			}

			locals, _ := ResolveIncludeLocals(path)

			return cty.ObjectVal(map[string]cty.Value{
				"locals": locals,
			}), nil
		},
	})
}

func makeGetTerraformCommandsThatNeedVars() function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{},
		Type:   function.StaticReturnType(cty.List(cty.String)),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			commands := []cty.Value{
				cty.StringVal("apply"),
				cty.StringVal("console"),
				cty.StringVal("destroy"),
				cty.StringVal("import"),
				cty.StringVal("plan"),
				cty.StringVal("push"),
				cty.StringVal("refresh"),
			}
			return cty.ListVal(commands), nil
		},
	})
}

func makeGetTerraformCommandsThatNeedInput() function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{},
		Type:   function.StaticReturnType(cty.List(cty.String)),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			commands := []cty.Value{
				cty.StringVal("apply"),
				cty.StringVal("import"),
				cty.StringVal("init"),
				cty.StringVal("plan"),
				cty.StringVal("refresh"),
			}
			return cty.ListVal(commands), nil
		},
	})
}

func makeGetTerraformCommandsThatNeedLocking() function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{},
		Type:   function.StaticReturnType(cty.List(cty.String)),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			commands := []cty.Value{
				cty.StringVal("apply"),
				cty.StringVal("destroy"),
				cty.StringVal("import"),
				cty.StringVal("init"),
				cty.StringVal("plan"),
				cty.StringVal("refresh"),
				cty.StringVal("taint"),
				cty.StringVal("untaint"),
			}
			return cty.ListVal(commands), nil
		},
	})
}

func makeGetTerraformCommandsThatNeedParallelism() function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{},
		Type:   function.StaticReturnType(cty.List(cty.String)),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			commands := []cty.Value{
				cty.StringVal("apply"),
				cty.StringVal("plan"),
				cty.StringVal("destroy"),
			}
			return cty.ListVal(commands), nil
		},
	})
}

func makeGetDefaultRetryableErrors() function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{},
		Type:   function.StaticReturnType(cty.List(cty.String)),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			errors := []cty.Value{
				cty.StringVal("RequestLimitExceeded"),
				cty.StringVal("RequestError"),
				cty.StringVal("ProvisionedThroughputExceededException"),
				cty.StringVal("ThrottlingException"),
				cty.StringVal("Throttled"),
				cty.StringVal("Rate exceeded"),
				cty.StringVal("error: resource temporarily unavailable"),
			}
			return cty.ListVal(errors), nil
		},
	})
}

func makeGetPlatform() function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{},
		Type:   function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			return cty.StringVal(runtime.GOOS), nil
		},
	})
}

func makeGetWorkingDir() function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{},
		Type:   function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			wd, err := os.Getwd()
			if err != nil {
				return cty.StringVal(""), err
			}
			return cty.StringVal(wd), nil
		},
	})
}

func makePathRelativeFromInclude() function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{},
		Type:   function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			return cty.StringVal("."), nil
		},
	})
}

func makeGetParentTerragruntDir(configDir string) function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{},
		Type:   function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			return cty.StringVal(configDir), nil
		},
	})
}

func makeGetOriginalTerragruntDir(configDir string) function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{},
		Type:   function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			return cty.StringVal(configDir), nil
		},
	})
}

func makeReadTfvarsFile() function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{
			{Name: "path", Type: cty.String},
		},
		Type: function.StaticReturnType(cty.DynamicPseudoType),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			path := args[0].AsString()
			if path == "" {
				return cty.EmptyObjectVal, nil
			}

			// Try to read and parse the file as HCL
			content, err := os.ReadFile(path)
			if err != nil {
				return cty.EmptyObjectVal, nil //nolint:nilerr
			}

			// Parse as HCL
			file, diags := hclsyntax.ParseConfig(content, path, hcl.Pos{Line: 1, Column: 1})
			if diags.HasErrors() {
				return cty.EmptyObjectVal, nil //nolint:nilerr
			}

			// Evaluate all attributes and build object value
			ctx := &hcl.EvalContext{
				Variables: make(map[string]cty.Value),
				Functions: make(map[string]function.Function),
			}

			body, ok := file.Body.(*hclsyntax.Body)
			if !ok {
				return cty.EmptyObjectVal, nil
			}
			attrs, diags := body.JustAttributes()
			if diags.HasErrors() {
				return cty.EmptyObjectVal, nil
			}

			objAttrs := make(map[string]cty.Value)
			for _, attr := range attrs {
				val, diags := attr.Expr.Value(ctx)
				if !diags.HasErrors() {
					objAttrs[attr.Name] = val
				}
			}

			return cty.ObjectVal(objAttrs), nil
		},
	})
}

func makeRunCmd() function.Function {
	return function.New(&function.Spec{
		VarParam: &function.Parameter{Name: "args", Type: cty.String},
		Type:     function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			return cty.StringVal(""), nil
		},
	})
}

func makeSopsDecryptFile() function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{
			{Name: "path", Type: cty.String},
		},
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			return cty.StringVal(""), nil
		},
	})
}

func makeGetTerraformCommand() function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{},
		Type:   function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			return cty.StringVal("plan"), nil
		},
	})
}

func makeGetTerraformCliArgs() function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{},
		Type:   function.StaticReturnType(cty.List(cty.String)),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			return cty.ListValEmpty(cty.String), nil
		},
	})
}

func makeGetTerragruntSourceCliFlag() function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{},
		Type:   function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			return cty.StringVal(""), nil
		},
	})
}

// makeStaticStringFunc returns a function that takes no parameters and returns
// a static string value. Used for AWS stub functions and other placeholders.
func makeStaticStringFunc(value string) function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{},
		Type:   function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			return cty.StringVal(value), nil
		},
	})
}

// makeConstraintCheck returns a function that accepts variadic arguments
// and always returns true. It's used for constraint validation stubs.
func makeConstraintCheck() function.Function {
	return function.New(&function.Spec{
		VarParam: &function.Parameter{Name: "args", Type: cty.DynamicPseudoType},
		Type:     function.StaticReturnType(cty.Bool),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			return cty.True, nil
		},
	})
}

// makeMarkAsRead returns a function that accepts a single value and returns
// it unchanged. Used as a pass-through for value marking in configs.
func makeMarkAsRead() function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{
			{Name: "value", Type: cty.DynamicPseudoType},
		},
		Type: func(args []cty.Value) (cty.Type, error) {
			return args[0].Type(), nil
		},
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			return args[0], nil
		},
	})
}

// findRepoRoot walks up from startDir looking for a .git directory.
func findRepoRoot(startDir string) (string, error) {
	dir := startDir
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}
