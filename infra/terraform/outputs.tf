# ── VPC Outputs ────────────────────────────────────────────────
output "vpc_id" {
  description = "VPC ID — used by all other modules"
  value       = module.vpc.vpc_id
}

output "private_subnet_ids" {
  description = "Private subnet IDs — EKS nodes run here"
  value       = module.vpc.private_subnet_ids
}

output "public_subnet_ids" {
  description = "Public subnet IDs — ALB runs here"
  value       = module.vpc.public_subnet_ids
}

# ── EKS Outputs ────────────────────────────────────────────────
output "eks_cluster_name" {
  description = "EKS cluster name — used in kubectl commands"
  value       = module.eks.cluster_name
}

output "eks_cluster_endpoint" {
  description = "EKS cluster API endpoint"
  value       = module.eks.cluster_endpoint
}

output "eks_cluster_arn" {
  description = "EKS cluster ARN"
  value       = module.eks.cluster_arn
}

output "node_security_group_id" {
  description = "Security group ID of EKS worker nodes"
  value       = module.eks.node_security_group_id
}

# ── RDS Outputs ────────────────────────────────────────────────
output "rds_endpoint" {
  description = "RDS PostgreSQL endpoint — used by Auth and Orders services"
  value       = module.rds.db_endpoint
  sensitive   = true
}

output "rds_port" {
  description = "RDS PostgreSQL port"
  value       = module.rds.db_port
}

# ── ElastiCache Outputs ────────────────────────────────────────
output "redis_endpoint" {
  description = "ElastiCache Redis endpoint — used by Auth and Orders services"
  value       = module.elasticache.redis_endpoint
  sensitive   = true
}

output "redis_port" {
  description = "ElastiCache Redis port"
  value       = module.elasticache.redis_port
}

# ── Amazon MQ Outputs ──────────────────────────────────────────
output "mq_endpoint" {
  description = "Amazon MQ RabbitMQ endpoint — used by Orders, Notifications, Worker"
  value       = module.mq.mq_endpoint
  sensitive   = true
}

# ── ECR Outputs ────────────────────────────────────────────────
output "ecr_repository_urls" {
  description = "ECR repository URLs for all 5 services"
  value = {
    auth          = "350480401763.dkr.ecr.ap-south-1.amazonaws.com/shopsre/auth"
    orders        = "350480401763.dkr.ecr.ap-south-1.amazonaws.com/shopsre/orders"
    notifications = "350480401763.dkr.ecr.ap-south-1.amazonaws.com/shopsre/notifications"
    worker        = "350480401763.dkr.ecr.ap-south-1.amazonaws.com/shopsre/worker"
    frontend      = "350480401763.dkr.ecr.ap-south-1.amazonaws.com/shopsre/frontend"
  }
}

# ── IAM Outputs ────────────────────────────────────────────────
output "service_role_arns" {
  description = "IAM role ARNs for all services — used in Helm charts"
  value = {
    auth          = module.iam.auth_service_role_arn
    orders        = module.iam.orders_service_role_arn
    notifications = module.iam.notifications_service_role_arn
    worker        = module.iam.worker_service_role_arn
  }
}

output "cloudwatch_log_groups" {
  description = "CloudWatch log group names"
  value       = module.iam.cloudwatch_log_groups
}

# ── Summary ────────────────────────────────────────────────────
# This prints a helpful summary after terraform apply
output "summary" {
  description = "Summary of all created resources"
  value = <<-EOT

    ============================================
    ShopSRE Platform Infrastructure Summary
    ============================================
    EKS Cluster:  ${module.eks.cluster_name}
    Region:       ap-south-1
    Environment:  ${var.environment}

    Next steps:
    1. Run: aws eks update-kubeconfig --name ${module.eks.cluster_name} --region ap-south-1
    2. Run: kubectl get nodes
    3. Deploy services via Helm
    ============================================
  EOT
}