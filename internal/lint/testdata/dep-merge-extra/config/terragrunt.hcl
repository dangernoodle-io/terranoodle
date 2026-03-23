dependency "project" {
  config_path = "../project"
}

terraform {
  source = "../child-module"
}

inputs = merge(dependency.project.outputs, {
  name      = "test-service"
  bogus_key = "this-has-no-matching-variable"
})
