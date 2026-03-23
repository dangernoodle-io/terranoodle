module "vpc" {
  source = "../child-module"

  project_id  = "prj-test-001"
  environment = "dev"
}
