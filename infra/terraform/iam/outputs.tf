output "auth_service_role_arn" {
  description = "IAM role ARN for Auth service — used in Helm chart"
  value       = aws_iam_role.auth_service.arn
}

output "orders_service_role_arn" {
  description = "IAM role ARN for Orders service — used in Helm chart"
  value       = aws_iam_role.orders_service.arn
}

output "notifications_service_role_arn" {
  description = "IAM role ARN for Notifications service — used in Helm chart"
  value       = aws_iam_role.notifications_service.arn
}

output "worker_service_role_arn" {
  description = "IAM role ARN for Worker service — used in Helm chart"
  value       = aws_iam_role.worker_service.arn
}

output "cloudwatch_log_groups" {
  description = "CloudWatch log group names for all services"
  value = {
    auth          = aws_cloudwatch_log_group.auth.name
    orders        = aws_cloudwatch_log_group.orders.name
    notifications = aws_cloudwatch_log_group.notifications.name
    worker        = aws_cloudwatch_log_group.worker.name
    frontend      = aws_cloudwatch_log_group.frontend.name
  }
}