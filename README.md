# terranoodle

[![Go](https://img.shields.io/badge/Go-1.26.1-00ADD8?logo=go)](https://go.dev/)
[![Test](https://github.com/dangernoodle-io/terranoodle/actions/workflows/test.yml/badge.svg)](https://github.com/dangernoodle-io/terranoodle/actions/workflows/test.yml)
[![Release](https://github.com/dangernoodle-io/terranoodle/actions/workflows/release.yml/badge.svg)](https://github.com/dangernoodle-io/terranoodle/actions/workflows/release.yml)
[![Coverage Status](https://coveralls.io/repos/github/dangernoodle-io/terranoodle/badge.svg?branch=main)](https://coveralls.io/github/dangernoodle-io/terranoodle?branch=main)

A unified CLI for managing Terragrunt/Terraform infrastructure. terranoodle consolidates three essential operations into a single binary:

- **catalog generate** — generate [implicit terragrunt stacks](https://terragrunt.gruntwork.io/docs/features/stacks/#implicit-stacks) from a catalog-driven template system
- **state import** — generate and apply terraform import blocks from plan output
- **lint** — validate terragrunt configs against their referenced terraform modules

> **Note:** terranoodle is in pre-release. The CLI interface is subject to change.

## Commands

```
terranoodle
  catalog
    generate    generate terragrunt stack from catalog template
  lint          validate terragrunt configs against terraform modules
  state
    import      generate and apply import blocks from terraform plan
    rename      detect renames and generate moved blocks or execute state mv
    scaffold    scaffold import config YAML from existing plan
```

### catalog generate

Generate a terragrunt stack from a catalog template definition.

| Flag | Short | Description |
|------|-------|-------------|
| `--template` | `-t` | Template definition HCL file (required) |
| `--catalog` | `-c` | Catalog source: local path or git URL (required) |
| `--output` | `-o` | Output directory (required) |
| `--dry-run` | | Preview generation without writing files |
| `--scaffold` | | Write stubs for unconfigured services |

### lint

Validate terragrunt configs against terraform modules. Checks for missing required inputs, extra inputs, and type mismatches.

| Flag | Short | Description |
|------|-------|-------------|
| `--dir` | `-d` | Directory to lint (default: cwd) |
| `--recursive` | `-r` | Walk subdirectories |

### state import

Generate import blocks from a terraform plan and apply them.

| Flag | Short | Description |
|------|-------|-------------|
| `--config` | `-c` | YAML import config file (required) |
| `--dir` | | Working directory (default: cwd) |
| `--var` | | Override config vars, repeatable (`key=value`) |
| `--dry-run` | | Preview import blocks without applying |
| `--force` | | Overwrite existing `imports.tf` |

### state rename

Detects resource renames from a Terraform plan and generates `moved {}` blocks or executes `terraform state mv` commands. Preview is the default — use `--apply` to write files or mutate state.

| Flag | Description |
|------|-------------|
| `--moved` | Generate `moved {}` blocks (required, or use `--mv`) |
| `--mv` | Execute `terraform state mv` commands (required, or use `--moved`) |
| `--apply` | Execute the operation (default: preview to stdout) |
| `--dir` | Working directory (default: current directory) |
| `--plan` | Path to existing plan JSON (optional, auto-generates if omitted) |
| `--output`, `-o` | Output file path (default: `moved.tf`, only with `--moved`) |
| `--force` | Overwrite existing output file |

### state scaffold

Scaffold an import config YAML from an existing terraform plan.

| Flag | Short | Description |
|------|-------|-------------|
| `--dir` | | Working directory (default: cwd) |
| `--output` | `-o` | Write to file instead of stdout |
| `--fetch-registry` | | Fetch import ID formats from provider docs |

## Prerequisites

- **terraform >= 1.5** — required for native import block syntax
- **terragrunt >= 0.90** — required when managing terragrunt-based infrastructure
- **git** — required when using remote catalog sources (`git::https://...`)
- **Go 1.22+** — required when building from source

## Installation

### From GitHub Releases

Download the latest release from the [releases page](https://github.com/dangernoodle-io/terranoodle/releases).

Archives are provided for:
- `linux_amd64`, `linux_arm64`
- `darwin_amd64`, `darwin_arm64` (macOS)

### From Source

```bash
go install dangernoodle.io/terranoodle@latest
```

## License

See LICENSE file in repository.
