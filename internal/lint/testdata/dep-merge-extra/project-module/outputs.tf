output "project_id" {
  value = "prj-test"
}

output "environment" {
  value = "dev"
}

# region is output by project but NOT a variable in child-module.
# It must be exempt from the extra-input check.
output "region" {
  value = "us-east1"
}
