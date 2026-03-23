terraform {
  source = "${get_terragrunt_dir()}/../module"
}

inputs = {
  project_id  = "prj-test-001"
  environment = "dev"
}
