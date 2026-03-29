package config

type OptionMeta struct {
	Key     string
	Example string
	Desc    string
}

type RuleMeta struct {
	Name        string
	Default     bool
	Description string
	Options     []OptionMeta
	Autofix     bool
}

var Rules = []RuleMeta{
	{
		Name:        "missing-required",
		Default:     true,
		Description: "flag variables that are required but not provided in inputs",
	},
	{
		Name:        "extra-inputs",
		Default:     true,
		Description: "flag input keys with no matching variable in the module",
		Options: []OptionMeta{
			{
				Key:     "allow",
				Example: `["pattern*"]`,
				Desc:    "glob patterns to downgrade to warning",
			},
		},
	},
	{
		Name:        "type-mismatch",
		Default:     true,
		Description: "flag input values that don't match the variable type constraint",
	},
	{
		Name:        "source-ref-semver",
		Default:     true,
		Description: "flag remote source ref= that is not a semver version",
		Options: []OptionMeta{
			{
				Key:     "sha",
				Example: "warn",
				Desc:    "treat commit SHAs as warn or error",
			},
			{
				Key:     "allow",
				Example: `["pattern*"]`,
				Desc:    "glob patterns to downgrade to warning",
			},
		},
	},
	{
		Name:        "source-protocol",
		Default:     false,
		Description: "flag remote sources using the wrong transport protocol",
		Options: []OptionMeta{
			{
				Key:     "enforce",
				Example: "https",
				Desc:    "require https or ssh",
			},
		},
	},
	{
		Name:        "missing-description",
		Default:     false,
		Description: "flag variables and outputs missing a description attribute",
		Autofix:     true,
	},
	{
		Name:        "non-snake-case",
		Default:     false,
		Description: "flag variable and output names that are not snake_case",
	},
	{
		Name:        "unused-variables",
		Default:     false,
		Description: "flag variables declared but never referenced",
	},
	{
		Name:        "optional-without-default",
		Default:     false,
		Description: "flag variables using optional() without a default value",
	},
	{
		Name:        "missing-include-expose",
		Default:     false,
		Description: "flag include blocks missing expose = true",
		Options: []OptionMeta{
			{
				Key:     "exclude",
				Example: `["pattern*"]`,
				Desc:    "skip include names matching patterns",
			},
		},
	},
	{
		Name:        "allowed-filenames",
		Default:     false,
		Description: "flag .tf files not in the permitted filename set",
		Options: []OptionMeta{
			{
				Key:     "preset",
				Example: "extended",
				Desc:    "use extended preset (adds providers.tf, locals.tf, data.tf, terraform.tf)",
			},
			{
				Key:     "additional",
				Example: `["custom.tf"]`,
				Desc:    "extra allowed filenames",
			},
		},
	},
	{
		Name:        "has-versions-tf",
		Default:     false,
		Description: "flag modules missing versions.tf or required provider attributes",
	},
	{
		Name:        "no-tg-provider-blocks",
		Default:     false,
		Description: "flag terragrunt configs containing provider blocks",
	},
	{
		Name:        "set-string-type",
		Default:     false,
		Description: "flag variables using set(string) instead of list(string)",
	},
	{
		Name:        "provider-constraint-style",
		Default:     false,
		Description: "flag provider version constraints not matching required style",
		Options: []OptionMeta{
			{
				Key:     "style",
				Example: "pessimistic",
				Desc:    "require pessimistic, exact, or range style",
			},
			{
				Key:     "depth",
				Example: "minor",
				Desc:    "constraint depth for pessimistic style (major, minor, patch)",
			},
		},
	},
	{
		Name:        "empty-outputs-tf",
		Default:     false,
		Description: "flag outputs.tf files with no output blocks",
	},
	{
		Name:        "versions-tf-symlink",
		Default:     false,
		Description: "flag submodule versions.tf not symlinked to root",
	},
	{
		Name:        "missing-validation",
		Default:     false,
		Description: "flag variables missing validation blocks",
		Options: []OptionMeta{
			{
				Key:     "exclude",
				Example: `["pattern*"]`,
				Desc:    "skip variable names matching patterns",
			},
		},
		Autofix: true,
	},
	{
		Name:        "sensitive-output",
		Default:     false,
		Description: "flag outputs referencing sensitive variables without sensitive = true",
		Autofix:     true,
	},
	{
		Name:        "dependency-merge-order",
		Default:     false,
		Description: "flag merge() args not in configured priority order",
		Options: []OptionMeta{
			{
				Key:     "first",
				Example: `["dep-name"]`,
				Desc:    "ordered dependency names that must appear first",
			},
		},
	},
}
