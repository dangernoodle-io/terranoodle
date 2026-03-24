terraform {
  source = "git::https://example.com/modules/vpc.git?ref=v1.0.0"
}

inputs = {
  name = "test"
}
