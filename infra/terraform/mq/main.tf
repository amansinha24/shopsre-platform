# ── Security Group for Amazon MQ ──────────────────────────────
# Only EKS nodes can connect to RabbitMQ
resource "aws_security_group" "mq" {
  name        = "${var.project_name}-mq-sg"
  description = "Security group for Amazon MQ RabbitMQ"
  vpc_id      = var.vpc_id

  # AMQP protocol — used by services to publish/consume messages
  ingress {
    from_port       = 5671
    to_port         = 5671
    protocol        = "tcp"
    security_groups = [var.eks_security_group]
    description     = "AMQP from EKS nodes only"
  }

  # RabbitMQ management console
  ingress {
    from_port       = 443
    to_port         = 443
    protocol        = "tcp"
    security_groups = [var.eks_security_group]
    description     = "RabbitMQ management UI"
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name = "${var.project_name}-mq-sg"
  }
}

# ── Amazon MQ Broker ───────────────────────────────────────────
resource "aws_mq_broker" "main" {
  broker_name = "${var.project_name}-rabbitmq"

  # RabbitMQ engine
  engine_type        = "RabbitMQ"
  engine_version     = "3.13"
  host_instance_type = "mq.t3.micro"

  # Single instance for cost saving
  # In production use CLUSTER_MULTI_AZ for high availability
  deployment_mode = "SINGLE_INSTANCE"

  # Network
  subnet_ids         = [var.private_subnet_ids[0]]
  security_groups    = [aws_security_group.mq.id]
  publicly_accessible = false

  # Admin credentials — come from Secrets Manager
  user {
    username = var.mq_username
    password = var.mq_password
  }

  # Maintenance window
  maintenance_window_start_time {
    day_of_week = "SUNDAY"
    time_of_day = "05:00"
    time_zone   = "UTC"
  }

  # Enable CloudWatch logging
  logs {
    general = true
  }

  tags = {
    Name = "${var.project_name}-rabbitmq"
  }
}