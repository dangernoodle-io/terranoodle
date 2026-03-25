dependency "project" {
  config_path = "../project"
}

dependency "network" {
  config_path = "../network"
}

terraform {
  source = "../module"
}

inputs = merge(dependency.project.outputs, dependency.network.outputs, {
  name = "test-service"
})
