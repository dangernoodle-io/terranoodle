resource "null_resource" "example" {
  triggers = {
    value = var.used_var
  }
}
