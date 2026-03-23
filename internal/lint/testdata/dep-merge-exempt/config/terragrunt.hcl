dependency "project" {
  config_path = "../project"
}

terraform {
  source = "../child-module"
}

inputs = merge(dependency.project.outputs, {
  name = "test-service"
})
