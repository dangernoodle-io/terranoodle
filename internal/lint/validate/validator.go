package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"dangernoodle.io/terranoodle/internal/hclutils"
	"dangernoodle.io/terranoodle/internal/hclutils/tfmod"
	"dangernoodle.io/terranoodle/internal/hclutils/tftype"
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

// ErrorKind categorizes a validation error.
type ErrorKind int

const (
	MissingRequired ErrorKind = iota
	ExtraInput
	TypeMismatch // Phase 5
)

func (k ErrorKind) String() string {
	switch k {
	case MissingRequired:
		return "missing required input"
	case ExtraInput:
		return "extra input"
	case TypeMismatch:
		return "type mismatch"
	default:
		return "unknown"
	}
}

// Error represents a single validation finding.
type Error struct {
	File     string
	Variable string
	Kind     ErrorKind
	Detail   string
}

// tfVarEnvKeys returns a map of variable names parsed from TF_VAR_* environment variables.
// For each TF_VAR_foo=bar in the environment, the key "foo" is added to the map with value true.
func tfVarEnvKeys() map[string]bool {
	keys := make(map[string]bool)
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "TF_VAR_") {
			// Extract the variable name (everything after "TF_VAR_" up to "=")
			rest := strings.TrimPrefix(env, "TF_VAR_")
			if idx := strings.IndexByte(rest, '='); idx > 0 {
				varName := rest[:idx]
				keys[varName] = true
			}
		}
	}
	return keys
}

// File validates a single terragrunt.hcl file.
func File(path string) ([]Error, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolving path %s: %w", path, err)
	}

	cfg, err := hclutils.ParseFile(absPath)
	if err != nil {
		return nil, err
	}

	if cfg.Source == "" {
		// Can't validate without a resolvable source (remote sources handled in Phase 2)
		return nil, nil
	}

	modulePath := hclutils.ResolveSource(cfg.Source, absPath)
	if modulePath == "" {
		if hclutils.IsRemoteSource(cfg.Source) {
			return nil, fmt.Errorf("cannot resolve remote source %q — run 'terragrunt init' first to populate .terragrunt-cache/", cfg.Source)
		}
		return nil, nil
	}

	moduleDir, err := tfmod.ResolveModuleDir(modulePath)
	if err != nil {
		return nil, err
	}

	variables, err := tfmod.ParseVariables(moduleDir)
	if err != nil {
		return nil, err
	}

	depOutputKeys := resolveDepExemptions(cfg)
	envVarKeys := tfVarEnvKeys()

	return check(absPath, cfg.Inputs, variables, depOutputKeys, envVarKeys, cfg.EvalCtx), nil
}

// StackFile validates a terragrunt.stack.hcl file by checking each unit.
func StackFile(path string) ([]Error, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolving path %s: %w", path, err)
	}

	stack, err := hclutils.ParseStackFile(absPath)
	if err != nil {
		return nil, err
	}

	envVarKeys := tfVarEnvKeys()

	var allErrors []Error
	for _, unit := range stack.Units {
		if unit.Source == "" {
			continue
		}

		modulePath := hclutils.ResolveSource(unit.Source, absPath)
		if modulePath == "" {
			continue
		}

		moduleDir, err := tfmod.ResolveModuleDir(modulePath)
		if err != nil {
			continue // skip unresolvable units
		}

		variables, err := tfmod.ParseVariables(moduleDir)
		if err != nil {
			continue
		}

		// Stack units don't use merge(dependency.outputs) so no dep exemptions
		unitErrors := check(absPath, unit.Values, variables, nil, envVarKeys, unit.EvalCtx)
		// Tag errors with unit name for clarity
		for i := range unitErrors {
			unitErrors[i].Detail = fmt.Sprintf("[unit %q] %s", unit.Name, unitErrors[i].Detail)
		}
		allErrors = append(allErrors, unitErrors...)
	}

	return allErrors, nil
}

// resolveDepExemptions builds the set of input keys that are exempt from the
// extra-input check because they originate from dependency outputs in a merge().
func resolveDepExemptions(cfg *hclutils.TerragruntConfig) map[string]bool {
	if len(cfg.DepRefs) == 0 {
		return nil
	}

	depMap := make(map[string]hclutils.DependencyConfig, len(cfg.Dependencies))
	for _, d := range cfg.Dependencies {
		depMap[d.Name] = d
	}

	exempt := make(map[string]bool)

	for _, depName := range cfg.DepRefs {
		dep, ok := depMap[depName]
		if !ok {
			continue
		}

		depCfgFile := filepath.Join(dep.ConfigPath, "terragrunt.hcl")
		depCfg, err := hclutils.ParseFile(depCfgFile)
		if err != nil || depCfg.Source == "" {
			continue
		}

		depModulePath := hclutils.ResolveSource(depCfg.Source, depCfgFile)
		if depModulePath == "" {
			continue
		}

		depModuleDir, err := tfmod.ResolveModuleDir(depModulePath)
		if err != nil {
			continue
		}

		outputs, err := tfmod.ParseOutputs(depModuleDir)
		if err != nil {
			continue
		}

		for _, o := range outputs {
			exempt[o] = true
		}
	}

	return exempt
}

