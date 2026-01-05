# RDS PostgreSQL for Installer Application
# Creates a PostgreSQL RDS instance in private subnets

data "aws_subnets" "private" {
  filter {
    name   = "vpc-id"
    values = [var.vpc_id]
  }

  filter {
    name   = "tag:Name"
    values = ["*private*"]
  }
}

resource "aws_db_subnet_group" "main" {
  name       = "installer-${var.nuon_id}"
  subnet_ids = data.aws_subnets.private.ids

  tags = {
    Name   = "installer-${var.nuon_id}"
    NuonID = var.nuon_id
  }
}

resource "aws_security_group" "rds" {
  name        = "installer-rds-${var.nuon_id}"
  description = "Security group for RDS PostgreSQL"
  vpc_id      = var.vpc_id

  ingress {
    description = "PostgreSQL from VPC"
    from_port   = 5432
    to_port     = 5432
    protocol    = "tcp"
    cidr_blocks = [data.aws_vpc.main.cidr_block]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name   = "installer-rds-${var.nuon_id}"
    NuonID = var.nuon_id
  }
}

data "aws_vpc" "main" {
  id = var.vpc_id
}

resource "aws_db_instance" "main" {
  identifier     = "installer-${var.nuon_id}"
  engine         = "postgres"
  engine_version = "15"

  instance_class    = var.db_instance_class
  allocated_storage = var.allocated_storage
  storage_type      = "gp3"

  db_name  = var.db_name
  username = var.db_username
  password = var.db_password

  db_subnet_group_name   = aws_db_subnet_group.main.name
  vpc_security_group_ids = [aws_security_group.rds.id]

  publicly_accessible = false
  skip_final_snapshot = true

  backup_retention_period = 7
  backup_window           = "03:00-04:00"
  maintenance_window      = "Mon:04:00-Mon:05:00"

  tags = {
    Name   = "installer-${var.nuon_id}"
    NuonID = var.nuon_id
  }
}
