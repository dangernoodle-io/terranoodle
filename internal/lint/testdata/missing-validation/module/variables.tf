variable "project_id" {
  type        = string
  description = "The project identifier"
}

variable "environment" {
  type        = string
  description = "The deployment environment"

  validation {
    condition     = contains(["dev", "staging", "prod"], var.environment)
    error_message = "Environment must be dev, staging, or prod."
  }
}
