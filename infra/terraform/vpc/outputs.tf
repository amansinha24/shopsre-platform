output "vpc_id" {
  description = "VPC ID — passed to EKS, RDS, ElastiCache, MQ modules"
  value       = aws_vpc.main.id
}

output "public_subnet_ids" {
  description = "Public subnet IDs — ALB lives here"
  value       = aws_subnet.public[*].id
}

output "private_subnet_ids" {
  description = "Private subnet IDs — EKS nodes, RDS, Redis, MQ live here"
  value       = aws_subnet.private[*].id
}

output "vpc_cidr" {
  description = "VPC CIDR block"
  value       = aws_vpc.main.cidr_block
}