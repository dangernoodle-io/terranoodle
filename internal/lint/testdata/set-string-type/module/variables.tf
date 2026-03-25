variable "good_var" {
  description = "Uses list(string)"
  type        = list(string)
}

variable "bad_var" {
  description = "Uses set(string)"
  type        = set(string)
}

variable "nested_bad" {
  description = "Nested set(string)"
  type = object({
    tags = set(string)
  })
}
