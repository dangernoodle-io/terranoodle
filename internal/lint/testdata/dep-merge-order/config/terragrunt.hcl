dependency "project" {
  config_path = "../project"
}

dependency "network" {
  config_path = "../network"
}

terraform {
  source = "../module"
}

inputs = merge(dependency.network.outputs, dependency.project.outputs, {
  name = "test-service"
})
