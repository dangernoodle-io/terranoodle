terraform {
  source = "git::https://gitlab.com/example/modules/cloud-run.git//bootstrap?ref=0.1.1"
}

inputs = {
  project_id = "prj-test-001"
  network    = "vpc-dev"
}
