package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"dangernoodle.io/terranoodle/internal/hclutils"
	"dangernoodle.io/terranoodle/internal/hclutils/tfmod"
	"dangernoodle.io/terranoodle/internal/hclutils/tftype"
	goversion "github.com/hashicorp/go-version"
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

// Severity indicates the severity level of a validation finding.
type Severity int

const (
	SeverityError Severity = iota
	SeverityWarning
)

// ErrorKind categorizes a validation error.
type ErrorKind int

const (
	MissingRequired ErrorKind = iota
	ExtraInput
	TypeMismatch // Phase 5
	SourceRefSemver
	SourceProtocol
	MissingDescription
	NonSnakeCase
	UnusedVariable
	OptionalWithoutDefault
	MissingIncludeExpose
	DisallowedFilename
	MissingVersionsTF
	MissingTerraformBlock
	MissingProviderSource
	MissingProviderVersion
	DuplicateProvider
	NoProviderBlock
	SetStringType
	ProviderConstraintStyle
	EmptyOutputsTF
	VersionsTFNotSymlink
	MissingValidation
	SensitiveOutput
)

func (k ErrorKind) String() string {
	switch k {
	case MissingRequired:
		return "missing required input"
	case ExtraInput:
		return "extra input"
	case TypeMismatch:
		return "type mismatch"
	case SourceRefSemver:
		return "non-semver source ref"
	case SourceProtocol:
		return "disallowed source protocol"
	case MissingDescription:
		return "missing description"
	case NonSnakeCase:
		return "non-snake-case name"
	case UnusedVariable:
		return "UnusedVariable"
	case OptionalWithoutDefault:
		return "OptionalWithoutDefault"
	case MissingIncludeExpose:
		return "MissingIncludeExpose"
	case DisallowedFilename:
		return "disallowed filename"
	case MissingVersionsTF:
		return "missing versions.tf"
	case MissingTerraformBlock:
		return "missing terraform block"
	case MissingProviderSource:
		return "missing provider source"
	case MissingProviderVersion:
		return "missing provider version"
	case DuplicateProvider:
		return "duplicate provider"
	case NoProviderBlock:
		return "provider block in terragrunt config"
	case SetStringType:
		return "set(string) type usage"
	case ProviderConstraintStyle:
		return "provider constraint style"
	case EmptyOutputsTF:
		return "empty outputs.tf"
	case VersionsTFNotSymlink:
		return "versions.tf not symlinked to root"
	case MissingValidation:
		return "missing validation block"
	case SensitiveOutput:
		return "sensitive output without sensitive flag"
	default:
		return "unknown"
	}
}

