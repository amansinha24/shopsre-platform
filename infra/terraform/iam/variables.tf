variable "project_name" {
  description = "Project name"
  type        = string
}

variable "environment" {
  description = "Environment name"
  type        = string
}

variable "oidc_provider_arn" {
  description = "OIDC provider ARN from EKS module — needed for IRSA"
  type        = string
}

variable "oidc_provider_url" {
  description = "OIDC provider URL from EKS module — needed for IRSA"
  type        = string
}