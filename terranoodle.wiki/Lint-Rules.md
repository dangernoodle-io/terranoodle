# Lint Rules

terranoodle includes a configurable lint engine that validates Terragrunt configurations against their referenced Terraform modules. This document describes all available lint rules.

## source-protocol

**Default:** disabled (`enabled: false`)

Enforces the transport protocol used in remote module source URLs.

### Options

| Option | Values | Default | Description |
|--------|--------|---------|-------------|
| `enforce` | `https`, `ssh`, `any` | `any` | Protocol to enforce |

### Behavior

| Source type | Example | `enforce: https` | `enforce: ssh` | `enforce: any` |
|-------------|---------|-------------------|----------------|----------------|
| Git SSH | `git::git@github.com:org/repo.git` | Error | Pass | Pass |
| Git HTTPS | `git::https://github.com/org/repo.git` | Pass | Error | Pass |
| GitHub shorthand | `github.com/org/modules//vpc` | Pass | Error | Pass |
| GitLab shorthand | `gitlab.com/org/modules//vpc` | Pass | Error | Pass |
| Registry (`tfr://`) | — | Skip | Skip | Skip |
| S3 (`s3://`) | — | Skip | Skip | Skip |
| Local | `../modules/vpc` | Skip | Skip | Skip |

### Configuration

Short form (disabled):
```yaml
lint:
  rules:
    source-protocol: false
```

Expanded form with enforcement:
```yaml
lint:
  rules:
    source-protocol:
      enabled: true
      enforce: https
```

### Notes

- GitHub and GitLab shorthand sources resolve to HTTPS.
- SSH is detected by the presence of `git@` in the source URL.
- Applies to `terragrunt.hcl`, `terragrunt.stack.hcl`, and Terraform module blocks.

## missing-description

**Default:** disabled (`enabled: false`)

Flags variables and outputs that lack a `description` attribute.

### Configuration

Short form:
```yaml
lint:
  rules:
    missing-description: true
```

Expanded form:
```yaml
lint:
  rules:
    missing-description:
      enabled: true
```

### Notes

- Applies to `variable` and `output` blocks in `.tf` files
- Only runs on pure Terraform module directories (not directories with `terragrunt.hcl`)
- Descriptions improve module documentation and are shown in `terraform-docs` output

## non-snake-case

**Default:** disabled (`enabled: false`)

Flags variable and output names that don't match the snake_case convention (`^[a-z][a-z0-9]*(_[a-z0-9]+)*$`).

### Examples

| Name | Valid |
|------|-------|
| `instance_type` | yes |
| `vpc_id` | yes |
| `instanceCount` | no (camelCase) |
| `BadOutput` | no (PascalCase) |
| `_leading` | no (leading underscore) |
| `trailing_` | no (trailing underscore) |

### Configuration

Short form:
```yaml
lint:
  rules:
    non-snake-case: true
```

Expanded form:
```yaml
lint:
  rules:
    non-snake-case:
      enabled: true
```

### Notes

- Applies to `variable` and `output` blocks in `.tf` files
- Only runs on pure Terraform module directories (not directories with `terragrunt.hcl`)
- snake_case is the Terraform naming convention per HashiCorp style guide

## unused-variable

**Default:** disabled

Flags variables declared in `.tf` files that are never referenced as `var.<name>` in any `.tf` file in the same directory.

### Configuration

Short form:
```yaml
lint:
  rules:
    unused-variable: true
```

Expanded form:
```yaml
lint:
  rules:
    unused-variable:
      enabled: true
```

### Notes

- Applies to module directories (non-terragrunt `.tf` directories only)
- Detects unused variables to help reduce module surface area and improve maintainability

## optional-without-default

**Default:** disabled

Flags variables whose type expression contains `optional()` calls without a default value argument. `optional(string)` is flagged; `optional(string, "default_value")` is not. Handles nested `object()` types recursively.

### Examples

Flagged code:
```hcl
variable "config" {
  type = object({
    name = optional(string)        # flagged — no default
    port = optional(number, 8080)  # ok — has default
  })
}
```

### Configuration

Short form:
```yaml
lint:
  rules:
    optional-without-default: true
```

Expanded form:
```yaml
lint:
  rules:
    optional-without-default:
      enabled: true
```

### Notes

- Applies to module directories (non-terragrunt `.tf` directories only)
- Enforces explicit defaults for optional type fields, reducing ambiguity and improving API clarity

## missing-include-expose

**Default:** disabled (`enabled: false`)

Flags `include` blocks in Terragrunt configurations that don't have `expose = true`. Without `expose`, the parent config's values are merged into the current configuration but are not individually addressable via the `include.<label>.<attr>` syntax.

### Scope

