module "good" {
  source = "../child-a"

  project_id  = "prj-test-001"
  environment = "dev"
}

module "bad" {
  source = "../child-b"

  name = "my-resource"
  # missing: region
  bogus = "extra"
}
