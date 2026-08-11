output "cluster_name" {
  description = "EKS cluster name"
  value       = aws_eks_cluster.main.name
}

output "cluster_endpoint" {
  description = "EKS cluster API endpoint"
  value       = aws_eks_cluster.main.endpoint
}

output "cluster_version" {
  description = "Kubernetes version"
  value       = aws_eks_cluster.main.version
}

output "node_security_group_id" {
  description = "Security group ID for EKS nodes — passed to RDS, Redis, MQ modules"
  value       = aws_security_group.eks_nodes.id
}

output "oidc_provider_arn" {
  description = "OIDC provider ARN — used for IRSA"
  value       = aws_iam_openid_connect_provider.eks.arn
}

output "oidc_provider_url" {
  description = "OIDC provider URL — used for IRSA"
  value       = aws_eks_cluster.main.identity[0].oidc[0].issuer
}