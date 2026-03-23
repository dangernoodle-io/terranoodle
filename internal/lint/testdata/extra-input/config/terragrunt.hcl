terraform {
  source = "../module"
}

inputs = {
  project_id  = "prj-test-001"
  environment = "dev"
  bogus_field = "should not be here"
}
