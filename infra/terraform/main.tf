# ── Terraform Configuration ────────────────────────────────────
# Defines which providers we need and where to store state
terraform {
  required_version = ">= 1.0"

  # Store state in S3 — shared, versioned, encrypted
  backend "s3" {
    bucket         = "shopsre-terraform-state-350480401763"
    key            = "shopsre/terraform.tfstate"
    region         = "ap-south-1"
    dynamodb_table = "shopsre-terraform-locks"
    encrypt        = true
  }

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

# ── AWS Provider ───────────────────────────────────────────────
# Tells Terraform which AWS region to use
# All resources created in ap-south-1 (Mumbai)
provider "aws" {
  region = var.aws_region

  # These tags are automatically applied to every resource
  # Makes it easy to find all ShopSRE resources in AWS console
  default_tags {
    tags = {
      Project     = "shopsre-platform"
      Environment = var.environment
      ManagedBy   = "terraform"
    }
  }
}

# ── Read Secrets from AWS Secrets Manager ──────────────────────
# Terraform reads these at apply time
# Passwords never stored in any file or Git
data "aws_secretsmanager_secret_version" "db_password" {
  secret_id = "shopsre/db-password"
}

data "aws_secretsmanager_secret_version" "mq_password" {
  secret_id = "shopsre/mq-password"
}

data "aws_secretsmanager_secret_version" "jwt_secret" {
  secret_id = "shopsre/jwt-secret"
}

data "aws_secretsmanager_secret_version" "redis_password" {
  secret_id = "shopsre/redis-password"
}

# ── Module 1: VPC ──────────────────────────────────────────────
# Creates the network — VPC, subnets, NAT gateway
# Must be created first — everything else depends on it
module "vpc" {
  source = "./vpc"

  project_name       = var.project_name
  environment        = var.environment
  vpc_cidr           = var.vpc_cidr
  availability_zones = var.availability_zones
  private_subnets    = var.private_subnets
  public_subnets     = var.public_subnets
}

# ── Module 2: EKS ──────────────────────────────────────────────
# Creates the Kubernetes cluster and worker nodes
# Depends on VPC — needs subnet IDs
module "eks" {
  source = "./eks"

  project_name       = var.project_name
  environment        = var.environment
  cluster_version    = var.cluster_version
  vpc_id             = module.vpc.vpc_id
  private_subnet_ids = module.vpc.private_subnet_ids
  public_subnet_ids  = module.vpc.public_subnet_ids
  node_instance_type = var.node_instance_type
  node_min_size      = var.node_min_size
  node_max_size      = var.node_max_size
  node_desired_size  = var.node_desired_size
}

# ── Module 3: RDS ──────────────────────────────────────────────
# Creates PostgreSQL database
# Password comes from Secrets Manager — not from any file
module "rds" {
  source = "./rds"

  project_name       = var.project_name
  environment        = var.environment
  vpc_id             = module.vpc.vpc_id
  private_subnet_ids = module.vpc.private_subnet_ids
  db_name            = var.db_name
  db_username        = var.db_username
  db_password        = data.aws_secretsmanager_secret_version.db_password.secret_string
  eks_security_group = module.eks.node_security_group_id
}

# ── Module 4: ElastiCache ──────────────────────────────────────
# Creates Redis cache
# Password comes from Secrets Manager
module "elasticache" {
  source = "./elasticache"

  project_name       = var.project_name
  environment        = var.environment
  vpc_id             = module.vpc.vpc_id
  private_subnet_ids = module.vpc.private_subnet_ids
  redis_password     = data.aws_secretsmanager_secret_version.redis_password.secret_string
  eks_security_group = module.eks.node_security_group_id
}

# ── Module 5: Amazon MQ ────────────────────────────────────────
# Creates RabbitMQ message broker
# Password comes from Secrets Manager
module "mq" {
  source = "./mq"

  project_name       = var.project_name
  environment        = var.environment
  vpc_id             = module.vpc.vpc_id
  private_subnet_ids = module.vpc.private_subnet_ids
  mq_username        = var.mq_username
  mq_password        = data.aws_secretsmanager_secret_version.mq_password.secret_string
  eks_security_group = module.eks.node_security_group_id
}

# ── Module 6: IAM ──────────────────────────────────────────────
# Creates IAM roles for each service (IRSA)
# Creates CloudWatch log groups
# Depends on EKS — needs OIDC provider ARN
module "iam" {
  source = "./iam"

  project_name      = var.project_name
  environment       = var.environment
  oidc_provider_arn = module.eks.oidc_provider_arn
  oidc_provider_url = module.eks.oidc_provider_url
}