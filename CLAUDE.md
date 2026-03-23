# terra-tools

Unified Terragrunt/Terraform toolchain. Single binary combining terra-generate, terra-import, and terra-lint.

## Module

`dangernoodle.io/terra-tools` — Go 1.26.1

## CLI

```
terra-tools
  catalog
    generate [flags]   # generate terragrunt stack from catalog template
    scaffold [flags]   # generate catalog from existing terragrunt dir (not yet implemented)
  lint [flags]         # lint terragrunt stack configs
  state
    import [flags]     # generate import blocks from terraform plan (--dry-run for preview)
    scaffold [flags]   # scaffold import config from existing state
  version              # print version
```

## Build

```
go build -o terra-tools ./
```

To embed a version:
```
go build -ldflags "-X dangernoodle.io/terra-tools/internal/cli.Version=v0.1.0-alpha.1" -o terra-tools ./
```

## Packages

| Package | Purpose |
|---------|---------|
| `internal/cli/` | Cobra root + subcommand wiring |
| `internal/output/` | Colored terminal output |
| `internal/ui/` | Terminal UI components (spinner) |
| `internal/hclutils/` | Shared HCL parsing utilities |
| `internal/catalog/catalog/` | Catalog fetch and walk |
| `internal/catalog/generator/` | Generation engine |
| `internal/catalog/hclparse/` | Template file HCL parsing |
| `internal/state/config/` | Import mapping config |
| `internal/state/importer/` | Terraform apply/state operations |
| `internal/state/plan/` | Plan JSON parsing |
| `internal/state/prompt/` | Interactive prompts |
| `internal/state/resolver/` | ID resolution engine |
| `internal/state/scaffold/` | Scaffold YAML generation |
| `internal/lint/validate/` | Lint validation rules and walker |
| `internal/lint/report/` | Lint reporting |
