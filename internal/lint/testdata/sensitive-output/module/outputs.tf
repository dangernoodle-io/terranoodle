output "api_key_value" {
  description = "The API key output"
  value       = var.api_key
}

output "project_id_value" {
  description = "The project ID output"
  value       = var.project_id
  sensitive   = true
}
