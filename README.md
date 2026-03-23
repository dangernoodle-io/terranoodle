# terra-tools

[![Go](https://img.shields.io/badge/Go-1.26.1-00ADD8?logo=go)](https://go.dev/)
[![Test](https://github.com/dangernoodle-io/terra-tools/actions/workflows/test.yml/badge.svg)](https://github.com/dangernoodle-io/terra-tools/actions/workflows/test.yml)
[![Release](https://github.com/dangernoodle-io/terra-tools/actions/workflows/release.yml/badge.svg)](https://github.com/dangernoodle-io/terra-tools/actions/workflows/release.yml)
[![Coverage Status](https://coveralls.io/repos/github/dangernoodle-io/terra-tools/badge.svg?branch=main)](https://coveralls.io/github/dangernoodle-io/terra-tools?branch=main)

A unified CLI for managing Terragrunt/Terraform infrastructure. terra-tools consolidates three essential operations into a single binary:

- **catalog generate** — generate [implicit terragrunt stacks](https://terragrunt.gruntwork.io/docs/features/stacks/#implicit-stacks) from a catalog-driven template system
- **state import** — generate and apply terraform import blocks from plan output
- **lint** — validate terragrunt configs against their referenced terraform modules

> **Note:** terra-tools is in pre-release. The CLI interface is subject to change.

## Commands

```
terra-tools
  catalog
    generate    generate terragrunt stack from catalog template
  lint          validate terragrunt configs against terraform modules
  state
    import      generate and apply import blocks from terraform plan
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

### state scaffold

Scaffold an import config YAML from an existing terraform plan.

| Flag | Short | Description |
|------|-------|-------------|
| `--dir` | | Working directory (default: cwd) |
| `--output` | `-o` | Write to file instead of stdout |
| `--fetch-registry` | | Fetch import ID formats from provider docs |

## Prerequisites

- **terraform >= 1.5** — required for native import block syntax
- **git** — required when using remote catalog sources (`git::https://...`)
- **Go 1.22+** — required when building from source

## Installation

### From GitHub Releases

Download the latest release from the [releases page](https://github.com/dangernoodle-io/terra-tools/releases).

Archives are provided for:
- `linux_amd64`, `linux_arm64`
- `darwin_amd64`, `darwin_arm64` (macOS)

### From Source

```bash
go install dangernoodle.io/terra-tools@latest
```

## License

See LICENSE file in repository.