// TerraformDir validates all module blocks in .tf files within a directory.
func TerraformDir(dir string) ([]Error, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolving path %s: %w", dir, err)
	}

	calls, err := hclutils.ParseModuleCalls(absDir)
	if err != nil {
		return nil, err
	}

	envVarKeys := tfVarEnvKeys()

	var allErrors []Error
	for _, mc := range calls {
		modulePath := hclutils.ResolveSource(mc.Source, filepath.Join(absDir, "main.tf"))
		if modulePath == "" {
			continue // remote/registry source — skip
		}

		moduleDir, err := tfmod.ResolveModuleDir(modulePath)
		if err != nil {
			continue // unresolvable module — skip
		}

		variables, err := tfmod.ParseVariables(moduleDir)
		if err != nil {
			continue
		}

		mcErrors := check(absDir, mc.Inputs, variables, nil, envVarKeys, mc.EvalCtx)
		for i := range mcErrors {
			mcErrors[i].Detail = fmt.Sprintf("[module %q] %s", mc.Name, mcErrors[i].Detail)
		}
		allErrors = append(allErrors, mcErrors...)
	}

	return allErrors, nil
}

func check(file string, inputs map[string]hcl.Expression, variables []tfmod.Variable, depOutputKeys map[string]bool, envVarKeys map[string]bool, evalCtx *hcl.EvalContext) []Error {
	// Build lookup sets.
	// Dep output keys count as provided (they satisfy required variables AND
	// are exempt from extra-input errors — Terraform silently ignores them).
	// Env var keys count as provided (they satisfy required variables but are NOT
	// exempt from extra-input errors — they are not explicit inputs).
	inputKeys := make(map[string]bool, len(inputs)+len(depOutputKeys)+len(envVarKeys))
	for k := range inputs {
		inputKeys[k] = true
	}
	for k := range depOutputKeys {
		inputKeys[k] = true
	}
	for k := range envVarKeys {
		inputKeys[k] = true
	}

	varMap := make(map[string]tfmod.Variable, len(variables))
	for _, v := range variables {
		varMap[v.Name] = v
	}

	var errs []Error

	// Check for missing required inputs
	for _, v := range variables {
		if !v.HasDefault && !inputKeys[v.Name] {
			errs = append(errs, Error{
				File:     file,
				Variable: v.Name,
				Kind:     MissingRequired,
				Detail:   fmt.Sprintf("variable %q is required (no default) but not provided in inputs", v.Name),
			})
		}
	}

	// Check for extra inputs — dep output keys are exempt even if no matching variable
	extraKeys := make([]string, 0)
	for k := range inputs {
		if _, ok := varMap[k]; !ok && !depOutputKeys[k] {
			extraKeys = append(extraKeys, k)
		}
	}
	sort.Strings(extraKeys)

	for _, k := range extraKeys {
		errs = append(errs, Error{
			File:     file,
			Variable: k,
			Kind:     ExtraInput,
			Detail:   fmt.Sprintf("input %q has no matching variable in module", k),
		})
	}

	// Check for type mismatches
	var typeKeys []string
	for k := range inputs {
		if _, ok := varMap[k]; ok {
			typeKeys = append(typeKeys, k)
		}
	}
	sort.Strings(typeKeys)

	for _, k := range typeKeys {
		v := varMap[k]
		if v.Type == nil {
			continue // no type constraint → any
		}

		constraintType, err := tftype.ParseConstraint(v.Type)
		if err != nil || constraintType == cty.DynamicPseudoType {
			continue
		}

		val, diags := inputs[k].Value(evalCtx)
		if diags.HasErrors() {
			continue // dynamic/unresolvable expression — skip
		}
		if val.Type() == cty.DynamicPseudoType || !val.IsKnown() {
			continue
		}

		typeErrs := tftype.ExtraAttrs(val.Type(), constraintType)
		for _, detail := range typeErrs {
			errs = append(errs, Error{
				File:     file,
				Variable: k,
				Kind:     TypeMismatch,
				Detail:   fmt.Sprintf("input %q: %s", k, detail),
			})
		}
	}

	return errs
}
