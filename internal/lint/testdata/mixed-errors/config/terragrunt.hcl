terraform {
  source = "../module"
}

inputs = {
  project_id  = "prj-test-001"
  bogus_field = "should not be here"
}
