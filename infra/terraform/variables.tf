variable "project_name" {
  description = "Short project identifier used in resource names."
  type        = string
}

variable "environment" {
  description = "Deployment environment, e.g. dev/stage/prod."
  type        = string
}

variable "location" {
  description = "Azure region for resources."
  type        = string
  default     = "westeurope"
}

variable "tags" {
  description = "Common tags applied to resources."
  type        = map(string)
  default     = {}
}
