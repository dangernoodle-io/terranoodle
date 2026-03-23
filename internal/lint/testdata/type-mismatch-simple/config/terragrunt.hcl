terraform {
  source = "../module"
}

inputs = {
  project_id = 42
  enabled    = "not-a-bool"
}
