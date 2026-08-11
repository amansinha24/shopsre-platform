output "mq_endpoint" {
  description = "RabbitMQ AMQP endpoint — services connect to this"
  value       = aws_mq_broker.main.instances[0].endpoints[0]
  sensitive   = true
}

output "mq_console_url" {
  description = "RabbitMQ management console URL"
  value       = aws_mq_broker.main.instances[0].console_url
  sensitive   = true
}

output "mq_broker_id" {
  description = "Amazon MQ broker ID"
  value       = aws_mq_broker.main.id
}