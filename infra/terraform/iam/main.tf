# ── Data Sources ───────────────────────────────────────────────
# Get current AWS account ID
data "aws_caller_identity" "current" {}

# Get current AWS region
data "aws_region" "current" {}

# ── IRSA Trust Policy ──────────────────────────────────────────
# This allows a Kubernetes service account to assume an IAM role
# Each service gets its own trust policy
locals {
  oidc_provider = replace(var.oidc_provider_url, "https://", "")
  account_id    = data.aws_caller_identity.current.account_id
  region        = data.aws_region.current.name
}

# ── Auth Service IAM Role ──────────────────────────────────────
# Auth service needs to read JWT secret from Secrets Manager
resource "aws_iam_role" "auth_service" {
  name = "${var.project_name}-auth-service-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Principal = {
          Federated = var.oidc_provider_arn
        }
        Action = "sts:AssumeRoleWithWebIdentity"
        Condition = {
          StringEquals = {
            "${local.oidc_provider}:sub" = "system:serviceaccount:production:auth-service"
            "${local.oidc_provider}:aud" = "sts.amazonaws.com"
          }
        }
      }
    ]
  })

  tags = {
    Name = "${var.project_name}-auth-service-role"
  }
}

resource "aws_iam_role_policy" "auth_service" {
  name = "${var.project_name}-auth-service-policy"
  role = aws_iam_role.auth_service.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      # Read secrets from Secrets Manager
      {
        Effect = "Allow"
        Action = [
          "secretsmanager:GetSecretValue",
          "secretsmanager:DescribeSecret"
        ]
        Resource = [
          "arn:aws:secretsmanager:${local.region}:${local.account_id}:secret:shopsre/*"
        ]
      },
      # Write logs to CloudWatch
      {
        Effect = "Allow"
        Action = [
          "logs:CreateLogGroup",
          "logs:CreateLogStream",
          "logs:PutLogEvents",
          "logs:DescribeLogStreams"
        ]
        Resource = "arn:aws:logs:${local.region}:${local.account_id}:log-group:/shopsre/*"
      },
      # Send traces to X-Ray
      {
        Effect = "Allow"
        Action = [
          "xray:PutTraceSegments",
          "xray:PutTelemetryRecords",
          "xray:GetSamplingRules",
          "xray:GetSamplingTargets"
        ]
        Resource = "*"
      }
    ]
  })
}

# ── Orders Service IAM Role ────────────────────────────────────
# Orders service needs Secrets Manager, X-Ray, CloudWatch
resource "aws_iam_role" "orders_service" {
  name = "${var.project_name}-orders-service-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Principal = {
          Federated = var.oidc_provider_arn
        }
        Action = "sts:AssumeRoleWithWebIdentity"
        Condition = {
          StringEquals = {
            "${local.oidc_provider}:sub" = "system:serviceaccount:production:orders-service"
            "${local.oidc_provider}:aud" = "sts.amazonaws.com"
          }
        }
      }
    ]
  })

  tags = {
    Name = "${var.project_name}-orders-service-role"
  }
}

resource "aws_iam_role_policy" "orders_service" {
  name = "${var.project_name}-orders-service-policy"
  role = aws_iam_role.orders_service.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "secretsmanager:GetSecretValue",
          "secretsmanager:DescribeSecret"
        ]
        Resource = [
          "arn:aws:secretsmanager:${local.region}:${local.account_id}:secret:shopsre/*"
        ]
      },
      {
        Effect = "Allow"
        Action = [
          "logs:CreateLogGroup",
          "logs:CreateLogStream",
          "logs:PutLogEvents",
          "logs:DescribeLogStreams"
        ]
        Resource = "arn:aws:logs:${local.region}:${local.account_id}:log-group:/shopsre/*"
      },
      {
        Effect = "Allow"
        Action = [
          "xray:PutTraceSegments",
          "xray:PutTelemetryRecords",
          "xray:GetSamplingRules",
          "xray:GetSamplingTargets"
        ]
        Resource = "*"
      }
    ]
  })
}

# ── Notifications Service IAM Role ─────────────────────────────
# Notifications needs Secrets Manager, SES, CloudWatch, X-Ray
resource "aws_iam_role" "notifications_service" {
  name = "${var.project_name}-notifications-service-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Principal = {
          Federated = var.oidc_provider_arn
        }
        Action = "sts:AssumeRoleWithWebIdentity"
        Condition = {
          StringEquals = {
            "${local.oidc_provider}:sub" = "system:serviceaccount:production:notifications-service"
            "${local.oidc_provider}:aud" = "sts.amazonaws.com"
          }
        }
      }
    ]
  })

  tags = {
    Name = "${var.project_name}-notifications-service-role"
  }
}

