# ── Security Group for RDS ─────────────────────────────────────
# Controls which resources can connect to PostgreSQL
# Only EKS nodes can connect — nothing else
resource "aws_security_group" "rds" {
  name        = "${var.project_name}-rds-sg"
  description = "Security group for RDS PostgreSQL"
  vpc_id      = var.vpc_id

  # Allow PostgreSQL traffic only from EKS nodes
  ingress {
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = [var.eks_security_group]
    description     = "PostgreSQL from EKS nodes only"
  }

  # Allow all outbound traffic
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name = "${var.project_name}-rds-sg"
  }
}

# ── RDS Subnet Group ───────────────────────────────────────────
# Tells RDS which subnets it can use
# We use private subnets — RDS is never publicly accessible
resource "aws_db_subnet_group" "main" {
  name       = "${var.project_name}-rds-subnet-group"
  subnet_ids = var.private_subnet_ids

  tags = {
    Name = "${var.project_name}-rds-subnet-group"
  }
}

# ── RDS PostgreSQL Instance ────────────────────────────────────
resource "aws_db_instance" "main" {
  identifier = "${var.project_name}-postgres"

  # Database engine
  engine         = "postgres"
  engine_version = "15.4"
  instance_class = "db.t3.micro"

  # Storage
  allocated_storage     = 20
  max_allocated_storage = 100
  storage_type          = "gp2"
  storage_encrypted     = true

  # Database credentials
  db_name  = var.db_name
  username = var.db_username
  password = var.db_password

  # Network
  db_subnet_group_name   = aws_db_subnet_group.main.name
  vpc_security_group_ids = [aws_security_group.rds.id]
  publicly_accessible    = false

  # Backups
  backup_retention_period = 7
  backup_window           = "03:00-04:00"
  maintenance_window      = "Mon:04:00-Mon:05:00"

  # High availability
  multi_az = false

  # Don't delete DB when terraform destroy is run
  # Protects against accidental data loss
  deletion_protection = false
  skip_final_snapshot = true

  # Performance insights
  performance_insights_enabled = true

  tags = {
    Name = "${var.project_name}-postgres"
  }
}