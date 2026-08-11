variable "project_name" {
  description = "Project name"
  type        = string
}

variable "environment" {
  description = "Environment name"
  type        = string
}

variable "vpc_id" {
  description = "VPC ID"
  type        = string
}

variable "private_subnet_ids" {
  description = "Private subnet IDs for ElastiCache"
  type        = list(string)
}

variable "redis_password" {
  description = "Redis auth password — comes from Secrets Manager"
  type        = string
  sensitive   = true
}

variable "eks_security_group" {
  description = "EKS node security group — only this can access Redis"
  type        = string
}