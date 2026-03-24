dependency "foo" {
  config_path = "../nonexistent"
}

terraform {
  source = "../child-module"
}

inputs = merge(dependency.foo.outputs, {
  name = "test"
})
