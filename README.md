# terranoodle

[![Go](https://img.shields.io/badge/Go-1.26.1-00ADD8?logo=go)](https://go.dev/)
[![Build](https://github.com/dangernoodle-io/terranoodle/actions/workflows/build.yml/badge.svg)](https://github.com/dangernoodle-io/terranoodle/actions/workflows/build.yml)
[![Release](https://github.com/dangernoodle-io/terranoodle/actions/workflows/release.yml/badge.svg)](https://github.com/dangernoodle-io/terranoodle/actions/workflows/release.yml)
[![Coverage Status](https://coveralls.io/repos/github/dangernoodle-io/terranoodle/badge.svg?branch=main)](https://coveralls.io/github/dangernoodle-io/terranoodle?branch=main)

A unified CLI for managing Terragrunt/Terraform infrastructure.

> **Maintained by AI** — This project is developed and maintained by Claude (via [@dangernoodle-io](https://github.com/dangernoodle-io)).
> If you find a bug or have a feature request, please [open an issue](https://github.com/dangernoodle-io/terranoodle/issues) with examples so it can be addressed.

## Commands

| Command | Description | Docs |
|---------|-------------|------|
| `catalog` | Generate terragrunt stacks from catalog templates | [Wiki](../../wiki/Catalog) |
| `config` | Manage project and global configuration | [Wiki](../../wiki/Config) |
| `lint` | Validate terragrunt configs against terraform modules | [Wiki](../../wiki/Lint) |
| `state` | Import, remove, rename, and scaffold terraform state | [Wiki](../../wiki/State) |

## Install

### Homebrew

```bash
brew install dangernoodle-io/tap/terranoodle
```

### From Source

```bash
go install dangernoodle.io/terranoodle@latest
```

### From GitHub Releases

Download the latest release from the [releases page](https://github.com/dangernoodle-io/terranoodle/releases).

Archives are provided for:
- `linux_amd64`, `linux_arm64`
- `darwin_amd64`, `darwin_arm64` (macOS)

## Prerequisites

- **terraform >= 1.5** — required for native import block syntax
- **terragrunt >= 0.90** — required when managing terragrunt-based infrastructure
- **git** — required when using remote catalog sources (`git::https://...`)

## License

See LICENSE file in repository.
