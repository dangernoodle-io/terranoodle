locals {
  project_id = "prj-test-001"
  region     = "us-east1"
}

unit "main" {
  source = "./module"
  path   = "main"
  values = {
    project_id = local.project_id
    region     = local.region
  }
}
