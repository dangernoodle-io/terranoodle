variable "with_default" {
  description = "Has optional with default"
  type = object({
    name = optional(string, "default")
  })
}

variable "without_default" {
  description = "Has optional without default"
  type = object({
    name = optional(string)
  })
}

variable "nested_without_default" {
  description = "Nested optional without default"
  type = object({
    inner = object({
      name = optional(string)
    })
  })
}

variable "simple_type" {
  description = "No optional at all"
  type        = string
}
