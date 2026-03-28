# terranoodle

Unified Terragrunt/Terraform toolchain.

## Module

`dangernoodle.io/terranoodle` — Go 1.26.1

## CLI

```
terranoodle
  catalog
    generate [flags]   # generate terragrunt stack from catalog template
    scaffold [flags]   # generate catalog from existing terragrunt dir (not yet implemented)
  config
    get <key>          # get effective config value
    set <key> <value>  # set config value (--global for global config)
    list               # show effective merged config
    init               # scaffold .terranoodle.yml with defaults
  lint [flags]         # lint terragrunt stack configs (--config for explicit config path)
  state
    import [flags]     # generate import blocks from terraform plan (--dry-run for preview)
    remove [flags]     # remove destroyed resources from state without destroying infrastructure
    rename [flags]     # detect renames and generate moved blocks or execute state mv
    scaffold [flags]   # scaffold import config from plan (-o file, --save to central state)
```

## Install

### Homebrew

```bash
brew install dangernoodle-io/tap/terranoodle
```

### From Source

```bash
go install dangernoodle.io/terranoodle@latest
```

## Build

```
go build -o terranoodle ./
```

To embed a version:
```
go build -ldflags "-X dangernoodle.io/terranoodle/internal/cli.Version=v0.1.0-alpha.1" -o terranoodle ./
```

## Wiki

Commit wiki changes locally on `master`. The `/dn-go-release` skill pushes wiki commits to origin during releases.

## Packages

| Package | Purpose |
|---------|---------|
| `internal/cli/` | Cobra root + subcommand wiring |
| `internal/config/` | Project/global config (`.terranoodle.yml`) |
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
| `internal/state/remove/` | State removal operations |
| `internal/state/rename/` | Rename detection, moved block generation, state mv |
| `internal/state/resolver/` | ID resolution engine |
| `internal/state/scaffold/` | Scaffold YAML generation |
| `internal/state/scaffold/store/` | Central scaffold state persistence |
| `internal/lint/validate/` | Lint validation rules and walker |
| `internal/lint/report/` | Lint reporting |