resource "aws_iam_role_policy" "notifications_service" {
  name = "${var.project_name}-notifications-service-policy"
  role = aws_iam_role.notifications_service.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "secretsmanager:GetSecretValue",
          "secretsmanager:DescribeSecret"
        ]
        Resource = [
          "arn:aws:secretsmanager:${local.region}:${local.account_id}:secret:shopsre/*"
        ]
      },
      # Send emails via AWS SES
      {
        Effect = "Allow"
        Action = [
          "ses:SendEmail",
          "ses:SendRawEmail"
        ]
        Resource = "*"
      },
      {
        Effect = "Allow"
        Action = [
          "logs:CreateLogGroup",
          "logs:CreateLogStream",
          "logs:PutLogEvents",
          "logs:DescribeLogStreams"
        ]
        Resource = "arn:aws:logs:${local.region}:${local.account_id}:log-group:/shopsre/*"
      },
      {
        Effect = "Allow"
        Action = [
          "xray:PutTraceSegments",
          "xray:PutTelemetryRecords",
          "xray:GetSamplingRules",
          "xray:GetSamplingTargets"
        ]
        Resource = "*"
      }
    ]
  })
}

# ── Worker Service IAM Role ────────────────────────────────────
# Worker needs Secrets Manager, X-Ray, CloudWatch
resource "aws_iam_role" "worker_service" {
  name = "${var.project_name}-worker-service-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Principal = {
          Federated = var.oidc_provider_arn
        }
        Action = "sts:AssumeRoleWithWebIdentity"
        Condition = {
          StringEquals = {
            "${local.oidc_provider}:sub" = "system:serviceaccount:production:worker-service"
            "${local.oidc_provider}:aud" = "sts.amazonaws.com"
          }
        }
      }
    ]
  })

  tags = {
    Name = "${var.project_name}-worker-service-role"
  }
}

resource "aws_iam_role_policy" "worker_service" {
  name = "${var.project_name}-worker-service-policy"
  role = aws_iam_role.worker_service.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "secretsmanager:GetSecretValue",
          "secretsmanager:DescribeSecret"
        ]
        Resource = [
          "arn:aws:secretsmanager:${local.region}:${local.account_id}:secret:shopsre/*"
        ]
      },
      {
        Effect = "Allow"
        Action = [
          "logs:CreateLogGroup",
          "logs:CreateLogStream",
          "logs:PutLogEvents",
          "logs:DescribeLogStreams"
        ]
        Resource = "arn:aws:logs:${local.region}:${local.account_id}:log-group:/shopsre/*"
      },
      {
        Effect = "Allow"
        Action = [
          "xray:PutTraceSegments",
          "xray:PutTelemetryRecords",
          "xray:GetSamplingRules",
          "xray:GetSamplingTargets"
        ]
        Resource = "*"
      }
    ]
  })
}

# ── CloudWatch Log Groups ──────────────────────────────────────
# Create log groups for each service
# Logs are retained for 30 days
resource "aws_cloudwatch_log_group" "auth" {
  name              = "/shopsre/auth"
  retention_in_days = 30

  tags = {
    Name    = "/shopsre/auth"
    Service = "auth"
  }
}

resource "aws_cloudwatch_log_group" "orders" {
  name              = "/shopsre/orders"
  retention_in_days = 30

  tags = {
    Name    = "/shopsre/orders"
    Service = "orders"
  }
}

resource "aws_cloudwatch_log_group" "notifications" {
  name              = "/shopsre/notifications"
  retention_in_days = 30

  tags = {
    Name    = "/shopsre/notifications"
    Service = "notifications"
  }
}

resource "aws_cloudwatch_log_group" "worker" {
  name              = "/shopsre/worker"
  retention_in_days = 30

  tags = {
    Name    = "/shopsre/worker"
    Service = "worker"
  }
}

resource "aws_cloudwatch_log_group" "frontend" {
  name              = "/shopsre/frontend"
  retention_in_days = 30

  tags = {
    Name    = "/shopsre/frontend"
    Service = "frontend"
  }
}