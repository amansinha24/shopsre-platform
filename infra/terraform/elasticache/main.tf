# ── Security Group for ElastiCache ────────────────────────────
# Only EKS nodes can connect to Redis
resource "aws_security_group" "redis" {
  name        = "${var.project_name}-redis-sg"
  description = "Security group for ElastiCache Redis"
  vpc_id      = var.vpc_id

  # Allow Redis traffic only from EKS nodes
  ingress {
    from_port       = 6379
    to_port         = 6379
    protocol        = "tcp"
    security_groups = [var.eks_security_group]
    description     = "Redis from EKS nodes only"
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name = "${var.project_name}-redis-sg"
  }
}

# ── ElastiCache Subnet Group ───────────────────────────────────
# Tells ElastiCache which subnets it can use
resource "aws_elasticache_subnet_group" "main" {
  name       = "${var.project_name}-redis-subnet-group"
  subnet_ids = var.private_subnet_ids

  tags = {
    Name = "${var.project_name}-redis-subnet-group"
  }
}

# ── ElastiCache Parameter Group ────────────────────────────────
# Redis configuration settings
resource "aws_elasticache_parameter_group" "main" {
  family = "redis7"
  name   = "${var.project_name}-redis-params"

  # Enable keyspace notifications
  # Useful for cache expiry events
  parameter {
    name  = "notify-keyspace-events"
    value = "Ex"
  }

  tags = {
    Name = "${var.project_name}-redis-params"
  }
}

# ── ElastiCache Redis Cluster ──────────────────────────────────
resource "aws_elasticache_cluster" "main" {
  cluster_id           = "${var.project_name}-redis"
  engine               = "redis"
  engine_version       = "7.0"
  node_type            = "cache.t3.micro"
  num_cache_nodes      = 1
  parameter_group_name = aws_elasticache_parameter_group.main.name
  subnet_group_name    = aws_elasticache_subnet_group.main.name
  security_group_ids   = [aws_security_group.redis.id]
  port                 = 6379

  # Maintenance window — when AWS can apply updates
  maintenance_window = "sun:05:00-sun:06:00"

  # Snapshot for backup
  snapshot_retention_limit = 1
  snapshot_window          = "04:00-05:00"

  tags = {
    Name = "${var.project_name}-redis"
  }
}