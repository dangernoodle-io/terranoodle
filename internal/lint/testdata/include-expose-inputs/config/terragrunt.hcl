include "root" {
  path   = "../parent/terragrunt.hcl"
  expose = true
}

terraform {
  source = "../module"
}

inputs = {
  project_id  = include.root.inputs.project_id
  environment = include.root.inputs.environment
}
