include "root" {
  path   = "../root/terragrunt.hcl"
  expose = false
}

terraform {
  source = "../module"
}

inputs = {
  service_name = "api-gateway"
}
