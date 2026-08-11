# ── VPC ────────────────────────────────────────────────────────
# Main network boundary for all ShopSRE resources
resource "aws_vpc" "main" {
  cidr_block           = var.vpc_cidr
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = {
    Name = "${var.project_name}-vpc"
    # Required tags for EKS to discover this VPC
    "kubernetes.io/cluster/${var.project_name}-cluster" = "shared"
  }
}

# ── Internet Gateway ───────────────────────────────────────────
# Allows resources in public subnets to reach internet
# ALB needs this to receive traffic from users
resource "aws_internet_gateway" "main" {
  vpc_id = aws_vpc.main.id

  tags = {
    Name = "${var.project_name}-igw"
  }
}

# ── Public Subnets ─────────────────────────────────────────────
# ALB lives here — internet facing
# One subnet per availability zone for high availability
resource "aws_subnet" "public" {
  count = length(var.availability_zones)

  vpc_id                  = aws_vpc.main.id
  cidr_block              = var.public_subnets[count.index]
  availability_zone       = var.availability_zones[count.index]
  map_public_ip_on_launch = true

  tags = {
    Name = "${var.project_name}-public-${var.availability_zones[count.index]}"
    # Required for ALB Ingress Controller to find public subnets
    "kubernetes.io/role/elb"                            = "1"
    "kubernetes.io/cluster/${var.project_name}-cluster" = "shared"
  }
}

# ── Private Subnets ────────────────────────────────────────────
# EKS nodes, RDS, Redis, RabbitMQ live here
# No direct internet access — much more secure
resource "aws_subnet" "private" {
  count = length(var.availability_zones)

  vpc_id            = aws_vpc.main.id
  cidr_block        = var.private_subnets[count.index]
  availability_zone = var.availability_zones[count.index]

  tags = {
    Name = "${var.project_name}-private-${var.availability_zones[count.index]}"
    # Required for internal load balancers
    "kubernetes.io/role/internal-elb"                   = "1"
    "kubernetes.io/cluster/${var.project_name}-cluster" = "shared"
  }
}

# ── Elastic IP for NAT Gateway ─────────────────────────────────
# NAT gateway needs a static public IP
resource "aws_eip" "nat" {
  domain = "vpc"

  tags = {
    Name = "${var.project_name}-nat-eip"
  }

  depends_on = [aws_internet_gateway.main]
}

# ── NAT Gateway ────────────────────────────────────────────────
# Allows private subnet resources to reach internet
# Example: EKS nodes pulling Docker images from ECR
# But internet cannot reach private subnets directly
resource "aws_nat_gateway" "main" {
  allocation_id = aws_eip.nat.id
  subnet_id     = aws_subnet.public[0].id

  tags = {
    Name = "${var.project_name}-nat"
  }

  depends_on = [aws_internet_gateway.main]
}

# ── Public Route Table ─────────────────────────────────────────
# Routes public subnet traffic to internet gateway
resource "aws_route_table" "public" {
  vpc_id = aws_vpc.main.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.main.id
  }

  tags = {
    Name = "${var.project_name}-public-rt"
  }
}

# ── Private Route Table ────────────────────────────────────────
# Routes private subnet traffic through NAT gateway
resource "aws_route_table" "private" {
  vpc_id = aws_vpc.main.id

  route {
    cidr_block     = "0.0.0.0/0"
    nat_gateway_id = aws_nat_gateway.main.id
  }

  tags = {
    Name = "${var.project_name}-private-rt"
  }
}

# ── Route Table Associations ───────────────────────────────────
# Connect each subnet to its route table
resource "aws_route_table_association" "public" {
  count          = length(var.availability_zones)
  subnet_id      = aws_subnet.public[count.index].id
  route_table_id = aws_route_table.public.id
}

resource "aws_route_table_association" "private" {
  count          = length(var.availability_zones)
  subnet_id      = aws_subnet.private[count.index].id
  route_table_id = aws_route_table.private.id
}