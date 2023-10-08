variable "display_name" {
  type = string
}

variable "kv_name" {
  type = string
}

variable "app_roles" {
  type    = list(string)
  default = []
}

variable "identifier_uri" {
  type = string
}

variable "resource_group_name" {
  type = string
}