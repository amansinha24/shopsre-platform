# ── General ────────────────────────────────────────────────────
variable "project_name" {
  description = "Name of the project — used in all resource names"
  type        = string
  default     = "shopsre"
}

variable "environment" {
  description = "Environment name — production, staging, dev"
  type        = string
  default     = "production"
}

variable "aws_region" {
  description = "AWS region where all resources will be created"
  type        = string
  default     = "ap-south-1"
}

# ── VPC ────────────────────────────────────────────────────────
variable "vpc_cidr" {
  description = "CIDR block for the VPC — defines IP range"
  type        = string
  default     = "10.0.0.0/16"
}

variable "availability_zones" {
  description = "List of availability zones to use"
  type        = list(string)
  default     = ["ap-south-1a", "ap-south-1b", "ap-south-1c"]
}

variable "private_subnets" {
  description = "CIDR blocks for private subnets — EKS nodes live here"
  type        = list(string)
  default     = ["10.0.1.0/24", "10.0.2.0/24", "10.0.3.0/24"]
}

variable "public_subnets" {
  description = "CIDR blocks for public subnets — ALB lives here"
  type        = list(string)
  default     = ["10.0.101.0/24", "10.0.102.0/24", "10.0.103.0/24"]
}

# ── EKS ────────────────────────────────────────────────────────
variable "cluster_version" {
  description = "Kubernetes version for EKS cluster"
  type        = string
  default     = "1.35"
}

variable "node_instance_type" {
  description = "EC2 instance type for EKS worker nodes"
  type        = string
  default     = "t3.medium"
}

variable "node_min_size" {
  description = "Minimum number of worker nodes — never go below this"
  type        = number
  default     = 2
}

variable "node_max_size" {
  description = "Maximum number of worker nodes — never go above this"
  type        = number
  default     = 5
}

variable "node_desired_size" {
  description = "Desired number of worker nodes at startup"
  type        = number
  default     = 2
}

# ── RDS ────────────────────────────────────────────────────────
variable "db_name" {
  description = "PostgreSQL database name"
  type        = string
  default     = "shopsre"
}

variable "db_username" {
  description = "PostgreSQL master username"
  type        = string
  default     = "shopsreAdmin"
}

# ── Amazon MQ ──────────────────────────────────────────────────
variable "mq_username" {
  description = "RabbitMQ admin username"
  type        = string
  default     = "shopsreAdmin"
}