Applies to `terragrunt.hcl` and related Terragrunt config files containing `include` blocks.

### Options

| Option | Type | Description |
|--------|------|-------------|
| `exclude` | list of strings | Glob patterns matched against include block labels. Matching includes are not flagged. |

### Configuration

Short form (enabled):
```yaml
lint:
  rules:
    missing-include-expose: true
```

Expanded form with exclusions:
```yaml
lint:
  rules:
    missing-include-expose:
      enabled: true
      exclude:
        - "shared_*"
        - "common"
```

### Notes

- The `expose` attribute makes included configuration values accessible as a structured object, improving code clarity and reducing naming conflicts
- Useful for enforcing intentional parent-child configuration relationships in modular Terragrunt stacks
- Excluded labels use glob pattern matching (e.g., `shared_*` matches `shared_base`, `shared_vpc`, etc.)

## allowed-filenames

**Default:** disabled (`enabled: false`)

Flags `.tf` files not in the allowed set. Uses a preset system with optional additions. Applies only to pure Terraform module directories (non-terragrunt `.tf` directories).

### Scope

Module directories (non-terragrunt `.tf` directories only)

### Options

| Option | Values | Default | Description |
|--------|--------|---------|-------------|
| `preset` | `default`, `extended` | `default` | Filename allowlist preset |
| `additional` | list of strings | — | Extra filenames to allow beyond the preset |

### Presets

**default** includes:
- `main.tf`
- `variables.tf`
- `outputs.tf`
- `versions.tf`

**extended** includes all default files plus:
- `providers.tf`
- `locals.tf`
- `data.tf`
- `terraform.tf`

### Auto-integration

If the `versions-tf` rule is also enabled, `versions.tf` is automatically included in the allowed set.

### Configuration

Short form (uses default preset):
```yaml
lint:
  rules:
    allowed-filenames: true
```

Extended preset with additional files:
```yaml
lint:
  rules:
    allowed-filenames:
      enabled: true
      preset: extended
      additional:
        - backend.tf
        - override.tf
```

### Notes

- Helps enforce consistent module organization and structure
- The default preset aligns with Terraform module best practices
- Use `additional` to permit project-specific conventions without switching to the extended preset

## versions-tf

**Default:** disabled (`enabled: false`)

Validates module structure around `versions.tf`. Ensures the file exists, contains required blocks, and provider declarations are properly structured without duplicates.

### Scope

Module directories (non-terragrunt `.tf` directories only)

### Validation Rules

1. **File existence**: `versions.tf` must exist in the module directory
2. **Terraform block**: Must contain a `terraform {}` block
3. **Provider source**: Each provider in `required_providers` must declare a `source` attribute
4. **Provider version**: Each provider in `required_providers` must declare a `version` attribute
5. **No duplicates**: Provider declarations must not appear in multiple `.tf` files within the same module directory

### Error Types

| Error | Description |
|-------|-------------|
| `MissingVersionsTF` | `versions.tf` file not found in module directory |
| `MissingTerraformBlock` | No `terraform {}` block in `versions.tf` |
| `MissingProviderSource` | Provider in `required_providers` missing `source` attribute |
| `MissingProviderVersion` | Provider in `required_providers` missing `version` attribute |
| `DuplicateProvider` | Provider declared in multiple `.tf` files within the module |

### Configuration

Simple form:
```yaml
lint:
  rules:
    versions-tf: true
```

Expanded form:
```yaml
lint:
  rules:
    versions-tf:
      enabled: true
```

### Example

Valid `versions.tf`:
```hcl
terraform {
  required_version = ">= 1.5"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.0"
    }
  }
}
```

### Notes

- Centralizing provider configuration in `versions.tf` improves module maintainability and clarity
- Prevents accidental duplicate provider declarations when multiple `.tf` files exist
- Works in conjunction with `allowed-filenames` rule to enforce consistent module structure

## no-provider-block

**Default:** disabled (`enabled: false`)

Flags `provider` blocks in Terragrunt configurations. Provider configuration should be defined in the referenced Terraform module or generated via Terragrunt's `generate` blocks — not declared directly in `terragrunt.hcl`. Direct provider blocks in terragrunt configs can cause conflicts with module-level provider requirements and make the dependency graph harder to reason about.

### Scope

Applies to `terragrunt.hcl` and related Terragrunt config files.

### Configuration

Short form:
```yaml
lint:
  rules:
    no-provider-block: true
```

Expanded form:
```yaml
lint:
  rules:
    no-provider-block:
      enabled: true
```

### Notes

- Provider configuration belongs in Terraform modules or generated via `generate` blocks
- Direct `provider` blocks in `terragrunt.hcl` can lead to version conflicts and obscure module dependencies
- Promotes cleaner Terragrunt configurations and better separation of concerns
