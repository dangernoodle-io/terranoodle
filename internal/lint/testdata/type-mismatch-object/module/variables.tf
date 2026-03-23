variable "config" {
  type = object({
    name    = string
    enabled = bool
  })
}
