locals {
  name = "test"
}

unit "broken" {
  source = "./module"
  path   = "broken"
  values = {
    bogus = "extra"
  }
}
