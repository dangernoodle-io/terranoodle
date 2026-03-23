package hclutils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

// EvalContext builds an hcl.EvalContext with Terragrunt-compatible functions
// scoped to the given config file path.
func EvalContext(configPath string) *hcl.EvalContext {
	configDir := filepath.Dir(configPath)

	return &hcl.EvalContext{
		Functions: map[string]function.Function{
			"get_terragrunt_dir":       makeGetTerragruntDir(configDir),
			"get_repo_root":            makeGetRepoRoot(configDir),
			"get_env":                  makeGetEnv(),
			"get_path_from_repo_root":  makeGetPathFromRepoRoot(configDir),
			"find_in_parent_folders":   makeFindInParentFolders(configDir),
			"basename":                 makeBasename(),
			"replace":                  makeReplace(),
			"path_relative_to_include": makePathRelativeToInclude(configDir),
			"read_terragrunt_config":   makeReadTerragruntConfig(),
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
