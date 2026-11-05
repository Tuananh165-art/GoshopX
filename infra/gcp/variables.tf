variable "project_id" {
  type    = string
  default = "generated-mote-467410-b0"
}

variable "billing_account" {
  type      = string
  sensitive = true
}

variable "region" {
  type    = string
  default = "asia-southeast1"
}

variable "zone" {
  type    = string
  default = "asia-southeast1-a"
}

variable "domain" {
  type    = string
  default = "goshopx.click"
}

variable "machine_type" {
  type    = string
  default = "e2-standard-4"
}

variable "node_count" {
  type    = number
  default = 2
  validation {
    condition     = var.node_count >= 1 && var.node_count <= 2
    error_message = "The lab node pool is limited to one or two nodes."
  }
}

variable "github_repository" {
  description = "GitHub owner/repository allowed to publish GoshopX images"
  type        = string
}
