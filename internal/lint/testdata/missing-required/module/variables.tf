variable "project_id" {
  description = "GCP project ID"
  type        = string
}

variable "environment" {
  description = "Project environment"
  type        = string
}

variable "region" {
  description = "GCP region"
  type        = string
}

variable "labels" {
  default     = {}
  description = "Labels to apply"
  type        = map(string)
}
