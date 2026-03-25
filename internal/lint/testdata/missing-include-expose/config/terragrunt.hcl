include "root" {
  path = "../parent/terragrunt.hcl"
}

terraform {
  source = "../module"
}

inputs = {
  environment = "test"
}
