variable "project_id" {
  description = "GCP project ID"
  type        = string
}

resource "google_project" "project" {
  name       = "test"
  project_id = var.project_id
}