var snakeCaseRe = regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)*$`)
var shaPattern = regexp.MustCompile(`^[0-9a-fA-F]{7,40}$`)

// findModuleRoot walks up from dir to find the topmost ancestor containing .tf files.
func findModuleRoot(dir string) string {
	root := dir
	current := dir
	for {
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		if hclutils.HasTFFiles(parent) {
			root = parent
		}
		current = parent
	}
	return root
}

// isSHA reports whether ref is a commit SHA (7-40 hex characters).
func isSHA(ref string) bool {
	return shaPattern.MatchString(ref)
}

// getSHAOption reads the "sha" option from source-ref-semver rule's config.
// Returns "warn" if sha is "warn" or true (bool), "error" otherwise or if not set.
func getSHAOption(opts Options) string {
	if opts.Config == nil {
		return "error"
	}
	rule, ok := opts.Config.Rules["source-ref-semver"]
	if !ok {
		return "error"
	}
	raw, ok := rule.Options["sha"]
	if !ok {
		return "error"
	}
	switch v := raw.(type) {
	case string:
		if v == "warn" || v == "true" {
			return "warn"
		}
		return "error"
	case bool:
		if v {
			return "warn"
		}
		return "error"
	default:
		return "error"
	}
}

// Error represents a single validation finding.
type Error struct {
	File     string
	Variable string
	Kind     ErrorKind
	Detail   string
	Severity Severity
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

// checkSourceRef validates that remote source refs are semver versions.
// Returns nil for local sources, tfr:// sources, or sources without refs.
// Returns an error (downgraded to warning if matched by allow list) for non-semver refs.
func checkSourceRef(source, file string, opts Options) []Error {
	if !hclutils.IsRemoteSource(source) || strings.HasPrefix(source, "tfr://") {
		return nil
	}
	_, _, ref := hclutils.StripSubdir(source)
	if ref == "" {
		return nil
	}
	if _, err := goversion.NewVersion(ref); err == nil {
		return nil
	}

	// SHA detection — controlled by "sha" option
	if isSHA(ref) {
		sev := SeverityError
		shaOpt := getSHAOption(opts)
		if shaOpt == "warn" {
			sev = SeverityWarning
		}
		return []Error{{
			File:     file,
			Kind:     SourceRefSemver,
			Severity: sev,
			Detail:   fmt.Sprintf("source pinned to commit SHA %q, use a semver tag", ref),
		}}
	}

	// Branch/tag — check allow list
	sev := SeverityError
	for _, pattern := range getAllowPatterns(opts, "source-ref-semver") {
		if matched, _ := filepath.Match(pattern, ref); matched {
			sev = SeverityWarning
			break
		}
	}
	return []Error{{
		File:     file,
		Kind:     SourceRefSemver,
		Severity: sev,
		Detail:   fmt.Sprintf("source ref %q is not a semver version", ref),
	}}
}

// isSSHSource reports whether a remote source uses SSH transport.
func isSSHSource(source string) bool {
	base, ok := strings.CutPrefix(source, "git::")
	if !ok {
		base = source
	}
	return strings.Contains(base, "git@")
}

// checkSourceProtocol enforces the transport protocol used in remote source URLs.
func checkSourceProtocol(source, file string, opts Options) []Error {
	if !hclutils.IsRemoteSource(source) {
		return nil
	}
	if strings.HasPrefix(source, "tfr://") || strings.HasPrefix(source, "s3://") {
		return nil
	}

	enforce := getEnforceOption(opts, "source-protocol")
	switch enforce {
	case "https":
		if isSSHSource(source) {
			return []Error{{
				File:     file,
				Kind:     SourceProtocol,
				Severity: SeverityError,
				Detail:   fmt.Sprintf("source %q uses SSH transport; HTTPS is required", source),
			}}
		}
	case "ssh":
		if !isSSHSource(source) {
			return []Error{{
				File:     file,
				Kind:     SourceProtocol,
				Severity: SeverityError,
				Detail:   fmt.Sprintf("source %q uses HTTPS transport; SSH is required", source),
			}}
		}
	}
	return nil
}

// applyAllowList downgrades ExtraInput errors to warnings if the variable matches an allow pattern.
func applyAllowList(errs []Error, opts Options) []Error {
	patterns := getAllowPatterns(opts, "extra-inputs")
	if len(patterns) == 0 {
		return errs
	}
	for i := range errs {
		if errs[i].Kind != ExtraInput {
			continue
		}
		for _, p := range patterns {
			if matched, _ := filepath.Match(p, errs[i].Variable); matched {
				errs[i].Severity = SeverityWarning
				break
			}
		}
	}
	return errs
}

// File validates a single terragrunt.hcl file.
func File(path string, opts ...Options) ([]Error, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolving path %s: %w", path, err)
	}

	cfg, err := hclutils.ParseFile(absPath)
	if err != nil {
		return nil, err
	}

	var opt Options
	if len(opts) > 0 {
		opt = opts[0]
	}

	// Check include expose before source-dependent checks
	var includeErrs []Error
	checkIncludeExpose := opt.Config != nil && opt.Config.IsRuleEnabled("missing-include-expose", absPath)

	if checkIncludeExpose {
		excludePatterns := getExcludePatterns(opt, "missing-include-expose")
		for _, inc := range cfg.Includes {
			if inc.Expose {
				continue
			}
			excluded := false
			for _, p := range excludePatterns {
				if matched, _ := filepath.Match(p, inc.Name); matched {
					excluded = true
					break
				}
			}
			if excluded {
				continue
			}
			includeErrs = append(includeErrs, Error{
				File:     absPath,
				Variable: inc.Name,
				Kind:     MissingIncludeExpose,
				Detail:   fmt.Sprintf("include %q is missing expose = true", inc.Name),
			})
		}
	}

	// Check for provider blocks
	checkNoProviderBlock := opt.Config != nil && opt.Config.IsRuleEnabled("no-tg-provider-blocks", absPath)
	if checkNoProviderBlock {
		for _, name := range cfg.ProviderBlockNames {
			includeErrs = append(includeErrs, Error{
				File:     absPath,
				Variable: name,
				Kind:     NoProviderBlock,
				Detail:   fmt.Sprintf("provider %q block should not be in terragrunt config — use a generate block or define in the module", name),
			})
		}
	}

	if cfg.Source == "" {
		// Can't validate without a resolvable source (remote sources handled in Phase 2)
		if len(includeErrs) > 0 {
			return filterErrors(includeErrs, opt), nil
		}
		return nil, nil
	}

	var results []Error
	results = append(results, includeErrs...)
	results = append(results, checkSourceRef(cfg.Source, absPath, opt)...)
	results = append(results, checkSourceProtocol(cfg.Source, absPath, opt)...)

	modulePath := hclutils.ResolveSource(cfg.Source, absPath)
	if modulePath == "" {
		if hclutils.IsRemoteSource(cfg.Source) {
			if len(results) > 0 {
				return filterErrors(results, opt), nil
			}
			return nil, fmt.Errorf("cannot resolve remote source %q — run 'terragrunt init' first to populate .terragrunt-cache/", cfg.Source)
		}
		return filterErrors(results, opt), nil
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
	tfVarKeys := hclutils.ParseTfVarKeys(cfg.TfVarFiles)

	errs := check(absPath, cfg.Inputs, variables, depOutputKeys, envVarKeys, cfg.IncludeInputKeys, tfVarKeys, cfg.EvalCtx)
	results = append(results, errs...)
	results = filterErrors(applyAllowList(results, opt), opt)
	results = applySeverity(results, opt)
	return results, nil
}

// StackFile validates a terragrunt.stack.hcl file by checking each unit.
func StackFile(path string, opts ...Options) ([]Error, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolving path %s: %w", path, err)
	}

	stack, err := hclutils.ParseStackFile(absPath)
	if err != nil {
		return nil, err
	}

	var opt Options
	if len(opts) > 0 {
		opt = opts[0]
	}

	envVarKeys := tfVarEnvKeys()

	var allErrors []Error
	for _, unit := range stack.Units {
		if unit.Source == "" {
			continue
		}

		refErrs := checkSourceRef(unit.Source, absPath, opt)
		for i := range refErrs {
			refErrs[i].Detail = fmt.Sprintf("[unit %q] %s", unit.Name, refErrs[i].Detail)
		}
		allErrors = append(allErrors, refErrs...)

		protoErrs := checkSourceProtocol(unit.Source, absPath, opt)
		for i := range protoErrs {
			protoErrs[i].Detail = fmt.Sprintf("[unit %q] %s", unit.Name, protoErrs[i].Detail)
		}
		allErrors = append(allErrors, protoErrs...)

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

		// Stack units don't use merge(dependency.outputs) and don't use includes
		unitErrors := check(absPath, unit.Values, variables, nil, envVarKeys, nil, nil, unit.EvalCtx)
		// Tag errors with unit name for clarity
		for i := range unitErrors {
			unitErrors[i].Detail = fmt.Sprintf("[unit %q] %s", unit.Name, unitErrors[i].Detail)
		}
		allErrors = append(allErrors, unitErrors...)
	}

	allErrors = filterErrors(applyAllowList(allErrors, opt), opt)
	allErrors = applySeverity(allErrors, opt)
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
			exempt[o.Name] = true
		}
	}

	return exempt
}

// TerraformDir validates all module blocks in .tf files within a directory.
func TerraformDir(dir string, opts ...Options) ([]Error, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolving path %s: %w", dir, err)
	}

	calls, err := hclutils.ParseModuleCalls(absDir)
	if err != nil {
		return nil, err
	}

	envVarKeys := tfVarEnvKeys()

	var opt Options
	if len(opts) > 0 {
		opt = opts[0]
	}

	var allErrors []Error
	for _, mc := range calls {
		protoErrs := checkSourceProtocol(mc.Source, absDir, opt)
		for i := range protoErrs {
			protoErrs[i].Detail = fmt.Sprintf("[module %q] %s", mc.Name, protoErrs[i].Detail)
		}
		allErrors = append(allErrors, protoErrs...)

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

		mcErrors := check(absDir, mc.Inputs, variables, nil, envVarKeys, nil, nil, mc.EvalCtx)
		for i := range mcErrors {
			mcErrors[i].Detail = fmt.Sprintf("[module %q] %s", mc.Name, mcErrors[i].Detail)
		}
		allErrors = append(allErrors, mcErrors...)
	}

	allErrors = filterErrors(applyAllowList(allErrors, opt), opt)
	allErrors = applySeverity(allErrors, opt)
	return allErrors, nil
}

// ModuleDir validates variable and output declarations in a Terraform module directory.
func ModuleDir(dir string, opts ...Options) ([]Error, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolving path %s: %w", dir, err)
	}

	var opt Options
	if len(opts) > 0 {
		opt = opts[0]
	}

	variables, err := tfmod.ParseVariables(absDir)
	if err != nil {
		return nil, err
	}

	outputs, err := tfmod.ParseOutputs(absDir)
	if err != nil {
		return nil, err
	}

	var errs []Error

	for _, v := range variables {
		if !v.HasDescription {
			errs = append(errs, Error{
				File:     absDir,
				Variable: v.Name,
				Kind:     MissingDescription,
				Detail:   fmt.Sprintf("variable %q has no description", v.Name),
			})
		}
		if !snakeCaseRe.MatchString(v.Name) {
			errs = append(errs, Error{
				File:     absDir,
				Variable: v.Name,
				Kind:     NonSnakeCase,
				Detail:   fmt.Sprintf("variable name %q is not snake_case", v.Name),
			})
		}
	}

	for _, o := range outputs {
		if !o.HasDescription {
			errs = append(errs, Error{
				File:     absDir,
				Variable: o.Name,
				Kind:     MissingDescription,
				Detail:   fmt.Sprintf("output %q has no description", o.Name),
			})
		}
		if !snakeCaseRe.MatchString(o.Name) {
			errs = append(errs, Error{
				File:     absDir,
				Variable: o.Name,
				Kind:     NonSnakeCase,
				Detail:   fmt.Sprintf("output name %q is not snake_case", o.Name),
			})
		}
	}

	// Rule: unused-variables — guard I/O behind rule check
	if opt.Config != nil && opt.Config.IsRuleEnabled("unused-variables", absDir) {
		refs, refErr := tfmod.CollectVarRefs(absDir)
		if refErr != nil {
			return nil, refErr
		}
		for _, v := range variables {
			if !refs[v.Name] {
				errs = append(errs, Error{
					File:     absDir,
					Variable: v.Name,
					Kind:     UnusedVariable,
					Detail:   fmt.Sprintf("variable %q is declared but never referenced", v.Name),
				})
			}
		}
	}

	// Rule: optional-without-default
	if opt.Config == nil || opt.Config.IsRuleEnabled("optional-without-default", absDir) {
		for _, v := range variables {
			if tfmod.HasOptionalWithoutDefault(v.Type) {
				errs = append(errs, Error{
					File:     absDir,
					Variable: v.Name,
					Kind:     OptionalWithoutDefault,
					Detail:   fmt.Sprintf("variable %q has optional() attribute without a default value", v.Name),
				})
			}
		}
	}

	// Rule: set-string-type
	if opt.Config != nil && opt.Config.IsRuleEnabled("set-string-type", absDir) {
		for _, v := range variables {
			if tfmod.HasSetString(v.Type) {
				errs = append(errs, Error{
					File:     absDir,
					Variable: v.Name,
					Kind:     SetStringType,
					Detail:   fmt.Sprintf("variable %q uses set(string) — consider list(string) instead", v.Name),
				})
			}
		}
	}

	// Rule: allowed-filenames — guard behind rule check
	if opt.Config != nil && opt.Config.IsRuleEnabled("allowed-filenames", absDir) {
		tfFiles, fileErr := tfmod.ListTFFiles(absDir)
		if fileErr != nil {
			return nil, fileErr
		}

		// Build allowed set from preset
		preset := getStringOption(opt, "allowed-filenames", "preset")
		allowed := make(map[string]bool)

		// Default preset files
		for _, f := range []string{"main.tf", "variables.tf", "outputs.tf", "versions.tf"} {
			allowed[f] = true
		}

		// Extended preset adds more
		if preset == "extended" {
			for _, f := range []string{"providers.tf", "locals.tf", "data.tf", "terraform.tf"} {
				allowed[f] = true
			}
		}

		// Additional user-specified files
		for _, f := range getListOption(opt, "allowed-filenames", "additional") {
			allowed[f] = true
		}

		// If has-versions-tf rule is also enabled, auto-include versions.tf
		if opt.Config.IsRuleEnabled("has-versions-tf", absDir) {
			allowed["versions.tf"] = true
		}

		for _, f := range tfFiles {
			if !allowed[f] {
				errs = append(errs, Error{
					File:     absDir,
					Variable: f,
					Kind:     DisallowedFilename,
					Detail:   fmt.Sprintf("file %q is not in the allowed filenames list", f),
				})
			}
		}
	}

	// Rule: has-versions-tf — guard behind rule check
	if opt.Config != nil && opt.Config.IsRuleEnabled("has-versions-tf", absDir) {
		vResult, vErr := tfmod.ParseVersionsTF(absDir)
		if vErr != nil {
			return nil, vErr
		}

		if !vResult.Exists {
			errs = append(errs, Error{
				File:   absDir,
				Kind:   MissingVersionsTF,
				Detail: "module directory is missing versions.tf",
			})
		} else {
			if !vResult.HasTerraformBlock {
				errs = append(errs, Error{
					File:   absDir,
					Kind:   MissingTerraformBlock,
					Detail: "versions.tf is missing a terraform block",
				})
			}

			seen := make(map[string]string) // provider name → first file
			for _, p := range vResult.Providers {
				if !p.HasSource {
					errs = append(errs, Error{
						File:     absDir,
						Variable: p.Name,
						Kind:     MissingProviderSource,
						Detail:   fmt.Sprintf("provider %q is missing source attribute", p.Name),
					})
				}
				if !p.HasVersion {
					errs = append(errs, Error{
						File:     absDir,
						Variable: p.Name,
						Kind:     MissingProviderVersion,
						Detail:   fmt.Sprintf("provider %q is missing version constraint", p.Name),
					})
				}
				if firstFile, exists := seen[p.Name]; exists {
					errs = append(errs, Error{
						File:     absDir,
						Variable: p.Name,
						Kind:     DuplicateProvider,
						Detail:   fmt.Sprintf("provider %q is declared in both %s and %s", p.Name, filepath.Base(firstFile), filepath.Base(p.File)),
					})
				} else {
					seen[p.Name] = p.File
				}
			}
		}

		// Rule: provider-constraint-style — check after existing provider validations
		style := getStringOption(opt, "provider-constraint-style", "style")
		depth := getStringOption(opt, "provider-constraint-style", "depth")
		if style != "" {
			for _, p := range vResult.Providers {
				if p.HasVersion && !checkConstraintStyle(p.Version, style, depth) {
					errs = append(errs, Error{
						File:     absDir,
						Variable: p.Name,
						Kind:     ProviderConstraintStyle,
						Detail:   fmt.Sprintf("provider %q version constraint %q does not match required style %q", p.Name, p.Version, describeStyle(style, depth)),
					})
				}
			}
		}
	}

	// Rule: empty-outputs-tf
	if opt.Config != nil && opt.Config.IsRuleEnabled("empty-outputs-tf", absDir) {
		tfFiles, fileErr := tfmod.ListTFFiles(absDir)
		if fileErr != nil {
			return nil, fileErr
		}

		// Check if outputs.tf exists but has no output blocks
		hasOutputsTF := false
		for _, f := range tfFiles {
			if f == "outputs.tf" {
				hasOutputsTF = true
				break
			}
		}

		if hasOutputsTF && len(outputs) == 0 {
			errs = append(errs, Error{
				File:   absDir,
				Kind:   EmptyOutputsTF,
				Detail: "outputs.tf exists but contains no output blocks",
			})
		}
	}

	// Rule: versions-tf-symlink
	if opt.Config != nil && opt.Config.IsRuleEnabled("versions-tf-symlink", absDir) {
		root := findModuleRoot(absDir)
		if absDir != root {
			// This is a submodule, check that versions.tf is a symlink
			versionsTFPath := filepath.Join(absDir, "versions.tf")
			fi, err := os.Lstat(versionsTFPath)
			if err != nil && !os.IsNotExist(err) {
				return nil, fmt.Errorf("versions-tf-symlink: lstat %s: %w", versionsTFPath, err)
			}

			if err == nil {
				// File exists, check if it's a symlink
				if fi.Mode()&os.ModeSymlink == 0 {
					errs = append(errs, Error{
						File:   absDir,
						Kind:   VersionsTFNotSymlink,
						Detail: "versions.tf is not a symlink (expected symlink to root module)",
					})
				} else {
					// It's a symlink, verify it resolves to the root's versions.tf
					resolved, err := filepath.EvalSymlinks(versionsTFPath)
					if err != nil {
						return nil, fmt.Errorf("versions-tf-symlink: eval symlinks %s: %w", versionsTFPath, err)
					}

					expectedTarget := filepath.Join(root, "versions.tf")
					if resolved != expectedTarget {
						errs = append(errs, Error{
							File:   absDir,
							Kind:   VersionsTFNotSymlink,
							Detail: fmt.Sprintf("versions.tf symlink resolves to %s, expected %s", resolved, expectedTarget),
						})
					}
				}
			}
		}
	}

	// Rule: missing-validation
	if opt.Config != nil && opt.Config.IsRuleEnabled("missing-validation", absDir) {
		excludes := getExcludePatterns(opt, "missing-validation")
		for _, v := range variables {
			if v.HasValidation {
				continue
			}

			// Check if variable name matches any exclude pattern
			excluded := false
			for _, p := range excludes {
				if matched, _ := filepath.Match(p, v.Name); matched {
					excluded = true
					break
				}
			}
			if excluded {
				continue
			}

			errs = append(errs, Error{
				File:     absDir,
				Variable: v.Name,
				Kind:     MissingValidation,
				Detail:   fmt.Sprintf("variable %q has no validation block", v.Name),
			})
		}
	}

	// Rule: sensitive-output
	if opt.Config != nil && opt.Config.IsRuleEnabled("sensitive-output", absDir) {
		sensitiveVars := make(map[string]bool)
		for _, v := range variables {
			if v.IsSensitive {
				sensitiveVars[v.Name] = true
			}
		}

		for _, o := range outputs {
			if o.IsSensitive {
				continue
			}

			for _, refName := range o.VarRefs {
				if sensitiveVars[refName] {
					errs = append(errs, Error{
						File:     absDir,
						Variable: o.Name,
						Kind:     SensitiveOutput,
						Detail:   fmt.Sprintf("output %q references sensitive variable %q but is not marked sensitive", o.Name, refName),
					})
					break
				}
			}
		}
	}

	errs = filterErrors(errs, opt)
	errs = applySeverity(errs, opt)
	return errs, nil
}

func check(file string, inputs map[string]hcl.Expression, variables []tfmod.Variable, depOutputKeys map[string]bool, envVarKeys map[string]bool, includeInputKeys map[string]bool, tfVarKeys map[string]bool, evalCtx *hcl.EvalContext) []Error {
	// Build lookup sets.
	// Dep output keys count as provided (they satisfy required variables AND
	// are exempt from extra-input errors — Terraform silently ignores them).
	// Env var keys count as provided (they satisfy required variables but are NOT
	// exempt from extra-input errors — they are not explicit inputs).
	// Include input keys count as provided (they satisfy required variables but are NOT
	// exempt from extra-input errors — they are merged from parent includes).
	// TfVar keys count as provided (they satisfy required variables but are NOT
	// exempt from extra-input errors — they are defined in tfvars files).
	inputKeys := make(map[string]bool, len(inputs)+len(depOutputKeys)+len(envVarKeys)+len(includeInputKeys)+len(tfVarKeys))
	for k := range inputs {
		inputKeys[k] = true
	}
	for k := range depOutputKeys {
		inputKeys[k] = true
	}
	for k := range envVarKeys {
		inputKeys[k] = true
	}
	for k := range includeInputKeys {
		inputKeys[k] = true
	}
	for k := range tfVarKeys {
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

	// Check for extra inputs from tfvars files
	extraTfVarKeys := make([]string, 0)
	for k := range tfVarKeys {
		if _, ok := varMap[k]; !ok {
			extraTfVarKeys = append(extraTfVarKeys, k)
		}
	}
	sort.Strings(extraTfVarKeys)

	for _, k := range extraTfVarKeys {
		errs = append(errs, Error{
			File:     file,
			Variable: k,
			Kind:     ExtraInput,
			Detail:   fmt.Sprintf("input %q has no matching variable in module (from tfvars file)", k),
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

// checkConstraintStyle validates that a version constraint matches the required style and optional depth.
func checkConstraintStyle(version, style, depth string) bool {
	trimmed := strings.TrimSpace(version)
	switch style {
	case "pessimistic":
		if !strings.HasPrefix(trimmed, "~>") {
			return false
		}
		if depth == "" {
			return true
		}
		num := strings.TrimSpace(strings.TrimPrefix(trimmed, "~>"))
		parts := strings.Split(num, ".")
		switch depth {
		case "major":
			return len(parts) == 2 && parts[1] == "0"
		case "minor":
			return len(parts) == 2 && parts[1] != "0"
		case "patch":
			return len(parts) == 3
		}
	case "exact":
		return strings.HasPrefix(trimmed, "=") && !strings.HasPrefix(trimmed, ">=")
	case "range":
		return strings.Contains(trimmed, ",")
	}
	return true
}

// describeStyle returns a human-readable description of the required style.
func describeStyle(style, depth string) string {
	if depth != "" {
		return style + "/" + depth
	}
	return style
}
