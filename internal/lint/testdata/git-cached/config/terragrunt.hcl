terraform {
  source = "git::https://gitlab.com/example/modules/vpc.git?ref=1.0.0"
}

inputs = {
  project_id = "prj-test-001"
  network    = "vpc-dev"
}
